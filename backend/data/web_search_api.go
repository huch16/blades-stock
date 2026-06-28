package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

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

func (bm *BrowserManager) GetOrCreateBrowser() (*BrowserInstance, error) {
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

	path := GetSettingConfig().BrowserPath
	if path == "" {
		return nil, fmt.Errorf("BrowserPath not configured")
	}

	timeout := GetSettingConfig().CrawlTimeOut
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
}

func NewWebSearchApi(timeout int) *WebSearchApi {
	if timeout <= 0 {
		timeout = 30
	}
	return &WebSearchApi{timeout: timeout}
}

func (s *WebSearchApi) Search(query string, maxResults int) []WebSearchResult {
	if maxResults <= 0 {
		maxResults = 10
	}

	cfg := GetSettingConfig()
	searxngUrl := strings.TrimSpace(cfg.SearxngUrl)
	if searxngUrl == "" {
		searxngUrl = "https://search.whohai.cfd"
	}
	useJson := cfg.SearxngJsonFormat || true // default to JSON API for private SearXNG

	// First try SearXNG JSON API directly (avoids chromedp 401/bot detection)
	results := s.searchSearxngWithHTTP(searxngUrl, query, maxResults, useJson)
	if len(results) >= maxResults {
		return results
	}

	// Fallback to browser-based engines if JSON API returns nothing
	browser, err := GetBrowserManager().GetOrCreateBrowser()
	if err != nil {
		logger.SugaredLogger.Warnf("Failed to get browser: %v", err)
		return results
	}

	if len(results) < maxResults {
		whohaiResults := s.searchWhohaiWithBrowser(browser, query, maxResults-len(results), searxngUrl)
		results = append(results, whohaiResults...)
	}
	if len(results) < maxResults {
		baiduResults := s.searchBaiduWithBrowser(browser, query, maxResults-len(results))
		results = append(results, baiduResults...)
	}
	return results
}

func (s *WebSearchApi) searchSearxngWithHTTP(searxngUrl, query string, maxResults int, jsonFormat bool) []WebSearchResult {
	var results []WebSearchResult

	u, err := url.Parse(strings.TrimRight(searxngUrl, "/"))
	if err != nil {
		logger.SugaredLogger.Errorf("Invalid SearXNG URL %q: %v", searxngUrl, err)
		return results
	}
	u.Path = "/search"
	q := u.Query()
	q.Set("q", query)
	if jsonFormat {
		q.Set("format", "json")
	}
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: time.Duration(s.timeout) * time.Second}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		logger.SugaredLogger.Errorf("SearXNG HTTP request build failed: %v", err)
		return results
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/html")

	resp, err := client.Do(req)
	if err != nil {
		logger.SugaredLogger.Errorf("SearXNG HTTP request failed: %v", err)
		return results
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.SugaredLogger.Errorf("SearXNG returned status %d", resp.StatusCode)
		return results
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.SugaredLogger.Errorf("SearXNG read body failed: %v", err)
		return results
	}

	if jsonFormat {
		results = s.parseSearxngJSON(body, maxResults)
	} else {
		results = s.parseBingResultsWithGoquery(string(body), maxResults)
	}
	logger.SugaredLogger.Infof("SearXNG HTTP search found %d results for query: %s", len(results), query)
	return results
}

func (s *WebSearchApi) parseSearxngJSON(body []byte, maxResults int) []WebSearchResult {
	var results []WebSearchResult

	type searxngResult struct {
		Title   string `json:"title"`
		Url     string `json:"url"`
		Content string `json:"content"`
		Engine  string `json:"engine"`
	}
	var payload struct {
		Results []searxngResult `json:"results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		logger.SugaredLogger.Errorf("SearXNG JSON parse failed: %v", err)
		return results
	}

	for i, r := range payload.Results {
		if i >= maxResults {
			break
		}
		results = append(results, WebSearchResult{
			Title:   r.Title,
			Url:     r.Url,
			Snippet: r.Content,
			Source:  r.Engine,
		})
	}
	return results
}

func (s *WebSearchApi) searchWhohaiWithBrowser(browser *BrowserInstance, query string, maxResults int, searxngUrl string) []WebSearchResult {
	var results []WebSearchResult

	tabCtx, tabCancel := browser.NewTab()
	defer tabCancel()

	encoded := url.QueryEscape(query)
	searchUrl := fmt.Sprintf("%s/?q=%s", strings.TrimRight(searxngUrl, "/"), encoded)

	var html string
	err := chromedp.Run(tabCtx,
		chromedp.Navigate(searchUrl),
		chromedp.Sleep(randomSleep(2000, 3000)),
		chromedp.InnerHTML("body", &html, chromedp.ByQuery),
	)
	if err != nil {
		logger.SugaredLogger.Errorf("Whohai browser search failed: %v", err)
		return results
	}

	results = s.parseBingResultsWithGoquery(html, maxResults)
	logger.SugaredLogger.Infof("Whohai browser search found %d results for query: %s", len(results), query)
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
		chromedp.Sleep(randomSleep(1000, 2000)),
		stealthScripts(),
		chromedp.Sleep(randomSleep(500, 1000)),
		chromedp.Navigate(searchUrl),
		chromedp.Sleep(randomSleep(2000, 3000)),
		chromedp.WaitVisible("#content_left", chromedp.ByID),
		chromedp.Sleep(randomSleep(1000, 2000)),
		chromedp.InnerHTML("#content_left", &htmlContent),
	)
	if err != nil {
		logger.SugaredLogger.Errorf("Baidu search failed: %v", err)
		return results
	}

	results = s.parseBaiduResultsWithGoquery(htmlContent, maxResults)
	logger.SugaredLogger.Infof("Baidu search found %d results for query: %s", len(results), query)

	return results
}

func (s *WebSearchApi) parseBingResultsWithGoquery(htmlContent string, maxResults int) []WebSearchResult {
	var results []WebSearchResult

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("Failed to parse Bing HTML: %v", err)
		return results
	}

	doc.Find("li.b_algo").Each(func(i int, li *goquery.Selection) {
		if i >= maxResults {
			return
		}

		result := WebSearchResult{Source: "Bing"}

		li.Find("h2 a").First().Each(func(_ int, a *goquery.Selection) {
			href, exists := a.Attr("href")
			if exists {
				result.Url = href
				result.Title = strings.TrimSpace(a.Text())
			}
		})

		if result.Url == "" {
			return
		}

		li.Find(".b_caption p").First().Each(func(_ int, p *goquery.Selection) {
			result.Snippet = strings.TrimSpace(p.Text())
		})

		if result.Snippet == "" {
			li.Find(".b_paractl").First().Each(func(_ int, p *goquery.Selection) {
				result.Snippet = strings.TrimSpace(p.Text())
			})
		}

		li.Find(".b_attribution, .news, .date").First().Each(func(_ int, span *goquery.Selection) {
			result.PublishTime = strings.TrimSpace(span.Text())
		})

		results = append(results, result)
	})

	logger.SugaredLogger.Debugf("Bing: found %d results using goquery", len(results))
	return results
}

func (s *WebSearchApi) parseBaiduResultsWithGoquery(htmlContent string, maxResults int) []WebSearchResult {
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
func cleanText(text string) string {
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

func randomSleep(minMs, maxMs int) time.Duration {
	if maxMs <= minMs {
		return time.Duration(minMs) * time.Millisecond
	}
	delta := rand.Intn(maxMs - minMs)
	return time.Duration(minMs+delta) * time.Millisecond
}

func (s *WebSearchApi) SearchToJson(query string, maxResults int) string {
	results := s.Search(query, maxResults)
	if len(results) == 0 {
		return "未找到相关搜索结果"
	}
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Sprintf("搜索结果序列化失败: %v", err)
	}
	return string(jsonData)
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
