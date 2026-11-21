package data

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
)

func (m MarketNewsApi) GlobalStockIndexes(crawlTimeOut uint) map[string]any {
	response, err := SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://stockapp.finance.qq.com/mstats").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get("https://proxy.finance.qq.com/ifzqgtimg/appstock/app/rank/indexRankDetail2")
	if err != nil {
		logger.SugaredLogger.Warnf("GlobalStockIndexes 请求失败: %v", err)
		return map[string]any{}
	}
	res := make(map[string]any)
	if err := json.Unmarshal(response.Body(), &res); err != nil {
		logger.SugaredLogger.Warnf("GlobalStockIndexes 解析失败: %v", err)
		return map[string]any{}
	}
	data, ok := res["data"].(map[string]any)
	if !ok || data == nil {
		logger.SugaredLogger.Warnf("GlobalStockIndexes 返回结构异常: data 缺失或类型不符")
		return map[string]any{}
	}
	return data
}

// GlobalStockIndexesReadable 获取全球指数并转换为 AI 易读的 Markdown 文本。

// GlobalStockIndexesReadable 获取全球指数并转换为 AI 易读的 Markdown 文本。
func (m MarketNewsApi) GlobalStockIndexesReadable(crawlTimeOut uint) string {
	data := m.GlobalStockIndexes(crawlTimeOut)
	return m.GlobalStockIndexesToReadable(data)
}

// GlobalStockIndexesToReadable 将 GlobalStockIndexes 返回的 JSON 转为 AI 易读格式（Markdown）。
//
//	输入示例：map[string]any{
//	  "america": []any{...},
//	  "asia":    []any{...},
//	  "europe":  []any{...},
//	  "other":   []any{...},
//	  "common":  []any{...},
//	}

//	输入示例：map[string]any{
//	  "america": []any{...},
//	  "asia":    []any{...},
//	  "europe":  []any{...},
//	  "other":   []any{...},
//	  "common":  []any{...},
//	}
func (m MarketNewsApi) GlobalStockIndexesToReadable(data map[string]any) string {
	if len(data) == 0 {
		return "暂无全球指数数据。"
	}
	type regionDef struct {
		Key   string
		Title string
	}
	regions := []regionDef{
		{Key: "common", Title: "重点关注"},
		{Key: "asia", Title: "亚洲市场"},
		{Key: "america", Title: "美洲市场"},
		{Key: "europe", Title: "欧洲市场"},
		{Key: "other", Title: "其他市场"},
	}

	stateText := func(v string) string {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "open":
			return "开盘"
		case "close":
			return "收盘"
		default:
			if v == "" {
				return "-"
			}
			return v
		}
	}

	var sb strings.Builder
	sb.WriteString("# 全球主要指数概览\n")
	sb.WriteString("> 数据来源：腾讯财经，已按区域整理。\n\n")

	written := 0
	for _, region := range regions {
		raw, ok := data[region.Key]
		if !ok || raw == nil {
			continue
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			continue
		}
		written++
		sb.WriteString("## ")
		sb.WriteString(region.Title)
		sb.WriteString("\n")
		sb.WriteString("| 指数 | 地区 | 最新点位 | 涨跌幅(%) | 状态 |\n")
		sb.WriteString("| --- | --- | ---: | ---: | --- |\n")

		for _, item := range list {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := convertor.ToString(row["name"])
			location := convertor.ToString(row["location"])
			zxj := convertor.ToString(row["zxj"])
			zdf := convertor.ToString(row["zdf"])
			state := stateText(convertor.ToString(row["state"]))
			if name == "" {
				name = convertor.ToString(row["code"])
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", name, location, zxj, zdf, state))
		}
		sb.WriteString("\n")
	}

	if written == 0 {
		return "暂无可解析的全球指数数据。"
	}
	return sb.String()
}

// CacheGlobalStockIndexes 将全球指数数据缓存到数据库
func (m MarketNewsApi) CacheGlobalStockIndexes(crawlTimeOut uint) error {
	data := m.GlobalStockIndexes(crawlTimeOut)
	if len(data) == 0 {
		return fmt.Errorf("获取全球指数数据失败")
	}

	// 定义区域映射
	regions := map[string]string{
		"america": "美洲",
		"asia":    "亚洲",
		"europe":  "欧洲",
		"common":  "重点关注",
		"other":   "其他",
	}

	// 先收集全部指数条目（原为每条一次 First + 一次 FirstOrCreate，N+1）
	var indexes []models.GlobalStockIndex
	qtCodes := make([]string, 0)
	for regionKey, regionName := range regions {
		raw, ok := data[regionKey]
		if !ok || raw == nil {
			continue
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			continue
		}

		for _, item := range list {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}

			indexes = append(indexes, models.GlobalStockIndex{
				Code:       convertor.ToString(row["code"]),
				Name:       convertor.ToString(row["name"]),
				Location:   convertor.ToString(row["location"]),
				Qtcode:     convertor.ToString(row["qtcode"]),
				State:      convertor.ToString(row["state"]),
				Zdf:        convertor.ToString(row["zdf"]),
				Zxj:        convertor.ToString(row["zxj"]),
				Img:        convertor.ToString(row["img"]),
				Region:     regionKey,
				RegionName: regionName,
			})
			qtCodes = append(qtCodes, convertor.ToString(row["qtcode"]))
		}
	}

	// 一次 IN 查询取回已存在的行，按 qtcode 查表（原先每条查一次）
	existingByQtcode := make(map[string]models.GlobalStockIndex, len(qtCodes))
	if len(qtCodes) > 0 {
		var existingRows []models.GlobalStockIndex
		db.Dao.Model(&models.GlobalStockIndex{}).Where("qtcode IN ?", qtCodes).Find(&existingRows)
		for _, r := range existingRows {
			existingByQtcode[r.Qtcode] = r
		}
	}

	// 如果已存在则更新，不存在则创建；新创建的行也记入 map，
	// 使同一 qtcode 在多个区域出现时第二次走更新（与原先一致）。
	for _, index := range indexes {
		if existing, ok := existingByQtcode[index.Qtcode]; ok {
			// 记录已存在，更新
			db.Dao.Model(&existing).Updates(map[string]any{
				"name":     index.Name,
				"location": index.Location,
				"state":    index.State,
				"zdf":      index.Zdf,
				"zxj":      index.Zxj,
				"img":      index.Img,
				"region":   index.Region,
			})
		} else {
			db.Dao.Model(&models.GlobalStockIndex{}).Create(&index)
			existingByQtcode[index.Qtcode] = index
		}
	}

	logger.SugaredLogger.Info("全球指数缓存完成")
	return nil
}
