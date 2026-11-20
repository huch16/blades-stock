package main

import (
	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/services"

	"context"
	"sync"
	"time"

	"github.com/coocood/freecache"
	cron "github.com/robfig/cron/v3"
)

// App struct
type App struct {
	ctx                context.Context
	cache              *freecache.Cache
	cron               *cron.Cron
	cronEntrys         map[string]cron.EntryID
	cronEntrysMu       sync.Mutex
	AiTools            []data.Tool
	summaryMu          sync.Mutex
	summaryCancel      context.CancelFunc
	stockAlertMu       sync.Mutex
	stockAlertLastSent map[string]time.Time
	priceAtAlertReset  map[string]float64
	memoryMgr          *agent.MemoryManager
	evaluator          *agent.Evaluator
	evolver            *agent.Evolver

	// [阶段 1 解耦服务委托层]
	marketService   *services.MarketService
	quantService    *services.QuantService
	agentService    *services.AgentService
	settingsService *services.SettingsService
}

// NewApp creates a new App application struct

// NewApp creates a new App application struct
func NewApp() *App {
	cacheSize := 512 * 1024
	cache := freecache.NewCache(cacheSize)
	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	c.Start()
	var tools []data.Tool
	tools = data.Tools(tools)
	return &App{
		cache:              cache,
		cron:               c,
		cronEntrys:         make(map[string]cron.EntryID),
		AiTools:            tools,
		stockAlertLastSent: make(map[string]time.Time),
		priceAtAlertReset:  make(map[string]float64),
		memoryMgr:          agent.NewMemoryManager(20),
		evaluator:          agent.NewEvaluator(),
		evolver:            agent.NewEvolver(),
		marketService:      services.NewMarketService(),
		quantService:       services.NewQuantService(),
		agentService:       services.NewAgentService(),
		settingsService:    services.NewSettingsService(),
	}
}
