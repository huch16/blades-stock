package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/robertkrimen/otto"
	lo "github.com/samber/lo"
)

func (m MarketNewsApi) TelegraphList(crawlTimeOut int64) *[]models.Telegraph {
	//https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9"
	res := map[string]any{}
	_, _ = SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetResult(&res).
		Get(clsURL)
	var telegraphs []models.Telegraph

	if v, _ := convertor.ToInt(res["errno"]); v == 0 {
		if res["data"] == nil {
			return m.GetNewTelegraph(crawlTimeOut)
		}
		data, ok := res["data"].(map[string]any)
		if !ok {
			return m.GetNewTelegraph(crawlTimeOut)
		}
		rollData, ok := data["roll_data"].([]any)
		if !ok || len(rollData) == 0 {
			return m.GetNewTelegraph(crawlTimeOut)
		}
		// 批量预取去重依据与标签（原为每条快讯一次 Count、每个标签一次
		// FirstOrCreate，N+1）。预取只覆盖本批 rollData 涉及的标题/内容/
		// 主题名；循环内再用 seen 集合补上本批已入库的行，与原先逐条
		// Count 能看到同批先入库的行等价。
		contents := make([]string, 0, len(rollData))
		titles := make([]string, 0, len(rollData))
		subjectNames := make([]string, 0)
		seenSubjectName := make(map[string]bool)
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			title, _ := news["title"].(string)
			content, _ := news["content"].(string)
			if title == "" {
				contents = append(contents, content)
			} else {
				titles = append(titles, title)
			}
			if subs, ok := news["subjects"].([]any); ok {
				for _, subject := range subs {
					if subMap, ok := subject.(map[string]any); ok {
						if name, ok := subMap["subject_name"].(string); ok && name != "" && !seenSubjectName[name] {
							seenSubjectName[name] = true
							subjectNames = append(subjectNames, name)
						}
					}
				}
			}
		}
		existingContent := make(map[string]bool, len(contents))
		if len(contents) > 0 {
			var rows []models.Telegraph
			db.Dao.Model(&models.Telegraph{}).Select("content").Where("content IN ?", contents).Find(&rows)
			for _, r := range rows {
				existingContent[r.Content] = true
			}
		}
		existingTitle := make(map[string]bool, len(titles))
		if len(titles) > 0 {
			var rows []models.Telegraph
			db.Dao.Model(&models.Telegraph{}).Select("title").Where("title IN ?", titles).Find(&rows)
			for _, r := range rows {
				existingTitle[r.Title] = true
			}
		}
		tagByName := make(map[string]*models.Tags)
		if len(subjectNames) > 0 {
			var tags []models.Tags
			db.Dao.Model(&models.Tags{}).Where("name IN ? AND type = ?", subjectNames, "subject").Find(&tags)
			for i := range tags {
				tagByName[tags[i].Name] = &tags[i]
			}
		}
		seenContent := make(map[string]bool)
		seenTitle := make(map[string]bool)
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			ctime, _ := convertor.ToInt(news["ctime"])
			dataTime := time.Unix(ctime, 0).Local()

			shareURL := ""
			if su, ok2 := news["shareurl"].(string); ok2 && su != "" {
				shareURL = su
			} else if id, ok2 := news["id"]; ok2 {
				shareURL = fmt.Sprintf("https://www.cls.cn/telegraph/%v", id)
			}

			title, _ := news["title"].(string)
			content, _ := news["content"].(string)
			level, _ := news["level"].(string)

			telegraph := models.Telegraph{
				Title:           title,
				Content:         content,
				Time:            dataTime.Format("15:04:05"),
				DataTime:        &dataTime,
				Url:             shareURL,
				Source:          "财联社电报",
				IsRed:           level != "C",
				SentimentResult: AnalyzeSentiment(content).Description,
			}
			// 去重：查批量预取的集合 + 本批已入库的 seen 集合，与原先逐条
			// Count 等价（原先 Count 也能看到同批先入库的行）。
			if telegraph.Title == "" {
				if existingContent[telegraph.Content] || seenContent[telegraph.Content] {
					continue
				}
				seenContent[telegraph.Content] = true
			} else {
				if existingTitle[telegraph.Title] || seenTitle[telegraph.Title] {
					continue
				}
				seenTitle[telegraph.Title] = true
			}
			telegraphs = append(telegraphs, telegraph)
			db.Dao.Model(&models.Telegraph{}).Create(&telegraph)
			if news["subjects"] == nil {
				continue
			}
			subjects, ok := news["subjects"].([]any)
			if !ok {
				continue
			}
			// 标签：先查批量预取的表，只对真正新增的标签单独 Create；
			// 关联行一次批量 Create（新快讯的 ID 不可能已有 TelegraphTags，
			// 与原先逐对 FirstOrCreate 等价；同主题去重后一次写入）。
			pairs := make([]models.TelegraphTags, 0, len(subjects))
			seenTagID := make(map[uint]bool)
			for _, subject := range subjects {
				subMap, ok := subject.(map[string]any)
				if !ok {
					continue
				}
				name, ok := subMap["subject_name"].(string)
				if !ok || name == "" {
					continue
				}
				tag, ok := tagByName[name]
				if !ok {
					tag = &models.Tags{
						Name: name,
						Type: "subject",
					}
					db.Dao.Model(tag).Create(tag)
					tagByName[name] = tag
				}
				if !seenTagID[tag.ID] {
					seenTagID[tag.ID] = true
					pairs = append(pairs, models.TelegraphTags{
						TelegraphId: telegraph.ID,
						TagId:       tag.ID,
					})
				}
			}
			if len(pairs) > 0 {
				db.Dao.Model(models.TelegraphTags{}).Create(&pairs)
			}
		}
	} else {
		return m.GetNewTelegraph(crawlTimeOut)
	}

	return &telegraphs
}

func (m MarketNewsApi) GetNewTelegraph(crawlTimeOut int64) *[]models.Telegraph {
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraphList&os=web&sv=8.7.9"
	res := map[string]any{}
	_, _ = SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetResult(&res).
		Get(clsURL)
	var telegraphs []models.Telegraph

	if v, _ := convertor.ToInt(res["errno"]); v == 0 {
		if res["data"] == nil {
			return &telegraphs
		}
		data, ok := res["data"].(map[string]any)
		if !ok {
			return &telegraphs
		}
		rollData, ok := data["roll_data"].([]any)
		if !ok {
			return &telegraphs
		}
		// 批量预取去重依据与标签（原为每条快讯一次 Count、每个标签一次
		// FirstOrCreate，N+1）。预取只覆盖本批 rollData 涉及的标题/内容/
		// 主题名；循环内再用 seen 集合补上本批已入库的行，与原先逐条
		// Count 能看到同批先入库的行等价。
		contents := make([]string, 0, len(rollData))
		titles := make([]string, 0, len(rollData))
		subjectNames := make([]string, 0)
		seenSubjectName := make(map[string]bool)
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			title, _ := news["title"].(string)
			content, _ := news["content"].(string)
			if title == "" {
				contents = append(contents, content)
			} else {
				titles = append(titles, title)
			}
			if subs, ok := news["subjects"].([]any); ok {
				for _, subject := range subs {
					if subMap, ok := subject.(map[string]any); ok {
						if name, ok := subMap["subject_name"].(string); ok && name != "" && !seenSubjectName[name] {
							seenSubjectName[name] = true
							subjectNames = append(subjectNames, name)
						}
					}
				}
			}
		}
		existingContent := make(map[string]bool, len(contents))
		if len(contents) > 0 {
			var rows []models.Telegraph
			db.Dao.Model(&models.Telegraph{}).Select("content").Where("content IN ?", contents).Find(&rows)
			for _, r := range rows {
				existingContent[r.Content] = true
			}
		}
		existingTitle := make(map[string]bool, len(titles))
		if len(titles) > 0 {
			var rows []models.Telegraph
			db.Dao.Model(&models.Telegraph{}).Select("title").Where("title IN ?", titles).Find(&rows)
			for _, r := range rows {
				existingTitle[r.Title] = true
			}
		}
		tagByName := make(map[string]*models.Tags)
		if len(subjectNames) > 0 {
			var tags []models.Tags
			db.Dao.Model(&models.Tags{}).Where("name IN ? AND type = ?", subjectNames, "subject").Find(&tags)
			for i := range tags {
				tagByName[tags[i].Name] = &tags[i]
			}
		}
		seenContent := make(map[string]bool)
		seenTitle := make(map[string]bool)
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			ctime, _ := convertor.ToInt(news["ctime"])
			dataTime := time.Unix(ctime, 0).Local()

			shareURL := ""
			if su, ok2 := news["shareurl"].(string); ok2 && su != "" {
				shareURL = su
			} else if id, ok2 := news["id"]; ok2 {
				shareURL = fmt.Sprintf("https://www.cls.cn/telegraph/%v", id)
			}

			title, _ := news["title"].(string)
			content, _ := news["content"].(string)
			level, _ := news["level"].(string)

			telegraph := models.Telegraph{
				Title:           title,
				Content:         content,
				Time:            dataTime.Format("15:04:05"),
				DataTime:        &dataTime,
				Url:             shareURL,
				Source:          "财联社电报",
				IsRed:           level != "C",
				SentimentResult: AnalyzeSentiment(content).Description,
			}
			// 去重：查批量预取的集合 + 本批已入库的 seen 集合，与原先逐条
			// Count 等价（原先 Count 也能看到同批先入库的行）。
			if telegraph.Title == "" {
				if existingContent[telegraph.Content] || seenContent[telegraph.Content] {
					continue
				}
				seenContent[telegraph.Content] = true
			} else {
				if existingTitle[telegraph.Title] || seenTitle[telegraph.Title] {
					continue
				}
				seenTitle[telegraph.Title] = true
			}
			telegraphs = append(telegraphs, telegraph)
			db.Dao.Model(&models.Telegraph{}).Create(&telegraph)
			if news["subjects"] == nil {
				continue
			}
			subjects, ok := news["subjects"].([]any)
			if !ok {
				continue
			}
			// 标签：先查批量预取的表，只对真正新增的标签单独 Create；
			// 关联行一次批量 Create（新快讯的 ID 不可能已有 TelegraphTags，
			// 与原先逐对 FirstOrCreate 等价；同主题去重后一次写入）。
			pairs := make([]models.TelegraphTags, 0, len(subjects))
			seenTagID := make(map[uint]bool)
			for _, subject := range subjects {
				subMap, ok := subject.(map[string]any)
				if !ok {
					continue
				}
				name, ok := subMap["subject_name"].(string)
				if !ok || name == "" {
					continue
				}
				tag, ok := tagByName[name]
				if !ok {
					tag = &models.Tags{
						Name: name,
						Type: "subject",
					}
					db.Dao.Model(tag).Create(tag)
					tagByName[name] = tag
				}
				if !seenTagID[tag.ID] {
					seenTagID[tag.ID] = true
					pairs = append(pairs, models.TelegraphTags{
						TelegraphId: telegraph.ID,
						TagId:       tag.ID,
					})
				}
			}
			if len(pairs) > 0 {
				db.Dao.Model(models.TelegraphTags{}).Create(&pairs)
			}
		}
	}
	return &telegraphs
}

func (m MarketNewsApi) GetTelegraphList(source string) *[]*models.Telegraph {
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=?", source).Order("data_time desc,time desc").Limit(50).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Order("data_time desc,time desc").Limit(50).Find(news)
	}
	// 批量取回所有标签（原为每条快讯一次查询，N+1），再按 ID 查表。
	// 原先每条快讯的标签名顺序是数据库返回顺序（按 id 升序），此处对每个快讯
	// 的标签 ID 排升序后再映射，保持同样的顺序。
	allTagIDs := make([]uint, 0)
	seenTagID := make(map[uint]bool)
	for _, item := range *news {
		for _, tt := range item.TelegraphTags {
			if !seenTagID[tt.TagId] {
				seenTagID[tt.TagId] = true
				allTagIDs = append(allTagIDs, tt.TagId)
			}
		}
	}
	tagNameByID := make(map[uint]string)
	if len(allTagIDs) > 0 {
		allTags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", allTagIDs).Order("id").Find(allTags)
		for _, t := range *allTags {
			tagNameByID[t.ID] = t.Name
		}
	}
	for _, item := range *news {
		ids := make([]uint, 0, len(item.TelegraphTags))
		for _, tt := range item.TelegraphTags {
			ids = append(ids, tt.TagId)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		tagNames := make([]string, 0, len(ids))
		for _, id := range ids {
			if name, ok := tagNameByID[id]; ok {
				tagNames = append(tagNames, name)
			}
		}
		item.SubjectTags = tagNames
		//logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
	}
	return news
}

func (m MarketNewsApi) GetSinaNews(crawlTimeOut uint) *[]models.Telegraph {
	news := &[]models.Telegraph{}
	response, _ := SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get("https://zhibo.sina.com.cn/api/zhibo/feed?callback=callback&page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20&id=4161089&type=0&_=" + strconv.FormatInt(time.Now().Unix(), 10))
	js := string(response.Body())
	js = strutil.ReplaceWithMap(js, map[string]string{
		"try{callback(":  "var data=",
		");}catch(e){};": ";",
	})
	//logger.SugaredLogger.Info(js)
	vm := otto.New()
	_, err := vm.Run(js)
	if err != nil {
		logger.SugaredLogger.Error(err)
	}
	vm.Run("var result = data.result;")
	//vm.Run("var resultStr =JSON.stringify(data);")
	vm.Run("var resultData = result.data;")
	vm.Run("var feed = resultData.feed;")
	vm.Run("var feedStr = JSON.stringify(feed);")

	value, _ := vm.Get("feedStr")
	// 检查返回值是否为 undefined 或空
	valueStr := value.String()
	if valueStr == "undefined" || valueStr == "" {
		logger.SugaredLogger.Debugf("fetchNewsList: JavaScript 返回 undefined，跳过解析")
		return news
	}

	feed := make(map[string]any)
	if err := json.Unmarshal([]byte(valueStr), &feed); err != nil {
		logger.SugaredLogger.Errorf("fetchNewsList json.Unmarshal feed error: %v, raw=%s", err, valueStr)
		return news
	}
	var telegraphs []models.Telegraph

	rawList, ok := feed["list"].([]any)
	if !ok || len(rawList) == 0 {
		return news
	}
	for _, item := range rawList {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		telegraph := models.Telegraph{Source: "新浪财经"}
		richText, _ := data["rich_text"].(string)
		createTime, _ := data["create_time"].(string)
		telegraph.Content = richText
		telegraph.Title = strutil.SubInBetween(richText, "【", "】")
		telegraph.Time = strings.Split(createTime, " ")[1]
		dataTime, _ := time.ParseInLocation("2006-01-02 15:04:05", createTime, time.Local)
		if !dataTime.IsZero() {
			telegraph.DataTime = &dataTime
		}
		rawTags, _ := data["tag"].([]any)
		telegraph.SubjectTags = lo.Map(rawTags, func(tagItem any, index int) string {
			tagMap, _ := tagItem.(map[string]any)
			name, _ := tagMap["name"].(string)
			tag := &models.Tags{
				Name: name,
				Type: "sina_subject",
			}
			db.Dao.Model(tag).Where("name=? and type=?", name, "sina_subject").FirstOrCreate(&tag)
			return name
		})
		if _, ok := lo.Find(telegraph.SubjectTags, func(item string) bool { return item == "焦点" }); ok {
			telegraph.IsRed = true
		}

		if telegraph.Content != "" {
			telegraph.SentimentResult = AnalyzeSentiment(telegraph.Content).Description
			cnt := int64(0)
			if telegraph.Title == "" {
				db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
			} else {
				db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
			}
			if cnt == 0 {
				db.Dao.Create(&telegraph)
				telegraphs = append(telegraphs, telegraph)
				for _, tag := range telegraph.SubjectTags {
					tagInfo := &models.Tags{}
					db.Dao.Model(models.Tags{}).Where("name=? and type=?", tag, "sina_subject").First(&tagInfo)
					if tagInfo.ID > 0 {
						db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tagInfo.ID).FirstOrCreate(&models.TelegraphTags{
							TelegraphId: telegraph.ID,
							TagId:       tagInfo.ID,
						})
					}
				}
			}
		}
	}
	return &telegraphs
}

func (m MarketNewsApi) GetNews24HoursList(source string, limit int) *[]*models.Telegraph {
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=? and created_at>?", source, time.Now().Add(-24*time.Hour)).Order("data_time desc,is_red desc").Limit(limit).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Where("created_at>?", time.Now().Add(-24*time.Hour)).Order("data_time desc,is_red desc").Limit(limit).Find(news)
	}
	// 内容去重
	uniqueNews := make([]*models.Telegraph, 0)
	seenContent := make(map[string]bool)
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		//logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
		// 使用内容作为去重键值，可以考虑只使用内容的前几个字符或哈希值
		contentKey := strings.TrimSpace(item.Content)
		if contentKey != "" && !seenContent[contentKey] {
			seenContent[contentKey] = true
			uniqueNews = append(uniqueNews, item)
		}
	}
	return &uniqueNews
}

// GetNewsListData 分页获取新闻列表，page 从 1 开始，pageSize 为每页条数（<=0 时默认 20）。返回本页去重后的列表与总条数。

// GetNewsListData 分页获取新闻列表，page 从 1 开始，pageSize 为每页条数（<=0 时默认 20）。返回本页去重后的列表与总条数。
func (m MarketNewsApi) GetNewsListData(keyWord string, startTime time.Time, page, pageSize int) (*[]*models.Telegraph, int64) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	whereCond := "created_at>? and (title like ? or content like ?)"
	args := []any{startTime, "%" + keyWord + "%", "%" + keyWord + "%"}
	var total int64
	db.Dao.Model(&models.Telegraph{}).Where(whereCond, args...).Count(&total)
	offset := (page - 1) * pageSize
	news := &[]*models.Telegraph{}
	db.Dao.Model(news).Preload("TelegraphTags").Where(whereCond, args...).Order("data_time desc,is_red desc").Offset(offset).Limit(pageSize).Find(news)
	// 内容去重
	uniqueNews := make([]*models.Telegraph, 0)
	seenContent := make(map[string]bool)
	for _, item := range *news {
		contentKey := strings.TrimSpace(item.Content)
		if item.Title != "" {
			contentKey = strings.TrimSpace(item.Title)
		}
		if contentKey != "" && !seenContent[contentKey] {
			seenContent[contentKey] = true
			uniqueNews = append(uniqueNews, item)
		}
	}
	return &uniqueNews, total
}
