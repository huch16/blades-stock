package datasource

import (
	"io"
	"log"
	"strings"
	"sync"
	"time"

	gotdx "github.com/bensema/gotdx"
	"github.com/bensema/gotdx/types"
)

func init() {
	log.SetOutput(io.Discard)
}

type TdxKLineApi struct {
	client      *gotdx.Client
	macClient   *gotdx.Client
	macExClient *gotdx.Client
	// crawlTimeout 由调用方注入的超时提供函数（每次新建连接时调用，
	// 读最新设置）；本包不读设置表。
	crawlTimeout func() int64
	mu           sync.Mutex // 保护 client
	macMu        sync.Mutex // 保护 macClient
	macExMu      sync.Mutex // 保护 macExClient
}

var (
	tdxApiInstance *TdxKLineApi
	tdxApiOnce     sync.Once
)

// NewTdxKLineApi 返回通达信客户端单例。crawlTimeout 为爬取超时（秒）提供函数，
// 由调用方（backend/data 的转发函数）注入，每次新建连接时调用以读最新设置；
// 本包不读设置表。
func NewTdxKLineApi(crawlTimeout func() int64) *TdxKLineApi {
	tdxApiOnce.Do(func() {
		tdxApiInstance = &TdxKLineApi{crawlTimeout: crawlTimeout}
	})
	return tdxApiInstance
}

func (t *TdxKLineApi) newClient() *gotdx.Client {
	timeoutSec := t.crawlTimeout()
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return gotdx.New(
		gotdx.WithAutoSelectFastest(true),
		gotdx.WithTimeoutSec(int(timeoutSec)),
	)
}

func (t *TdxKLineApi) newMACClient() *gotdx.Client {
	timeoutSec := t.crawlTimeout()
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return gotdx.NewMAC(
		gotdx.WithAutoSelectFastest(true),
		gotdx.WithTimeoutSec(int(timeoutSec)),
	)
}

func (t *TdxKLineApi) newMACExClient() *gotdx.Client {
	timeoutSec := t.crawlTimeout()
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return gotdx.NewMACEx(
		gotdx.WithAutoSelectFastest(true),
		gotdx.WithTimeoutSec(int(timeoutSec)),
	)
}

func (t *TdxKLineApi) ensureClient() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client == nil {
		t.client = t.newClient()
	}
	return nil
}

func (t *TdxKLineApi) reconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client != nil {
		t.client.Disconnect()
	}
	t.client = t.newClient()
	return nil
}

func (t *TdxKLineApi) ensureMACClient() error {
	t.macMu.Lock()
	defer t.macMu.Unlock()
	if t.macClient == nil {
		t.macClient = t.newMACClient()
	}
	return nil
}

func (t *TdxKLineApi) reconnectMAC() error {
	t.macMu.Lock()
	defer t.macMu.Unlock()
	if t.macClient != nil {
		t.macClient.Disconnect()
	}
	t.macClient = t.newMACClient()
	return nil
}

func (t *TdxKLineApi) ensureMACExClient() error {
	t.macExMu.Lock()
	defer t.macExMu.Unlock()
	if t.macExClient == nil {
		t.macExClient = t.newMACExClient()
	}
	return nil
}

func (t *TdxKLineApi) reconnectMACEx() error {
	t.macExMu.Lock()
	defer t.macExMu.Unlock()
	if t.macExClient != nil {
		t.macExClient.Disconnect()
	}
	t.macExClient = t.newMACExClient()
	return nil
}

// tdxRetryDelay sleeps with exponential backoff before a retry attempt.
// attempt=0 returns immediately, attempt=1 → 1s, attempt=2 → 2s, attempt=3 → 4s, capped at 8s.
func tdxRetryDelay(attempt int) {
	if attempt <= 0 {
		return
	}
	d := time.Duration(1<<uint(attempt-1)) * time.Second
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	time.Sleep(d)
}

func tdxMarketFromStockCode(stockCode string) (uint8, string) {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(code, ".") {
		parts := strings.Split(code, ".")
		if len(parts) == 2 {
			market := parts[1]
			pureCode := parts[0]
			switch market {
			case "SH", "SS":
				return uint8(types.MarketSH), pureCode
			case "SZ":
				return uint8(types.MarketSZ), pureCode
			case "BJ":
				return uint8(types.MarketBJ), pureCode
			case "HK":
				return uint8(types.MarketHK), pureCode
			case "US":
				return uint8(types.MarketUSA), pureCode
			}
		}
	}
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") || strings.HasPrefix(code, "BJ") {
		marketStr := code[:2]
		pureCode := code[2:]
		switch strings.ToUpper(marketStr) {
		case "SH":
			return uint8(types.MarketSH), pureCode
		case "SZ":
			return uint8(types.MarketSZ), pureCode
		case "BJ":
			return uint8(types.MarketBJ), pureCode
		}
	}
	// hk00700 → MarketHK, "00700"
	if strings.HasPrefix(code, "HK") {
		return uint8(types.MarketHK), code[2:]
	}
	// usAAPL → MarketUSA, "AAPL"
	if strings.HasPrefix(code, "US") {
		return uint8(types.MarketUSA), code[2:]
	}
	// gb_AAPL → MarketUSA, "AAPL"
	if strings.HasPrefix(code, "GB_") {
		return uint8(types.MarketUSA), code[3:]
	}
	if len(code) >= 1 {
		first := code[0:1]
		switch first {
		case "6", "5":
			return uint8(types.MarketSH), code
		case "0", "3", "1":
			return uint8(types.MarketSZ), code
		case "4", "8", "9":
			return uint8(types.MarketBJ), code
		}
	}
	return uint8(types.MarketSH), code
}

// TdxMarketFromStockCode 是 tdxMarketFromStockCode 的导出版本，供外部包调用
func TdxMarketFromStockCode(stockCode string) (uint8, string) {
	return tdxMarketFromStockCode(stockCode)
}

// macExMarketFromStockCode 将港美股代码转为扩展行情的 category 值和纯代码。
// 港股：主板 category=31，创业板 category=48（代码 08 开头为创业板）。
// 美股：category=74。
// A股代码返回 ok=false，应使用 tdxMarketFromStockCode + MAC 客户端。
func macExMarketFromStockCode(stockCode string) (category uint8, code string, ok bool) {
	upper := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(upper, ".") {
		parts := strings.Split(upper, ".")
		if len(parts) == 2 {
			switch parts[1] {
			case "HK":
				return hkCategoryFromCode(parts[0]), parts[0], true
			case "US":
				return uint8(types.ExCategoryUSStock), parts[0], true
			}
		}
	}
	if strings.HasPrefix(upper, "HK") {
		c := upper[2:]
		return hkCategoryFromCode(c), c, true
	}
	if strings.HasPrefix(upper, "US") {
		return uint8(types.ExCategoryUSStock), upper[2:], true
	}
	if strings.HasPrefix(upper, "GB_") {
		return uint8(types.ExCategoryUSStock), upper[3:], true
	}
	return 0, "", false
}

// hkCategoryFromCode 根据港股代码判断板块分类：08 开头为创业板(48)，其余为主板(31)
func hkCategoryFromCode(code string) uint8 {
	if len(code) >= 2 && code[:2] == "08" {
		return 48 // 香港创业板
	}
	return 31 // 香港主板
}

type TdxCallAuctionData struct {
	Time      string `json:"time"`
	Price     string `json:"price"`
	Matched   string `json:"matched"`
	Unmatched string `json:"unmatched"`
	Flag      string `json:"flag"`
}

type TdxCompanyInfoSection struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type TdxFinanceInfo struct {
	Market              uint8   `json:"market"`
	Code                string  `json:"code"`
	FloatShares         float64 `json:"floatShares"`
	TotalShares         float64 `json:"totalShares"`
	EPS                 float64 `json:"eps"`
	TotalAssets         float64 `json:"totalAssets"`
	CurrentAssets       float64 `json:"currentAssets"`
	FixedAssets         float64 `json:"fixedAssets"`
	IntangibleAssets    float64 `json:"intangibleAssets"`
	ShareholderCount    float64 `json:"shareholderCount"`
	CurrentLiabilities  float64 `json:"currentLiabilities"`
	LongTermLiabilities float64 `json:"longTermLiabilities"`
	CapitalReserve      float64 `json:"capitalReserve"`
	TotalEquity         float64 `json:"totalEquity"`
	OperatingRevenue    float64 `json:"operatingRevenue"`
	OperatingCost       float64 `json:"operatingCost"`
	AccountsReceivable  float64 `json:"accountsReceivable"`
	OperatingProfit     float64 `json:"operatingProfit"`
	InvestmentIncome    float64 `json:"investmentIncome"`
	NetCashFlow         float64 `json:"netCashFlow"`
	Inventory           float64 `json:"inventory"`
	TotalProfit         float64 `json:"totalProfit"`
	AfterTaxProfit      float64 `json:"afterTaxProfit"`
	NetProfit           float64 `json:"netProfit"`
	UndistributedProfit float64 `json:"undistributedProfit"`
	NetAssetsPerShare   float64 `json:"netAssetsPerShare"`
	IPODate             string  `json:"ipoDate"`
	UpdatedDate         string  `json:"updatedDate"`
}

type TdxXDXRItem struct {
	Date            string   `json:"date"`
	Category        uint8    `json:"category"`
	Name            string   `json:"name"`
	Fenhong         *float64 `json:"fenhong"`
	Peigujia        *float64 `json:"peigujia"`
	Songzhuangu     *float64 `json:"songzhuangu"`
	Peigu           *float64 `json:"peigu"`
	Suogu           *float64 `json:"suogu"`
	PreFloatShares  *float64 `json:"preFloatShares"`
	PreTotalShares  *float64 `json:"preTotalShares"`
	PostFloatShares *float64 `json:"postFloatShares"`
	PostTotalShares *float64 `json:"postTotalShares"`
}

type TdxCompanyInfoBundle struct {
	Sections []TdxCompanyInfoSection `json:"sections"`
	XDXR     []TdxXDXRItem           `json:"xdxr"`
	Finance  *TdxFinanceInfo         `json:"finance"`
}

type TdxCompanyCategory struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
}

func init() {
	_ = time.DateTime
}

// MACBelongBoardItem 股票所属板块信息
type MACBelongBoardItem struct {
	BoardType      string  `json:"boardType" md:"板块类型"`
	BoardCode      string  `json:"boardCode" md:"板块代码"`
	BoardName      string  `json:"boardName" md:"板块名称"`
	Price          float64 `json:"price" md:"板块价格/指数"`
	PreClose       float64 `json:"preClose" md:"板块昨收"`
	LimitUpCount   float64 `json:"limitUpCount" md:"涨停数"`
	LimitDownCount float64 `json:"limitDownCount" md:"跌停数"`
}

type TdxStockBasic struct {
	StockCode    string  // 带市场前缀的小写代码，如 sh600519 / sz000001 / bj430047
	Code         string  // 不带前缀的纯代码，如 600519
	Name         string  // 证券名称
	Market       string  // 市场标识：SH/SZ/BJ
	PreClose     float64 // 昨收价
	DecimalPoint int8    // 价格小数位数
	VolUnit      uint16  // 成交量单位
}

const (
	exCategoryHKMain = 31 // 香港主板
	exCategoryHKGem  = 48 // 香港创业板
	exCategoryUS     = 74 // 美国股票
)

// TdxExStockBasic 扩展行情返回的股票基础信息（港股/美股）
type TdxExStockBasic struct {
	Code     string // 代码：港股为 5 位数字（00001），美股为字母代码（AAPL）
	Name     string // 名称
	Category uint8  // 扩展行情分类
	Market   string // 市场标识：HK / US
}
