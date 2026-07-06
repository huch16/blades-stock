package data

import (
	"context"
	"errors"
	"blades-stock/backend/logger"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserPool 浏览器池结构
type BrowserPool struct {
	pool chan *context.Context
	mu   sync.Mutex
	size int
}

// NewBrowserPool 创建新的浏览器池
func NewBrowserPool(size int) *BrowserPool {
	pool := make(chan *context.Context, size)
	for i := 0; i < size; i++ {
		path := GetSettingConfig().BrowserPath
		crawlTimeOut := GetSettingConfig().CrawlTimeOut
		if crawlTimeOut < 15 {
			crawlTimeOut = 30
		}
		if path != "" {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
			_ = cancel
			ctx, _ = chromedp.NewExecAllocator(
				ctx,
				chromedp.ExecPath(path),
				chromedp.Flag("headless", true),
				chromedp.Flag("blink-settings", "imagesEnabled=false"),
				chromedp.Flag("disable-javascript", false),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("disable-background-networking", true),
				chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
				chromedp.Flag("disable-background-timer-throttling", true),
				chromedp.Flag("disable-backgrounding-occluded-windows", true),
				chromedp.Flag("disable-breakpad", true),
				chromedp.Flag("disable-client-side-phishing-detection", true),
				chromedp.Flag("disable-default-apps", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.Flag("disable-extensions", true),
				chromedp.Flag("disable-features", "site-per-process,Translate,BlinkGenPropertyTrees"),
				chromedp.Flag("disable-hang-monitor", true),
				chromedp.Flag("disable-ipc-flooding-protection", true),
				chromedp.Flag("disable-popup-blocking", true),
				chromedp.Flag("disable-prompt-on-repost", true),
				chromedp.Flag("disable-renderer-backgrounding", true),
				chromedp.Flag("disable-sync", true),
				chromedp.Flag("force-color-profile", "srgb"),
				chromedp.Flag("metrics-recording-only", true),
				chromedp.Flag("safebrowsing-disable-auto-update", true),
				chromedp.Flag("enable-automation", true),
				chromedp.Flag("password-store", "basic"),
				chromedp.Flag("use-mock-keychain", true),
			)
			ctx, _ = chromedp.NewContext(ctx, chromedp.WithLogf(logger.SugaredLogger.Infof))
			pool <- &ctx
		} else {
			pool <- nil
		}
	}
	return &BrowserPool{
		pool: pool,
		size: size,
	}
}

// Get 从池中获取浏览器实例
// Get retrieves a browser instance from the pool. Although channel reads are safe,
// we acquire the mutex for symmetry and to guard against future modifications.
func (pool *BrowserPool) Get() *context.Context {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return <-pool.pool
}

// Put 将浏览器实例放回池中
func (pool *BrowserPool) Put(ctx *context.Context) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if ctx != nil {
		chromedp.Cancel(*ctx)
	}
	if len(pool.pool) >= pool.size {
		return
	}
	pool.pool <- ctx
}

func (pool *BrowserPool) Close() {
	close(pool.pool)
	for ctx := range pool.pool {
		if ctx != nil {
			chromedp.Cancel(*ctx)
		}
	}
}

// FetchPage 使用浏览器池获取页面内容
func (pool *BrowserPool) FetchPage(url, waitVisible string) (string, error) {
	ctx := pool.Get()
	defer pool.Put(ctx)
	if ctx == nil {
		return "", errors.New("浏览器未配置，请先在设置中配置BrowserPath")
	}
	var htmlContent string
	err := chromedp.Run(*ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(waitVisible, chromedp.ByQuery), // 确保  元素可见
		chromedp.WaitReady(waitVisible, chromedp.ByQuery),   // 确保  元素准备好
		chromedp.InnerHTML("body", &htmlContent),
		chromedp.Evaluate(`window.close()`, nil),
	)
	if err != nil {
		return "", err
	}
	return htmlContent, nil
}
