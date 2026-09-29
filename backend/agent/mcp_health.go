package agent

import (
	"context"
	"sync"
	"time"

	"blades-stock/backend/circuitbreaker"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	bladesmc "github.com/go-kratos/blades/contrib/mcp"
)

// MCPHealthPinger MCP 服务器健康检查器
type MCPHealthPinger struct {
	mu           sync.Mutex
	interval     time.Duration
	stopCh       chan struct{}
	running      bool
	healthStatus map[string]*MCPServerHealth // serverName -> health
}

type MCPServerHealth struct {
	ServerName            string
	LastCheck             time.Time
	LastSuccess           time.Time
	ConsecutiveFailures   int
	IsHealthy             bool
	LastError             string
}

// NewMCPHealthPinger 创建健康检查器
func NewMCPHealthPinger(interval time.Duration) *MCPHealthPinger {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &MCPHealthPinger{
		interval:     interval,
		stopCh:       make(chan struct{}),
		healthStatus: make(map[string]*MCPServerHealth),
	}
}

// Start 启动后台健康检查循环
func (p *MCPHealthPinger) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		return
	}
	p.running = true
	p.mu.Unlock()

	go p.run(ctx)
}

// Stop 停止健康检查
func (p *MCPHealthPinger) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	close(p.stopCh)
	p.running = false
	p.stopCh = make(chan struct{})
}

func (p *MCPHealthPinger) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// 立即执行一次
	p.checkAllServers(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkAllServers(ctx)
		}
	}
}

func (p *MCPHealthPinger) checkAllServers(ctx context.Context) {
	api := data.NewMCPServerApi()
	resp := api.List(&models.MCPServerQuery{
		Enable:    boolPtr(true),
		PageSize:  1000,
	})
	if resp == nil {
		logger.SugaredLogger.Warnf("获取 MCP 服务器列表失败")
		return
	}

	for _, server := range resp.Data {
		select {
		case <-p.stopCh:
			return
		default:
			p.checkSingleServer(ctx, server)
		}
	}
}

func (p *MCPHealthPinger) checkSingleServer(ctx context.Context, server models.MCPServer) {
	var transport bladesmc.TransportType
	var endpoint string
	var command string
	var args []string
	var env map[string]string
	var headers map[string]string

	switch {
	case server.Command != "":
		transport = bladesmc.TransportStdio
		command = server.Command
		args = parseMCPArgs(server.Args)
		env = parseMCPEnv(server.Env)
	case server.URL != "":
		transport = bladesmc.TransportHTTP
		endpoint = server.URL
		headers = parseMCPHeaders(server.Headers)
	default:
		logger.SugaredLogger.Warnf("MCP 服务器 [%s] 未配置 URL 或 Command，跳过", server.Name)
		return
	}

	mcpClient, err := bladesmc.NewClient(bladesmc.ClientConfig{
		Name:      server.Name,
		Transport: transport,
		Endpoint:  endpoint,
		Command:   command,
		Args:      args,
		Env:       env,
		Headers:   headers,
		Timeout:   10 * time.Second,
	})
	if err != nil {
		p.recordFailure(server.Name, err)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := mcpClient.Connect(ctx); err != nil {
		p.recordFailure(server.Name, err)
		return
	}

	_, err = mcpClient.Resolve(ctx)
	if err != nil {
		p.recordFailure(server.Name, err)
		return
	}

	// 探活成功
	p.recordSuccess(server.Name)
}

func (p *MCPHealthPinger) recordSuccess(serverName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	h, ok := p.healthStatus[serverName]
	if !ok {
		h = &MCPServerHealth{ServerName: serverName}
		p.healthStatus[serverName] = h
	}
	h.LastCheck = time.Now()
	h.LastSuccess = time.Now()
	h.ConsecutiveFailures = 0
	h.IsHealthy = true
	h.LastError = ""

	// 触发断路器恢复
	breaker := circuitbreaker.Get("mcp:" + serverName)
	breaker.Success()
	logger.SugaredLogger.Infof("MCP 服务器 [%s] 健康检查成功，断路器恢复", serverName)
}

func (p *MCPHealthPinger) recordFailure(serverName string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	h, ok := p.healthStatus[serverName]
	if !ok {
		h = &MCPServerHealth{ServerName: serverName}
		p.healthStatus[serverName] = h
	}
	h.LastCheck = time.Now()
	h.ConsecutiveFailures++
	h.LastError = err.Error()
	if h.ConsecutiveFailures >= 3 {
		h.IsHealthy = false
	}

	breaker := circuitbreaker.Get("mcp:" + serverName)
	breaker.Failure()
	logger.SugaredLogger.Warnf("MCP 服务器 [%s] 健康检查失败 (%d次连续失败): %v", serverName, h.ConsecutiveFailures, err)
}

// GetHealthStatus 获取指定服务器的健康状态
func (p *MCPHealthPinger) GetHealthStatus(serverName string) (*MCPServerHealth, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h, ok := p.healthStatus[serverName]
	return h, ok
}

// GetAllHealthStatus 获取所有服务器的健康状态
func (p *MCPHealthPinger) GetAllHealthStatus() map[string]*MCPServerHealth {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make(map[string]*MCPServerHealth, len(p.healthStatus))
	for k, v := range p.healthStatus {
		result[k] = v
	}
	return result
}

// IsHealthy 判断指定服务器是否健康
func (p *MCPHealthPinger) IsHealthy(serverName string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	h, ok := p.healthStatus[serverName]
	if !ok {
		return false // 未知视为不健康
	}
	return h.IsHealthy
}

// boolPtr 辅助函数
func boolPtr(b bool) *bool {
	return &b
}

var (
	globalPinger     *MCPHealthPinger
	globalPingerOnce sync.Once
)

// GetGlobalMCPHealthPinger 获取全局 MCP 健康检查单例
func GetGlobalMCPHealthPinger() *MCPHealthPinger {
	globalPingerOnce.Do(func() {
		globalPinger = NewMCPHealthPinger(60 * time.Second)
	})
	return globalPinger
}

// StartMCPHealthChecker 启动全局 MCP 健康探活循环
func StartMCPHealthChecker(ctx context.Context) {
	pinger := GetGlobalMCPHealthPinger()
	pinger.Start(ctx)
	logger.SugaredLogger.Info("MCP 健康检查后台探活服务已成功启动 (间隔 60s)")
}