//go:build probe

package data

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// 手动运行：go test -tags probe ./backend/data/ -run TestProbeVerifyLive -count=1 -v
// 需要临时文件存在：C:\Users\huch\AppData\Local\Temp\opencode\id39_content.txt
// 作用：用真实报告内容（含编造市值/价格）跑一遍 verifyAndCorrectContent 网络校验，
//
//	输出写到 C:\Users\huch\AppData\Local\Temp\opencode\probe_out.txt。
func TestProbeVerifyLive(t *testing.T) {
	path := `C:\Users\huch\AppData\Local\Temp\opencode\id39_content.txt`
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("probe file missing: %v", err)
	}
	start := time.Now()
	out := verifyAndCorrectContent(string(content), "agent", "")
	fmt.Printf("=== elapsed %s ===\n", time.Since(start))
	os.WriteFile(`C:\Users\huch\AppData\Local\Temp\opencode\probe_out.txt`, []byte(out), 0644)
	fmt.Println("written probe_out.txt len=", len(out))
}
