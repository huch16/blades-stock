package main

// @Author spark
// @Date 2025/7/8 18:51
// @Desc
//-----------------------------------------------------------------------------------

import "runtime"

// IsMacOS 判断是否为 macOS 系统
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}
