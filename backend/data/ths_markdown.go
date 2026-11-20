package data

import (
	"encoding/json"

	"github.com/duke-git/lancet/v2/convertor"
)

// ThsResultToMarkdown 将通联/同花顺搜索结果统一转换为 markdown 表格。
// 由 backend/agent/tools 与 backend/data 两处的同名实现收敛而来
// （R-09）：以 tools 版的安全逻辑为准（多处 ok 判断，脏数据返回
// “无符合条件的数据”而不是直接断言 panic）；原 openai_tools.go 版
// 的直接断言已删除。
func ThsResultToMarkdown(res map[string]any, title string) string {
	if convertor.ToString(res["code"]) != "100" {
		return "无符合条件的数据"
	}

	resData, ok := res["data"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}
	result, ok := resData["result"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}

	dataList, ok := result["dataList"].([]any)
	if !ok {
		return "无符合条件的数据"
	}
	columns, ok := result["columns"].([]any)
	if !ok {
		return "无符合条件的数据"
	}

	headers := map[string]string{}
	for _, v := range columns {
		d, ok := v.(map[string]any)
		if !ok {
			continue
		}
		colTitle := convertor.ToString(d["title"])
		if dm := convertor.ToString(d["dateMsg"]); dm != "" {
			colTitle += "[" + dm + "]"
		}
		if u := convertor.ToString(d["unit"]); u != "" {
			colTitle += "(" + u + ")"
		}
		if keyStr, ok := d["key"].(string); ok {
			headers[keyStr] = colTitle
		}
	}

	table := &[]map[string]any{}
	for _, v := range dataList {
		d, ok := v.(map[string]any)
		if !ok {
			continue
		}
		row := map[string]any{}
		for key, colTitle := range headers {
			row[colTitle] = convertor.ToString(d[key])
		}
		*table = append(*table, row)
	}

	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	return "\r\n### " + title + "：\r\n" + markdownTable + "\r\n"
}
