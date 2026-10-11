package data

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

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
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       60 * time.Second,
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

// Chaos injection for testing and resilience validation
var (
	chaosMutex       sync.RWMutex
	mockLatencyMs    int
	mockErrorRate    float64
	chaosEnabled     bool
)

// EnableChaos enables chaos injection globally
func EnableChaos() {
	chaosMutex.Lock()
	defer chaosMutex.Unlock()
	chaosEnabled = true
}

// DisableChaos disables chaos injection
func DisableChaos() {
	chaosMutex.Lock()
	defer chaosMutex.Unlock()
	chaosEnabled = false
	mockLatencyMs = 0
	mockErrorRate = 0
}

// SetMockLatency sets artificial latency in milliseconds
func SetMockLatency(ms int) {
	chaosMutex.Lock()
	defer chaosMutex.Unlock()
	if ms < 0 {
		ms = 0
	}
	mockLatencyMs = ms
}

// GetMockLatency returns current mock latency
func GetMockLatency() int {
	chaosMutex.RLock()
	defer chaosMutex.RUnlock()
	return mockLatencyMs
}

// SetMockErrorRate sets artificial error rate (0.0 - 1.0)
func SetMockErrorRate(rate float64) {
	chaosMutex.Lock()
	defer chaosMutex.Unlock()
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	mockErrorRate = rate
}

// GetMockErrorRate returns current mock error rate
func GetMockErrorRate() float64 {
	chaosMutex.RLock()
	defer chaosMutex.RUnlock()
	return mockErrorRate
}

// ApplyChaos applies configured chaos to the request context
// Returns an error if chaos should fail the request
func ApplyChaos() error {
	chaosMutex.RLock()
	enabled := chaosEnabled
	latency := mockLatencyMs
	errorRate := mockErrorRate
	chaosMutex.RUnlock()

	if !enabled {
		return nil
	}

	// Apply latency
	if latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	// Apply error injection
	if errorRate > 0 && rand.Float64() < errorRate {
		return fmt.Errorf("chaos injected error (rate=%.2f)", errorRate)
	}

	return nil
}

// CreateChaosHTTPClient creates an HTTP client with chaos injection
func CreateChaosHTTPClient(timeout time.Duration) *resty.Client {
	client := CreateHTTPClientWithTimeout(timeout)
	
	// Add middleware for chaos injection
	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		return ApplyChaos()
	})
	
	return client
}
