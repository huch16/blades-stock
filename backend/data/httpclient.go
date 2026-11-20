package data

import (
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// FirefoxUserAgent 是 market_news_api.go 内 19 处共用的 UA 字面量。
// 注意：该文件另有 12 处 UA 是各端点专用的 Chrome/Edge/Firefox 字面量
// （版本号不同），刻意不同，不得统一成这一个常量。
const FirefoxUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0"

var (
	sharedTransport  *http.Transport
	sharedHTTPClient *http.Client
	SharedHTTPClient *resty.Client
	httpConfigMutex  sync.RWMutex
	currentProxyURL  string
)

func init() {
	sharedTransport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   25,
		MaxConnsPerHost:       50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 nil,
	}

	sharedHTTPClient = &http.Client{
		Transport: sharedTransport,
		Timeout:   300 * time.Second,
	}

	SharedHTTPClient = resty.NewWithClient(sharedHTTPClient).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		SetRetryCount(0).
		SetTimeout(300 * time.Second)
}

func UpdateHTTPClientProxy(proxyURL string) {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	if proxyURL == "" || proxyURL == currentProxyURL {
		return
	}

	sharedTransport.Proxy = http.ProxyURL(parseProxyURL(proxyURL))
	currentProxyURL = proxyURL
}

func DisableHTTPClientProxy() {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	sharedTransport.Proxy = nil
	currentProxyURL = ""
}

func UpdateHTTPClientTimeout(timeout time.Duration) {
	sharedHTTPClient.Timeout = timeout
	SharedHTTPClient.SetTimeout(timeout)
}

func parseProxyURL(proxyURL string) *url.URL {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}
	return u
}

func ConfigureFromSettings(config *SettingConfig) {
	if config == nil {
		return
	}

	if config.HttpProxyEnabled && config.HttpProxy != "" {
		UpdateHTTPClientProxy(config.HttpProxy)
	} else {
		DisableHTTPClientProxy()
	}

	if config.CrawlTimeOut > 0 {
		UpdateHTTPClientTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	} else {
		UpdateHTTPClientTimeout(300 * time.Second)
	}
}

func CreateHTTPClientWithTimeout(timeout time.Duration) *resty.Client {
	httpConfigMutex.RLock()
	transport := sharedTransport
	httpConfigMutex.RUnlock()

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return resty.NewWithClient(httpClient).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		SetTimeout(timeout).
		SetRetryCount(0)
}

func CreateDownloadClient() *resty.Client {
	httpConfigMutex.RLock()
	transport := sharedTransport
	httpConfigMutex.RUnlock()

	downloadTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          5,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       2,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 transport.Proxy,
	}

	downloadHTTPClient := &http.Client{
		Transport: downloadTransport,
		Timeout:   0,
	}

	return resty.NewWithClient(downloadHTTPClient).
		SetTimeout(0).
		SetRetryCount(2).
		SetRetryWaitTime(5 * time.Second).
		SetRetryMaxWaitTime(30 * time.Second)
}
