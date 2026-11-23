package datasource

// K 线 WithContext 版回归测试（第五轮 A 组）：验证降级链的源级超时 ctx
// 真的接到了各数据源的 HTTP 请求上——ctx 超时后在途请求应立即中断、
// 方法快速返回空结果，而不是等客户端自身的构造期超时跑完。
// 全部用本地 stub Transport，不碰真实网络。

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

// blockingTransport 模拟挂起的网络请求：一直等到请求 ctx 被取消才返回。
type blockingTransport struct{}

func (blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestKLineWithContextCancelsInFlightRequest(t *testing.T) {
	blocking := resty.New().SetTransport(blockingTransport{})
	timeout := 100 * time.Millisecond

	t.Run("Sina", func(t *testing.T) {
		api := &SinaKLineApi{client: blocking}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		start := time.Now()
		k := api.GetKLineDataWithContext(ctx, "600519.SH", "101", 10)
		if k == nil || len(*k) != 0 {
			t.Errorf("cancelled Sina request want empty, got %v", k)
		}
		if took := time.Since(start); took > 5*time.Second {
			t.Errorf("Sina request was not aborted by ctx, took %v", took)
		}
	})

	t.Run("Tencent", func(t *testing.T) {
		api := &TencentKLineApi{client: blocking}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		start := time.Now()
		k := api.GetKLineDataWithContext(ctx, "600519.SH", "101", 10)
		if k == nil || len(*k) != 0 {
			t.Errorf("cancelled Tencent request want empty, got %v", k)
		}
		if took := time.Since(start); took > 5*time.Second {
			t.Errorf("Tencent request was not aborted by ctx, took %v", took)
		}
	})

	t.Run("EastMoney", func(t *testing.T) {
		api := &EastMoneyKLineApi{client: blocking}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		start := time.Now()
		k := api.GetKLineDataBeforeWithContext(ctx, "600519.SH", "101", "", 10, "20500101")
		if k == nil || len(*k) != 0 {
			t.Errorf("cancelled EastMoney request want empty, got %v", k)
		}
		if took := time.Since(start); took > 5*time.Second {
			t.Errorf("EastMoney request was not aborted by ctx, took %v", took)
		}
	})

	t.Run("AlreadyCancelled", func(t *testing.T) {
		// 已取消的 ctx：请求根本不应发出，立即返回空结果。
		api := &SinaKLineApi{client: blocking}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		k := api.GetKLineDataWithContext(ctx, "600519.SH", "101", 10)
		if k == nil || len(*k) != 0 {
			t.Errorf("already-cancelled Sina request want empty, got %v", k)
		}
		if took := time.Since(start); took > 5*time.Second {
			t.Errorf("already-cancelled request took %v", took)
		}
	})
}
