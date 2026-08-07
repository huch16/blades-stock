//go:build probe

package data

import (
	"fmt"
	"strings"
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// 手动运行：go test -tags probe ./backend/data/ -run TestRerunReportVerify -count=1 -v
// 作用：对已入库的三份六张网报告（id=37/39/41）重跑 verifyAndCorrectContent
//       （价格/市值/代码存在性硬校验），并在内容头部追加人工核验结论备注，写回 DB。
// 注意：自动化校验无法发现"名称错配"（英脉科技≠华融化学）与"已退市"（外运发展）
//       两类业务硬伤，由人工备注补充记录。
func TestRerunReportVerify(t *testing.T) {
	db.Init("../../data/stock.db")
	type job struct {
		id       int
		question string
		note     string
	}
	jobs := []job{
		{37, "国家推行“六张网”计划，哪些公司最受益？", "【核验备注 2026-08-04】v1 版存在标的编造：英脉科技 301256 实为华融化学、联塑科技 002131 实为利欧股份、金牛管业 002727 实为一心堂；海光信息/工业富联市值表述与自身「选股纪律(剔除>500亿)」矛盾。以下内容为自动化价格/市值校正结果，业务硬伤未自动修复，引用需谨慎。"},
		{39, "国家推行“六张网”计划，哪些公司最受益？", "【核验备注 2026-08-04】v2 版标的价格/市值大量编造（如正元地信实为 4.20 元/32 亿 vs 文中 45-50 元/~110 亿），以下内容已逐项校正并标注 [已校正]。原市值/价格表述不可信。"},
		{41, "推荐3个ETF基金。", "【核验备注 2026-08-04】本份实为「六张网」报告（入库时 question 记录错位）。24 只 A 股代码-名称 100% 真实；两处业务硬伤：华立股份 603038 主营家居用品/装修装饰而非智能电表（真智能电表龙头为林洋能源 601222 等）；外运发展 600270 已于 2018 年被中国外运换股吸收合并并退市。以下为自动化校正结果，上述硬伤未自动修复。"},
	}
	for _, j := range jobs {
		var rec models.AIResponseResult
		if err := db.Dao.First(&rec, j.id).Error; err != nil {
			t.Fatalf("load id=%d: %v", j.id, err)
		}
		corrected := verifyAndCorrectContent(rec.Content, rec.StockCode, j.question)
		newContent := j.note + "\n\n" + corrected
		if err := db.Dao.Model(&rec).Update("content", newContent).Error; err != nil {
			t.Fatalf("update id=%d: %v", j.id, err)
		}
		fmt.Printf("id=%d corrected, len %d -> %d, has已校正=%v has代码不存在=%v\n",
			j.id, len(rec.Content), len(newContent),
			strings.Contains(newContent, "已校正"), strings.Contains(newContent, "代码不存在"))
	}
}
