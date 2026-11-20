package datasource

import (
	"fmt"
	"strings"

	uaFake "github.com/lib4u/fake-useragent"
)

// KLineData 是 K 线行 DTO（R-25a 自 backend/data/stock_data_api.go 移入；
// 纯传输结构，无 TableName、无入库；data 侧留 type alias）。
type KLineData struct {
	Day           string            `json:"day" md:"时间/日期"`
	Open          string            `json:"open" md:"开盘价"`
	Close         string            `json:"close" md:"收盘价"`
	High          string            `json:"high" md:"最高价"`
	Low           string            `json:"low" md:"最低价"`
	Volume        string            `json:"volume" md:"成交量"`
	Amount        string            `json:"amount" md:"成交额"`
	ChangePercent string            `json:"changePercent" md:"涨跌幅"`
	ChangeValue   string            `json:"changeValue" md:"涨跌额"`
	Amplitude     string            `json:"amplitude" md:"振幅"`
	TurnoverRate  string            `json:"turnoverRate" md:"换手率"`
	VolumeRatio   string            `json:"volumeRatio" md:"量比"`
	MA            map[string]string `json:"ma,omitempty" md:"均线"` // 周期 -> 均线值，如 "5":"12.34"，由 GetKLineWithMA 填充
}

// GetRandomUA 随机返回一个 User-Agent（使用 fake-useragent 库）。
// R-25a 自 backend/data/eastmoney_kline_api.go 移入；data 侧留同名小写转发。
func GetRandomUA() string {
	ua, _ := uaFake.New()
	if ua != nil {
		randomUA := ua.Filter().Platform("desktop").Get()
		//logger.SugaredLogger.Infof("User-Agent: %s", randomUA)
		return randomUA
	}
	// 如果库获取失败，返回备用 UA
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
}

// ParseFloatToFloat 将 K 线价格字符串转为 float64。
// R-25a 自 backend/data/eastmoney_kline_api.go 移入；data 侧留同名小写转发。
// parseFloatToFloat 将 K 线价格字符串转为 float64
func ParseFloatToFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "null" {
		return 0, nil
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// AggregateKLineEveryN 将连续 n 根 1 分钟 K 线合并为一根
// （用于东方财富无原生 10 分钟周期时；假定 src 已按时间正序排列）。
// R-25b 自 backend/data/eastmoney_kline_api.go 移入；data 侧留同名转发。
// AggregateKLineEveryN 将连续 n 根 1 分钟 K 线合并为一根（用于东方财富无原生 10 分钟周期时）。
// 假定 src 已按时间正序排列。
func AggregateKLineEveryN(src *[]KLineData, n int) *[]KLineData {
	if src == nil || n < 2 {
		return src
	}
	arr := *src
	if len(arr) == 0 {
		return src
	}
	out := make([]KLineData, 0, len(arr)/n+1)
	for i := 0; i < len(arr); {
		end := i + n
		if end > len(arr) {
			end = len(arr)
		}
		chunk := arr[i:end]
		first := chunk[0]
		last := chunk[len(chunk)-1]
		highF := -1e18
		lowF := 1e18
		volSum := 0.0
		amtSum := 0.0
		for _, c := range chunk {
			h, _ := ParseFloatToFloat(c.High)
			l, _ := ParseFloatToFloat(c.Low)
			v, _ := ParseFloatToFloat(c.Volume)
			a, _ := ParseFloatToFloat(c.Amount)
			if h > highF {
				highF = h
			}
			if l < lowF {
				lowF = l
			}
			volSum += v
			amtSum += a
		}
		openS := first.Open
		closeS := last.Close
		highS := fmt.Sprintf("%.4f", highF)
		lowS := fmt.Sprintf("%.4f", lowF)
		if highF <= -1e17 {
			highS = first.High
		}
		if lowF >= 1e17 {
			lowS = first.Low
		}
		out = append(out, KLineData{
			Day:           last.Day,
			Open:          openS,
			Close:         closeS,
			High:          highS,
			Low:           lowS,
			Volume:        fmt.Sprintf("%.0f", volSum),
			Amount:        fmt.Sprintf("%.2f", amtSum),
			ChangePercent: last.ChangePercent,
			ChangeValue:   last.ChangeValue,
			TurnoverRate:  last.TurnoverRate,
			Amplitude:     last.Amplitude,
		})
		i = end
		if len(chunk) < n {
			break
		}
	}
	return &out
}
