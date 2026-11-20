package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"encoding/json"
	"fmt"
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
			cnt := int64(0)
			if telegraph.Title == "" {
				db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
			} else {
				db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
			}
			if cnt > 0 {
				continue
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
			for _, subject := range subjects {
				subMap, ok := subject.(map[string]any)
				if !ok {
					continue
				}
				name, ok := subMap["subject_name"].(string)
				if !ok || name == "" {
					continue
				}
				tag := &models.Tags{
					Name: name,
					Type: "subject",
				}
				db.Dao.Model(tag).Where("name=? and type=?", name, "subject").FirstOrCreate(&tag)
				db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tag.ID).FirstOrCreate(&models.TelegraphTags{
					TelegraphId: telegraph.ID,
					TagId:       tag.ID,
				})
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
			cnt := int64(0)
			if telegraph.Title == "" {
				db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
			} else {
				db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
			}
			if cnt > 0 {
				continue
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
			for _, subject := range subjects {
				subMap, ok := subject.(map[string]any)
				if !ok {
					continue
				}
				name, ok := subMap["subject_name"].(string)
				if !ok || name == "" {
					continue
				}
				tag := &models.Tags{
					Name: name,
					Type: "subject",
				}
				db.Dao.Model(tag).Where("name=? and type=?", name, "subject").FirstOrCreate(&tag)
				db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tag.ID).FirstOrCreate(&models.TelegraphTags{
					TelegraphId: telegraph.ID,
					TagId:       tag.ID,
				})
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
