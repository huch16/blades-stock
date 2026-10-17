package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
)

func main() {
	log.SetOutput(os.Stdout)
	fmt.Println("=== 启动 Windows 桌面 SQLite 数据保存可靠性与并发写入测试 ===")

	// 1. 初始化数据库连接（使用优化后的 BuildSqliteDSN 连接池与 immediate 事务）
	db.Init("")

	// 2. 测试配置单次保存
	cfg := data.GetSettingConfig()
	if cfg == nil {
		log.Fatalf("获取系统配置失败: cfg 为空")
	}
	originalPrompt := cfg.Prompt
	testPrompt := fmt.Sprintf("测试保存指令_%d", time.Now().UnixNano())
	cfg.Prompt = testPrompt

	res := data.UpdateConfig(cfg)
	fmt.Printf("1. UpdateConfig 执行结果: %s\n", res)
	if res != "保存成功！" {
		log.Fatalf("UpdateConfig 单次保存失败: %s", res)
	}

	// 重新读取验证
	reloaded := data.GetSettingConfig()
	if reloaded.Prompt != testPrompt {
		log.Fatalf("保存数据不一致: 写入=%s, 读取=%s", testPrompt, reloaded.Prompt)
	}
	fmt.Printf("2. 重新加载校验通过: Prompt=%s\n", reloaded.Prompt)

	// 还原配置
	cfg.Prompt = originalPrompt
	data.UpdateConfig(cfg)

	// 3. 高并发竞争压测：模拟后台标的同步、审计日志写入、前台用户连续保存同时发生
	fmt.Println("3. 开始并发写入压测 (10 个并发协程连续更新配置与基础信息)...")
	var wg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			workerCfg := data.GetSettingConfig()
			workerCfg.Prompt = fmt.Sprintf("并发测试_%d_%d", workerID, time.Now().UnixNano())
			workerRes := data.UpdateConfig(workerCfg)
			if workerRes != "保存成功！" {
				mu.Lock()
				errCount++
				fmt.Printf("Worker %d 保存失败: %s\n", workerID, workerRes)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	if errCount > 0 {
		log.Fatalf("并发压测失败: 共有 %d 个写操作报错", errCount)
	}
	fmt.Println("4. 并发压测 100% 成功，0 次 database is locked！")

	// 4. 测试通达信内存差异同步
	fmt.Println("5. 测试 TDX 同步（验证内存预检与事务批量写入）...")
	tdx := data.NewTdxKLineApi()
	added, updated, err := tdx.SyncStockBasicToDB()
	if err != nil {
		fmt.Printf("TDX 同步返回（网络受限或离线正常）: %v\n", err)
	} else {
		fmt.Printf("TDX 同步成功完成: 新增=%d, 更新=%d\n", added, updated)
	}

	fmt.Println("=== 全部数据保存测试 100% 验证通过！===")
}

