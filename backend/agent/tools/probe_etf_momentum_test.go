//go:build probe

package tools

import (
	"fmt"
	"testing"
)

// 手动运行：go test -tags probe ./backend/agent/tools/ -run TestProbeETFMomentum -count=1 -v
func TestProbeETFMomentum(t *testing.T) {
	rows := computeETFMomentumRows()
	fmt.Printf("=== total rows: %d ===\n", len(rows))
	fmt.Printf("|排名|代码|名称|最新收盘|涨跌幅%%|基准价(21日前)|21日加权动量%%|21日动量%%|\n")
	fmt.Printf("|---:|---|---:|---:|---:|---:|---:|---:|\n")
	positive := 0
	for i, r := range rows {
		mark := ""
		if r.WMom21 <= 0 {
			mark = "  <-- 动量<=0"
		} else {
			positive++
		}
		fmt.Printf("|%d|%s|%s|%.4f|%.2f|%.4f|%.2f|%.2f|%s\n",
			i+1, r.Code, r.Name, r.LastClose, r.ChgPct, r.Base21, r.WMom21, r.Mom21, mark)
	}
	fmt.Printf("\n=== 加权动量>0 的标的数: %d ===\n", positive)
}
