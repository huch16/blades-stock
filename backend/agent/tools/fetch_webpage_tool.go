package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/logger"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/go-resty/resty/v2"
)

func GetFetchWebPageTool() bladesTools.Tool {
	return types.NewTool("FetchWebPage",
		"抓取指定URL的网页正文内容。用于获取公告原文、环评报告、招投标页面、新闻文章等。返回页面的结构化文本内容。",
		map[string]*types.ParamDef{
			"url": {
				Type:     "string",
				Desc:     "目标网页URL，必须是完整的http/https地址",
				Required: true,
			},
			"waitVisible": {
				Type:     "string",
				Desc:     "等待页面中指定元素可见后抓取，默认body。如表格页可传#content或.table-class",
				Required: false,
			},
		},
		func(args string) (string, error) {
			parms := map[string]any{}
			if err := json.Unmarshal([]byte(args), &parms); err != nil {
				return "", err
			}
			url, ok := parms["url"].(string)
			if !ok || url == "" {
				return "URL不能为空", nil
			}
			client := resty.New()
			client.SetTimeout(30 * time.Second)
			resp, err := client.R().
				SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36").
				Get(url)
			if err != nil {
				return fmt.Sprintf("抓取失败: %v", err), nil
			}
			if resp.StatusCode() != 200 {
				return fmt.Sprintf("抓取失败: HTTP %d", resp.StatusCode()), nil
			}
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(resp.Body())))
			if err != nil {
				logger.SugaredLogger.Debugf("FetchWebPage goquery parse error: %v", err)
			}
			doc.Find("script, style, noscript, svg, meta, link, form, nav, footer, header, iframe, img, video, audio").
				Remove()
			html, _ := doc.Html()
			text := stripHTMLTags(html)
			text = strings.TrimSpace(text)
			if len(text) > 50000 {
				text = text[:50000] + "\n\n...（内容过长，已截断）"
			}
			return text, nil
		},
	)
}

func stripHTMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			sb.WriteRune(' ')
			continue
		}
		if !inTag {
			sb.WriteRune(r)
		}
	}
	result := sb.String()
	lines := strings.Split(result, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) > 2000 {
		out = out[:2000]
	}
	return strings.Join(out, "\n")
}
