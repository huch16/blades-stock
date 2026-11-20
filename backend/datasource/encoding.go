package datasource

import (
	"bytes"
	"io"

	"blades-stock/backend/logger"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// GB18030ToUTF8 将 GB18030 编码字节转换为 UTF8 字符串。
// R-25a 自 backend/data/stock_realtime.go 移入；data 侧留同名转发。
// GB18030ToUTF8 GB18030 转换为 UTF8
func GB18030ToUTF8(bs []byte) string {
	reader := transform.NewReader(bytes.NewReader(bs), simplifiedchinese.GB18030.NewDecoder())
	d, err := io.ReadAll(reader)
	if err != nil {
		logger.SugaredLogger.Warnf("GB18030ToUTF8 decode error: %v, returning raw string", err)
		return string(bs)
	}
	return string(d)
}
