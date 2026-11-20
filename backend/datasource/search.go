package datasource

import (
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/util"

	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/duke-git/lancet/v2/mathutil"
)

// @Author spark
// @Date 2025/6/28 21:02
// @Desc
// -----------------------------------------------------------------------------------
type SearchStockApi struct {
	words string
	// qgqpBId 为东财用户标识（请求 cookie 中的 qgqp_b_id），由调用方
	// （backend/data 的转发函数）从设置传入；本包不读设置表。
	qgqpBId string
}

// NewSearchStockApi 构造搜索客户端。qgqpBId 由调用方从设置传入
// （空时 SearchStock/SearchBk/SearchETF 返回提示信息，与原行为一致）。
func NewSearchStockApi(words, qgqpBId string) *SearchStockApi {
	return &SearchStockApi{words: words, qgqpBId: qgqpBId}
}
func (s SearchStockApi) SearchStock(pageSize int) map[string]any {
	qgqpBId := s.qgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	url := "https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code"
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(fmt.Sprintf(`{
				"keyWord": "%s",
				"pageSize": %d,
				"pageNo": 1,
				"fingerprint": "%s",
				"gids": [],
				"matchWord": "",
				"timestamp": "%d",
				"shareToGuba": false,
				"requestId": "",
				"needCorrect": true,
				"removedConditionIdList": [],
				"xcId": "",
				"ownSelectAll": false,
				"dxInfo": [],
				"extraCondition": ""
				}`, s.words, pageSize, qgqpBId, time.Now().Unix())).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}

func (s SearchStockApi) SearchBk(pageSize int) map[string]any {
	url := "https://np-tjxg-b.eastmoney.com/api/smart-tag/bkc/v3/pw/search-code"
	qgqpBId := s.qgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(fmt.Sprintf(`{
				"keyWord": "%s",
				"pageSize": %d,
				"pageNo": 1,
				"fingerprint": "%s",
				"gids": [],
				"matchWord": "",
				"timestamp": "%d",
				"shareToGuba": false,
				"requestId": "",
				"needCorrect": true,
				"removedConditionIdList": [],
				"xcId": "",
				"ownSelectAll": false,
				"dxInfo": [],
				"extraCondition": ""
				}`, s.words, pageSize, qgqpBId, time.Now().Unix())).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}

func (s SearchStockApi) SearchETF(pageSize int) map[string]any {
	url := "https://np-tjxg-b.eastmoney.com/api/smart-tag/etf/v3/pw/search-code"
	qgqpBId := s.qgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(fmt.Sprintf(`{
				"keyWord": "%s",
				"pageSize": %d,
				"pageNo": 1,
				"fingerprint": "%s",
				"gids": [],
				"matchWord": "",
				"timestamp": "%d",
				"shareToGuba": false,
				"requestId": "",
				"needCorrect": true,
				"removedConditionIdList": [],
				"xcId": "",
				"ownSelectAll": false,
				"dxInfo": [],
				"extraCondition": ""
				}`, s.words, pageSize, qgqpBId, time.Now().Unix())).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchETF-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}

func (s SearchStockApi) HotStrategy() map[string]any {
	url := fmt.Sprintf("https://np-ipick.eastmoney.com/recommend/stock/heat/ranking?count=20&trace=%d&client=web&biz=web_smart_tag", time.Now().Unix())
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-ipick.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotStrategy-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	return respMap
}

func (s SearchStockApi) HotStrategyTable() string {
	markdownTable := ""
	res := s.HotStrategy()
	bytes, _ := json.Marshal(res)
	strategy := &models.HotStrategy{}
	json.Unmarshal(bytes, strategy)
	for _, data := range strategy.Data {
		data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
	}
	markdownTable = util.MarkdownTableWithTitle("当前热门选股策略", strategy.Data)
	return markdownTable
}

func (s SearchStockApi) StrategySquare() map[string]any {
	//https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword=
	url := "https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword="
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "backtest.10jqka.com.cn").
		SetHeader("Origin", "https://backtest.10jqka.com.cn").
		SetHeader("Referer", "https://backtest.10jqka.com.cn/strategysquare/list").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("StrategySquare-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}

// ── 网络搜索（SearXNG / 百度 HTTP / 百度浏览器）──────────────────────────────
// R-25g 自 backend/data/web_search_api.go 移入。
type WebSearchResult struct {
	Title       string `json:"title"`
	Url         string `json:"url"`
	Snippet     string `json:"snippet"`
	Source      string `json:"source"`
	PublishTime string `json:"publish_time"`
}

type BrowserInstance struct {
	allocatorCtx    context.Context
	allocatorCancel context.CancelFunc
	browserCtx      context.Context
	browserCancel   context.CancelFunc
	mu              sync.Mutex
	lastUsed        time.Time
}

type BrowserManager struct {
	instance *BrowserInstance
	mu       sync.RWMutex
}

var browserManager = &BrowserManager{}

func GetBrowserManager() *BrowserManager {
	return browserManager
}

const browserIdleTimeout = 5 * time.Minute

func init() {
	go func() {
		for {
			time.Sleep(browserIdleTimeout)
			bm := GetBrowserManager()
			bm.mu.RLock()
			inst := bm.instance
			bm.mu.RUnlock()
			if inst == nil {
				continue
			}
			inst.mu.Lock()
			idle := time.Since(inst.lastUsed)
			inst.mu.Unlock()
			if idle >= browserIdleTimeout {
				logger.SugaredLogger.Infof("浏览器空闲 %v，自动关闭", idle)
				bm.CloseBrowser()
			}
		}
	}()
}

// GetOrCreateBrowser 获取或创建浏览器实例。browserPath 为浏览器可执行文件路径，
// crawlTimeoutSec 为启动超时（秒，<=0 取 60）；由调用方传入，本包不读设置表。
func (bm *BrowserManager) GetOrCreateBrowser(browserPath string, crawlTimeoutSec int64) (*BrowserInstance, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.instance != nil {
		bm.instance.mu.Lock()
		if bm.instance.browserCtx != nil {
			bm.instance.lastUsed = time.Now()
			bm.instance.mu.Unlock()
			return bm.instance, nil
		}
		bm.instance.mu.Unlock()
	}

	path := browserPath
	if path == "" {
		return nil, fmt.Errorf("BrowserPath not configured")
	}

	timeout := crawlTimeoutSec
	if timeout <= 0 {
		timeout = 60
	}

	allocatorCtx, allocatorCancel := getStealthAllocator(context.Background(), path, true)

	browserCtx, browserCancel := chromedp.NewContext(allocatorCtx, chromedp.WithLogf(logger.SugaredLogger.Infof))

	err := chromedp.Run(browserCtx)
	if err != nil {
		allocatorCancel()
		browserCancel()
		return nil, fmt.Errorf("failed to initialize browser: %v", err)
	}

	bm.instance = &BrowserInstance{
		allocatorCtx:    allocatorCtx,
		allocatorCancel: allocatorCancel,
		browserCtx:      browserCtx,
		browserCancel:   browserCancel,
		lastUsed:        time.Now(),
	}

	logger.SugaredLogger.Infof("Created new browser instance")
	return bm.instance, nil
}

func (bi *BrowserInstance) NewTab() (context.Context, context.CancelFunc) {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	bi.lastUsed = time.Now()
	tabCtx, tabCancel := chromedp.NewContext(bi.browserCtx)
	return tabCtx, tabCancel
}

func (bi *BrowserInstance) Close() {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	if bi.browserCancel != nil {
		bi.browserCancel()
	}
	if bi.allocatorCancel != nil {
		bi.allocatorCancel()
	}
	bi.browserCtx = nil
	bi.allocatorCtx = nil
}

func (bm *BrowserManager) CloseBrowser() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.instance != nil {
		bm.instance.Close()
		bm.instance = nil
		logger.SugaredLogger.Infof("Browser instance closed")
	}
}

func (bm *BrowserManager) ResetBrowser() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.instance != nil {
		bm.instance.Close()
		bm.instance = nil
	}
}

type WebSearchApi struct {
	timeout int
	// 以下三个提供函数由调用方（backend/data 的转发函数）注入，
	// 每次搜索时读最新设置；本包不读设置表。
	disableBrowserSearch func() bool
	browserPath          func() string
	crawlTimeout         func() int64
}

// NewWebSearchApi 构造网络搜索客户端。timeout 为 HTTP 超时（秒，<=0 取 30）；
// disableBrowserSearch/browserPath/crawlTimeout 为设置提供函数，
// 由调用方（backend/data 的转发函数）注入，每次搜索时读最新设置。
func NewWebSearchApi(timeout int, disableBrowserSearch func() bool, browserPath func() string, crawlTimeout func() int64) *WebSearchApi {
	if timeout <= 0 {
		timeout = 30
	}
	return &WebSearchApi{timeout: timeout, disableBrowserSearch: disableBrowserSearch, browserPath: browserPath, crawlTimeout: crawlTimeout}
}

const defaultSearxngURL = "https://search.woha.men"

func GetSearXNGEndpoints() []string {
	var endpoints []string
	if custom := os.Getenv("SEARXNG_URL"); custom != "" {
		endpoints = append(endpoints, strings.TrimRight(custom, "/"))
	}
	endpoints = append(endpoints, defaultSearxngURL, "https://search.sapti.me", "https://searx.be")
	return endpoints
}

func (s *WebSearchApi) Search(query string, maxResults int) []WebSearchResult {
	if maxResults <= 0 {
		maxResults = 10
	}

	results := s.SearchSearXNG(query, maxResults)
	if len(results) > 0 {
		return results
	}

	logger.SugaredLogger.Debugf("SearXNG无结果，尝试百度HTTP搜索: %s", query)
	results = s.searchBaiduHTTP(query, maxResults)
	if len(results) > 0 {
		return results
	}

	if s.disableBrowserSearch() {
		logger.SugaredLogger.Debugf("浏览器搜索已禁用，跳过: %s", query)
		return results
	}

	logger.SugaredLogger.Debugf("HTTP搜索无结果，尝试浏览器搜索: %s", query)
	browser, err := GetBrowserManager().GetOrCreateBrowser(s.browserPath(), s.crawlTimeout())
	if err != nil {
		logger.SugaredLogger.Debugf("浏览器不可用: %v", err)
		return results
	}

	return s.searchBaiduWithBrowser(browser, query, maxResults)
}

func (s *WebSearchApi) SearchSearXNG(query string, maxResults int) []WebSearchResult {
	client := &http.Client{Timeout: 8 * time.Second}
	endpoints := GetSearXNGEndpoints()

	for _, endpoint := range endpoints {
		searchURL := fmt.Sprintf("%s/search?q=%s&format=json&language=zh-CN", endpoint, url.QueryEscape(query))
		req, err := http.NewRequest("GET", searchURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			logger.SugaredLogger.Warnf("SearXNG节点 [%s] 请求失败: %v", endpoint, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var searxngResp struct {
			Results []struct {
				URL     string `json:"url"`
				Title   string `json:"title"`
				Content string `json:"content"`
				Engine  string `json:"engine"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &searxngResp); err != nil {
			logger.SugaredLogger.Warnf("SearXNG节点 [%s] 解析失败: %v", endpoint, err)
			continue
		}

		var results []WebSearchResult
		for i, r := range searxngResp.Results {
			if i >= maxResults {
				break
			}
			results = append(results, WebSearchResult{
				Title:   r.Title,
				Url:     r.URL,
				Snippet: r.Content,
				Source:  "SearXNG/" + r.Engine,
			})
		}

		if len(results) > 0 {
			logger.SugaredLogger.Infof("SearXNG节点 [%s] 搜索到 %d 条结果: %s", endpoint, len(results), query)
			return results
		}
	}
	return nil
}

// searchBaiduHTTP 使用简单 HTTP 请求 + goquery 解析百度搜索结果，作为 Chrome 不可用时的降级方案。
func (s *WebSearchApi) searchBaiduHTTP(query string, maxResults int) []WebSearchResult {
	client := &http.Client{Timeout: 15 * time.Second}
	encodedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("https://www.baidu.com/s?wd=%s&ie=utf-8", encodedQuery)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		logger.SugaredLogger.Warnf("HTTP百度搜索请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	results := s.ParseBaiduResultsWithGoquery(string(body), maxResults)
	logger.SugaredLogger.Infof("HTTP百度搜索到 %d 条结果（降级）: %s", len(results), query)
	return results
}

func (s *WebSearchApi) searchBaiduWithBrowser(browser *BrowserInstance, query string, maxResults int) []WebSearchResult {
	var results []WebSearchResult

	tabCtx, tabCancel := browser.NewTab()
	defer tabCancel()

	encodedQuery := url.QueryEscape(query)
	searchUrl := fmt.Sprintf("https://www.baidu.com/s?wd=%s", encodedQuery)

	var htmlContent string
	err := chromedp.Run(tabCtx,
		chromedp.Navigate("https://www.baidu.com"),
		chromedp.Sleep(RandomSleep(1000, 2000)),
		stealthScripts(),
		chromedp.Sleep(RandomSleep(500, 1000)),
		chromedp.Navigate(searchUrl),
		chromedp.Sleep(RandomSleep(2000, 3000)),
		chromedp.WaitVisible("#content_left", chromedp.ByID),
		chromedp.Sleep(RandomSleep(1000, 2000)),
		chromedp.InnerHTML("#content_left", &htmlContent),
	)
	if err != nil {
		logger.SugaredLogger.Errorf("Baidu search failed: %v", err)
		return results
	}

	results = s.ParseBaiduResultsWithGoquery(htmlContent, maxResults)
	logger.SugaredLogger.Infof("Baidu search found %d results for query: %s", len(results), query)

	return results
}

func (s *WebSearchApi) ParseBaiduResultsWithGoquery(htmlContent string, maxResults int) []WebSearchResult {
	var results []WebSearchResult

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("Failed to parse Baidu HTML: %v", err)
		return results
	}

	doc.Find(".result, .c-container").Each(func(i int, container *goquery.Selection) {
		if i >= maxResults {
			return
		}

		result := WebSearchResult{Source: "Baidu"}

		container.Find("h3 a, h3.t a").First().Each(func(_ int, a *goquery.Selection) {
			href, exists := a.Attr("href")
			if exists {
				result.Url = href
				result.Title = strings.TrimSpace(a.Text())
			}
		})

		if result.Url == "" {
			container.Find("a[data-click]").First().Each(func(_ int, a *goquery.Selection) {
				href, exists := a.Attr("href")
				if exists {
					result.Url = href
					a.Find("h3").First().Each(func(_ int, h3 *goquery.Selection) {
						result.Title = strings.TrimSpace(h3.Text())
					})
				}
			})
		}

		if result.Url == "" {
			return
		}

		container.Find(".c-abstract, .content-right, .c-span-last").First().Each(func(_ int, el *goquery.Selection) {
			text := strings.TrimSpace(el.Text())
			if len(text) > 10 {
				result.Snippet = text
			}
		})

		if result.Snippet == "" {
			container.Find(".c-row, p.content").First().Each(func(_ int, el *goquery.Selection) {
				text := strings.TrimSpace(el.Text())
				if len(text) > 10 {
					result.Snippet = text
				}
			})
		}

		results = append(results, result)
	})

	logger.SugaredLogger.Debugf("Baidu: found %d results using goquery", len(results))
	return results
}

//lint:ignore U1000 cleanText is unused but kept for possible future use
func CleanText(text string) string {
	text = strings.ReplaceAll(text, "\u00A0", " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return strings.TrimSpace(text)
}

func getStealthAllocator(ctx context.Context, path string, headless bool) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(path),
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process,ImprovedCookieControls,LazyFrameLoading,GlobalMediaControls,DestroyProfileOnBrowserClose"),
		chromedp.Flag("disable-background-networking", false),
		chromedp.Flag("disable-background-timer-throttling", false),
		chromedp.Flag("disable-backgrounding-occluded-windows", false),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-component-extensions-with-background-pages", false),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", false),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", false),
		chromedp.Flag("disable-popup-blocking", false),
		chromedp.Flag("disable-prompt-on-repost", false),
		chromedp.Flag("disable-renderer-backgrounding", false),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess,TrustTokens,TrustTokensAlwaysAllowIssuance"),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("start-maximized", false),
		chromedp.Flag("lang", "zh-CN"),
		chromedp.Flag("accept-lang", "zh-CN,zh;q=0.9,en;q=0.8"),
		chromedp.Flag("blink-settings", "imagesEnabled=true"),
		chromedp.Flag("intl-accept-languages", "zh-CN,zh,en"),
		chromedp.Flag("locale", "zh-CN"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
	)

	return chromedp.NewExecAllocator(ctx, opts...)
}

func stealthScripts() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		err := chromedp.Evaluate(`
			Object.defineProperty(navigator, 'webdriver', {
				get: () => undefined
			});
			Object.defineProperty(navigator, 'plugins', {
				get: () => [
					{filename: 'internal-pdf-viewer', name: 'Chrome PDF Plugin', description: 'Portable Document Format'},
					{filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', name: 'Chrome PDF Viewer', description: ''},
					{filename: 'internal-nacl-plugin', name: 'Native Client', description: ''}
				]
			});
			Object.defineProperty(navigator, 'language', {
				get: () => 'zh-CN'
			});
			Object.defineProperty(navigator, 'languages', {
				get: () => ['zh-CN', 'zh', 'zh-TW', 'en']
			});
			Object.defineProperty(navigator, 'platform', {
				get: () => 'Win32'
			});
			Object.defineProperty(navigator, 'hardwareConcurrency', {
				get: () => 8
			});
			Object.defineProperty(navigator, 'deviceMemory', {
				get: () => 8
			});
			Object.defineProperty(navigator, 'locale', {
				get: () => 'zh-CN'
			});
			window.chrome = {
				runtime: {},
				loadTimes: function() {},
				csi: function() {},
				app: {}
			};
			Object.defineProperty(navigator, 'permissions', {
				get: () => ({
					query: () => Promise.resolve({state: 'granted'})
				})
			});
			Object.defineProperty(Intl.DateTimeFormat.prototype, 'resolvedOptions', {
				value: function() {
					return {
						locale: 'zh-CN',
						calendar: 'gregory',
						numberingSystem: 'latn',
						timeZone: 'Asia/Shanghai',
						year: 'numeric',
						month: '2-digit',
						day: '2-digit'
					};
				}
			});
			Object.defineProperty(Intl.NumberFormat.prototype, 'resolvedOptions', {
				value: function() {
					return {
						locale: 'zh-CN',
						numberingSystem: 'latn',
						style: 'decimal',
						useGrouping: true
					};
				}
			});
			Object.defineProperty(Intl.Collator.prototype, 'resolvedOptions', {
				value: function() {
					return {
						locale: 'zh-CN',
						usage: 'sort',
						sensitivity: 'variant',
						ignorePunctuation: false,
						collation: 'pinyin',
						numeric: false,
						caseFirst: 'false'
					};
				}
			});
			const originalGetTimezoneOffset = Date.prototype.getTimezoneOffset;
			Date.prototype.getTimezoneOffset = function() {
				return -480;
			};
			const originalDateTimeFormat = Intl.DateTimeFormat;
			Intl.DateTimeFormat = function(locale, options) {
				if (!locale) locale = 'zh-CN';
				return new originalDateTimeFormat(locale, options);
			};
			Intl.DateTimeFormat.prototype = originalDateTimeFormat.prototype;
			Intl.DateTimeFormat.supportedLocalesOf = originalDateTimeFormat.supportedLocalesOf;
		`, nil).Do(ctx)
		return err
	})
}

func RandomSleep(minMs, maxMs int) time.Duration {
	if maxMs <= minMs {
		return time.Duration(minMs) * time.Millisecond
	}
	delta := rand.Intn(maxMs - minMs)
	return time.Duration(minMs+delta) * time.Millisecond
}

func (s *WebSearchApi) SearchToMarkdown(query string, maxResults int) string {
	results := s.Search(query, maxResults)
	if len(results) == 0 {
		return "未找到相关搜索结果"
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("## 搜索结果: %s\n\n", query))
	md.WriteString(fmt.Sprintf("共找到 %d 条结果\n\n", len(results)))

	for i, r := range results {
		md.WriteString(fmt.Sprintf("### %d. %s\n", i+1, r.Title))
		md.WriteString(fmt.Sprintf("- 来源: %s\n", r.Source))
		if r.PublishTime != "" {
			md.WriteString(fmt.Sprintf("- 时间: %s\n", r.PublishTime))
		}
		md.WriteString(fmt.Sprintf("- 链接: %s\n", r.Url))
		if r.Snippet != "" {
			md.WriteString(fmt.Sprintf("- 摘要: %s\n", r.Snippet))
		}
		md.WriteString("\n")
	}

	return md.String()
}
