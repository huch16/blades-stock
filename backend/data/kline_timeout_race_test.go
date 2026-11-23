package data

// K 线并发超时回归测试（第五轮 A 组）：
//   - 共享 resty 客户端的 SetTimeout 竞态：原来 stock_kline.go /
//     tushare_data_api.go 每次请求都调 receiver.client.SetTimeout，而
//     receiver.client 就是包级共享 SharedHTTPClient；并发拉取时多个
//     goroutine 同时改写共享 http.Client.Timeout，既是 -race 可复现的
//     数据竞态、又会让某次请求用上别人的超时值。现改为请求级 context
//     超时（crawlTimeoutContext / SetContext），共享客户端不再被改动。
//   - runKLineSourceWithTimeout 孤儿 goroutine：原来超时后不等 worker
//     退出，worker 继续在后台跑网络请求；现超时即取消 ctx，在途的 resty
//     请求中断、worker 随即退出。
// 全部用本地 stub Transport，不碰真实网络/DB/外部 AI。

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

// stubKLineTransport 按目标 host 返回写死的 K 线响应体，供并发测试用。
type stubKLineTransport struct{}

func (stubKLineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body string
	switch {
	case strings.Contains(req.URL.Host, "quotes.sina.cn"):
		// Sina JSONP：GetKLineData 取首个 [ 到末个 ] 之间内容解析。
		body = `callback_123([{"day":"2026-01-02","open":"10.00","high":"11.00","low":"9.50","close":"10.50","volume":"1000"}])`
	case strings.Contains(req.URL.Host, "web.ifzq.gtimg.cn") && strings.Contains(req.URL.Path, "fqkline"):
		// 腾讯前复权：GetCommonKLineData 按入参 stockCode 查 dataMap。
		body = `{"code":0,"data":{"sh600519":{"qfqday":[["2026-01-02","10.00","10.50","11.00","9.50","1000"]]}}}`
	case strings.Contains(req.URL.Host, "web.ifzq.gtimg.cn") && strings.Contains(req.URL.Path, "minute"):
		// 腾讯分时：GetStockMinutePriceData 解析 m2["data"] 的 "0930 价 量"串。
		body = `{"code":0,"data":{"sh600519":{"data":{"date":"20260102","data":["0930 10.50 100"]}}}}`
	case strings.Contains(req.URL.Host, "api.tushare.pro"):
		body = `{"code":0,"msg":"","data":{"fields":["ts_code","trade_date","open","high","low","close","pre_close","change","pct_chg","vol","amount"],"items":[["600519.SH","20260102","10.00","11.00","9.50","10.50","10.00","0.5","5.0","1000","100000"]]}}`
	default:
		body = `{"code":-1}`
	}
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

// blockingKLineTransport 模拟挂起的网络请求：一直等到请求 ctx 被取消才返回，
// 用于验证请求级 context 超时真的能中断请求。
type blockingKLineTransport struct{}

func (blockingKLineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestStockKLineConcurrentSharedClientNoRace(t *testing.T) {
	// 一个共享 resty 客户端 + 三种不同超时：旧代码（每次请求 SetTimeout）
	// 下 -race 会报 http.Client.Timeout 的读写竞态；新代码不改客户端，
	// 应全程 clean，且每次请求都拿到自己那份数据。
	client := resty.New().SetTransport(stubKLineTransport{})
	api5 := &StockDataApi{client: client, config: &SettingConfig{Settings: &Settings{CrawlTimeOut: 5}}}
	api30 := &StockDataApi{client: client, config: &SettingConfig{Settings: &Settings{CrawlTimeOut: 30}}}
	tushare := &TushareApi{client: client, config: &SettingConfig{Settings: &Settings{CrawlTimeOut: 5, TushareToken: "test-token"}}}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			k := api5.GetKLineData("sh600519", "240", 30)
			if k == nil || len(*k) != 1 {
				t.Errorf("GetKLineData want 1 bar, got %v", k)
			}
		}()
		go func() {
			defer wg.Done()
			k := api30.GetCommonKLineData("sh600519", "101", 30)
			if k == nil || len(*k) != 1 {
				t.Errorf("GetCommonKLineData want 1 bar, got %v", k)
			}
		}()
		go func() {
			defer wg.Done()
			minutes, date := api5.GetStockMinutePriceData("sh600519")
			if minutes == nil || len(*minutes) != 1 || date != "20260102" {
				t.Errorf("GetStockMinutePriceData want 1 point date=20260102, got %v %q", minutes, date)
			}
		}()
		go func() {
			defer wg.Done()
			k := tushare.GetKLineData("600519.SH", "101", 10)
			if k == nil || len(*k) != 1 {
				t.Errorf("Tushare GetKLineData want 1 bar, got %v", k)
			}
		}()
	}
	wg.Wait()
}

func TestStockKLineRequestTimeoutCancels(t *testing.T) {
	// CrawlTimeOut=1s + 挂起的传输层：请求级 context 超时应在约 1s 中断
	// 请求、返回空结果；若超时未生效（例如又走回共享 client 的 SetTimeout
	// 而传输层不响应），本测试会在 5s 上限处失败。
	client := resty.New().SetTransport(blockingKLineTransport{})
	api := &StockDataApi{client: client, config: &SettingConfig{Settings: &Settings{CrawlTimeOut: 1}}}
	start := time.Now()
	k := api.GetKLineData("sh600519", "240", 30)
	elapsed := time.Since(start)
	if k == nil || len(*k) != 0 {
		t.Errorf("timed-out GetKLineData want empty, got %v", k)
	}
	if elapsed > 5*time.Second {
		t.Errorf("request was not aborted by context timeout, took %v", elapsed)
	}
}

func TestRunKLineSourceWithTimeoutCancelsWorker(t *testing.T) {
	// fn 在 ctx 取消前绝不返回：旧实现超时后 worker 会一直挂在 <-ctx.Done()
	// 上（真实场景里是挂在网络请求上），本测试断言 worker 在超时后很快退出。
	exited := make(chan struct{})
	fn := func(ctx context.Context) *KLineSourceResult {
		defer close(exited)
		<-ctx.Done()
		return &KLineSourceResult{Data: &[]KLineData{}}
	}
	start := time.Now()
	rs, ok := runKLineSourceWithTimeout("test-cancel", 50*time.Millisecond, "600519.SH", "101", fn)
	if rs != nil || ok {
		t.Fatalf("timeout path want (nil, false), got (%v, %v)", rs, ok)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("runKLineSourceWithTimeout did not return promptly on timeout, took %v", took)
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("worker goroutine did not exit after timeout+cancel (orphan goroutine)")
	}
}

func TestRunKLineSourceWithTimeoutSuccessAndPanic(t *testing.T) {
	want := &KLineSourceResult{Data: &[]KLineData{{Day: "2026-01-02"}}}
	rs, ok := runKLineSourceWithTimeout("test-ok", time.Second, "600519.SH", "101", func(ctx context.Context) *KLineSourceResult {
		return want
	})
	if !ok || rs != want {
		t.Fatalf("success path want (want, true), got (%v, %v)", rs, ok)
	}

	// panic 仍被捕获、返回 (nil, true)，由 tryKLineSource 按数据源失败处理。
	rs, ok = runKLineSourceWithTimeout("test-panic", time.Second, "600519.SH", "101", func(ctx context.Context) *KLineSourceResult {
		panic("boom")
	})
	if rs != nil || !ok {
		t.Fatalf("panic path want (nil, true), got (%v, %v)", rs, ok)
	}
}
