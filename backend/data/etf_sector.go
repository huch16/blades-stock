package data

import (
	"encoding/json"
	"os"
	"sync"
)

var (
	etfSectorMap  map[string]string
	etfSectorOnce sync.Once
	etfSectorErr  error
)

// LoadETFSectorMap 读取并缓存 ETF -> 板块 映射表
// 文件路径：backend/data/etf_sector_map.json
func LoadETFSectorMap() (map[string]string, error) {
	etfSectorOnce.Do(func() {
		path := "backend/data/etf_sector_map.json"
		data, err := os.ReadFile(path)
		if err != nil {
			etfSectorErr = err
			return
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			etfSectorErr = err
			return
		}
		etfSectorMap = m
	})
	return etfSectorMap, etfSectorErr
}

// GetETFSector 单个 ETF 代码查板块，找不到返回 "默认"
func GetETFSector(code string) string {
	m, _ := LoadETFSectorMap()
	if sec, ok := m[code]; ok {
		return sec
	}
	return "默认"
}

// GetETFSectors 批量查询，返回 code->sector map（缺省填 "默认"）
func GetETFSectors(codes []string) map[string]string {
	m, _ := LoadETFSectorMap()
	res := make(map[string]string, len(codes))
	for _, c := range codes {
		if sec, ok := m[c]; ok {
			res[c] = sec
		} else {
			res[c] = "默认"
		}
	}
	return res
}
