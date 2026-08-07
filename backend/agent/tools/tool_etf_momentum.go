package tools

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

var etfMomentumPool = []struct {
	Code string
	Name string
}{
	{"sh510300", "沪深300ETF"}, {"sh510500", "中证500ETF"}, {"sh512100", "中证1000ETF"},
	{"sh510050", "上证50ETF"}, {"sz159915", "创业板"}, {"sz159919", "沪深300"},
	{"sh563360", "A500ETF"}, {"sh510880", "红利ETF"}, {"sh512890", "红利低波ETF"},
	{"sz159920", "恒生ETF"}, {"sz159605", "互联中概"}, {"sh513100", "纳指ETF"},
	{"sh588000", "科创50ETF"}, {"sh515000", "科技ETF"}, {"sz159995", "芯片ETF"},
	{"sz159516", "半导设备"}, {"sz159852", "软件ETF"}, {"sz159998", "计算机"},
	{"sz159527", "云50"}, {"sz159869", "游戏ETF"}, {"sh515880", "通信ETF"},
	{"sh512880", "证券ETF"}, {"sh512000", "券商ETF"}, {"sh515030", "新能源车ETF"},
	{"sh516160", "新能源ETF"}, {"sh562500", "机器人ETF"}, {"sz159725", "线上消费"},
	{"sz159865", "养殖ETF"}, {"sz159992", "创新药"}, {"sz159938", "医药"},
}

type etfMomentumRow struct {
	Code      string
	Name      string
	LastDay   string
	LastClose float64
	ChgPct    float64
	Base21    float64
	WMom21    float64
	Mom21     float64
}

func computeETFMomentumRows() []etfMomentumRow {
	var (
		rows []etfMomentumRow
		mu   sync.Mutex
		wg   sync.WaitGroup
	)
	sem := make(chan struct{}, 8)
	for _, cand := range etfMomentumPool {
		wg.Add(1)
		sem <- struct{}{}
		go func(cand struct {
			Code string
			Name string
		}) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("panic in ETF momentum worker [%s]: %v", cand.Code, r)
				}
			}()
			// 取 90 根，使模型后续对同一代码的 GetStockKLine(count<=90) 能命中 K 线缓存
			res := data.FetchKLineWithFallback(cand.Code, cand.Name, "101", 90, "")
			if res == nil || res.Data == nil || len(*res.Data) < 22 {
				return
			}
			d := *res.Data
			last := d[len(d)-1]
			close0, _ := strconv.ParseFloat(last.Close, 64)
			prevClose, _ := strconv.ParseFloat(d[len(d)-2].Close, 64)
			chg := 0.0
			if prevClose > 0 {
				chg = (close0 - prevClose) / prevClose * 100
			}
			weighted := 0.0
			wsum := 0.0
			for i := 0; i <= 20; i++ {
				idx := len(d) - 1 - i
				c, _ := strconv.ParseFloat(d[idx].Close, 64)
				w := math.Pow(0.5, float64(i)/7.0)
				weighted += c * w
				wsum += w
			}
			base, _ := strconv.ParseFloat(d[len(d)-22].Close, 64)
			row := etfMomentumRow{Code: cand.Code, Name: cand.Name, LastDay: last.Day, LastClose: close0, ChgPct: chg, Base21: base}
			if base > 0 {
				row.WMom21 = (weighted/wsum - base) / base * 100
				row.Mom21 = (close0 - base) / base * 100
			}
			mu.Lock()
			rows = append(rows, row)
			mu.Unlock()
		}(cand)
	}
	wg.Wait()
	sort.Slice(rows, func(i, j int) bool { return rows[i].WMom21 > rows[j].WMom21 })
	return rows
}

func GetETFMomentumRankingTool() bladesTools.Tool {
	return types.NewTool("GetETFMomentumRanking",
		"获取30只主流场内ETF的21日加权动量排名表（半衰期7日加权，按动量降序），返回：排名/代码/名称/最新收盘/涨跌幅%/基准价(21交易日前收盘)/21日加权动量%/21日动量%。ETF动量轮动分析必须优先调用本工具获取权威动量数据，禁止自行估算或编造动量数值；候选池纪律（动量>0的标的须全量纳入、排除须给理由）详见系统提示词【ETF动量轮动纪律】。",
		map[string]*types.ParamDef{
			"count": {Type: "integer", Desc: "返回前N只，默认20，最大30", Required: false},
		},
		func(args string) (string, error) {
			logger.SugaredLogger.Infof("GetETFMomentumRanking called with args: %s", args)
			count := 20
			if c := gjson.Get(args, "count").Int(); c > 0 {
				count = int(c)
			}
			if count > 30 {
				count = 30
			}
			rows := computeETFMomentumRows()
			if len(rows) == 0 {
				return "数据不足：所有候选ETF的K线均获取失败，无法计算动量排名。", nil
			}
			if count > len(rows) {
				count = len(rows)
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("ETF 21日加权动量排名 Top%d（数据截至K线最新交易日）:\n\n", count))
			b.WriteString("|排名|代码|名称|最新收盘|涨跌幅%|基准价(21日前)|21日加权动量%|21日动量%|\n")
			b.WriteString("|---:|---|---:|---:|---:|---:|---:|---:|\n")
			for i, r := range rows[:count] {
				b.WriteString(fmt.Sprintf("|%d|%s|%s|%.4f|%.2f|%.4f|%.2f|%.2f|\n",
					i+1, r.Code, r.Name, r.LastClose, r.ChgPct, r.Base21, r.WMom21, r.Mom21))
			}
			return b.String(), nil
		},
	)
}
