package data

import (
	"blades-stock/backend/logger"
	"os"
	"path/filepath"
	"testing"

	"github.com/duke-git/lancet/v2/slice"
)

// TestRemoveNonPrintable tests the RemoveAllBlankChar function.
func TestRemoveNonPrintable(t *testing.T) {
	//tests := []struct {
	//	input    string
	//	expected string
	//}{
	//	{"新 希 望", "新希望"},
	//	{"", ""},
	//	{"Hello, World!", "Hello, World!"},
	//	{"\x00\x01\x02", ""},
	//	{"Hello\x00World", "HelloWorld"},
	//	{"\x1F\x20\x7E\x7F", " \x7E"},
	//}

	//for _, test := range tests {
	//	actual := RemoveAllBlankChar(test.input)
	//	if actual != test.expected {
	//		t.Errorf("RemoveAllBlankChar(%q) = %q; expected %q", test.input, actual, test.expected)
	//	}
	//}
	txt := "新 希 望"
	txt2 := RemoveAllBlankChar(txt)
	logger.SugaredLogger.Infof("RemoveAllBlankChar(%s)", txt2)
	logger.SugaredLogger.Infof("RemoveAllBlankChar(%s)", txt)

}

func TestConvertStockCodeToTushareCode(t *testing.T) {
	logger.SugaredLogger.Infof("ConvertStockCodeToTushareCode(%s)", ConvertStockCodeToTushareCode("sz000802"))
	logger.SugaredLogger.Infof("ConvertTushareCodeToStockCode(%s)", ConvertTushareCodeToStockCode("000802.SZ"))
}

func TestConvertTushareCodeToStockCodes(t *testing.T) {
	logger.SugaredLogger.Infof("ConvertTushareCodeToStockCodes(%s)", ConvertTushareCodeToStockCodes([]string{"000802.SZ", "000803.SZ"}))
}
func TestReplaceSensitiveWords(t *testing.T) {
	txt := "新 希 望习近平"
	txt2 := ReplaceSensitiveWords(txt)
	logger.SugaredLogger.Infof("ReplaceSensitiveWords(%s)", txt2)

	// v3.0.0 固化：此处原为 os.WriteFile("words.txt", ...)，会把 664 KB 的测试产物
	// 直接写进包目录，并被误提交进仓库（.gitignore 对已追踪文件无效）。
	// 现改写进 t.TempDir()，测试结束后自动清理，且永不污染工作区。
	tmpFile := filepath.Join(t.TempDir(), "words.txt")
	if err := os.WriteFile(tmpFile, []byte(slice.Join(SensitiveWords, "\n")), 0644); err != nil {
		t.Fatalf("写入临时词表失败: %v", err)
	}

	// 顺带固化词表加载本身：embed 数据必须非空且完成切分
	if len(SensitiveWords) == 0 {
		t.Fatal("SensitiveWords 为空，go:embed 词表加载失败")
	}
	t.Logf("敏感词表加载正常，共 %d 条", len(SensitiveWords))
}
