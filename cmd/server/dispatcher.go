package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"blades-stock/backend/logger"
	"blades-stock/backend/services"
)

// methodEntry 描述一个已注册的 RPC 方法及其所属的目标接收者
type methodEntry struct {
	receiver reflect.Value
	method   reflect.Method
}

// RPCDispatcher 负责反射分发来自前端的 RPC 请求
type RPCDispatcher struct {
	methods map[string]methodEntry
}

// NewRPCDispatcher 创建分发器并注册所有业务微服务
func NewRPCDispatcher(
	market *services.MarketService,
	quant *services.QuantService,
	agent *services.AgentService,
	settings *services.SettingsService,
) *RPCDispatcher {
	d := &RPCDispatcher{
		methods: make(map[string]methodEntry),
	}

	// 注册四个核心服务上的所有导出方法
	d.registerTarget(market)
	d.registerTarget(quant)
	d.registerTarget(agent)
	d.registerTarget(settings)

	return d
}

// registerTarget 遍历结构体上的所有 Public 方法并注册到路由表
func (d *RPCDispatcher) registerTarget(target any) {
	val := reflect.ValueOf(target)
	typ := val.Type()

	for i := 0; i < val.NumMethod(); i++ {
		m := typ.Method(i)
		// 仅注册大写开头的导出方法
		if m.PkgPath == "" {
			d.methods[m.Name] = methodEntry{
				receiver: val,
				method:   m,
			}
		}
	}
}

// HandleRPC 处理 POST /api/App/{method} 请求
func (d *RPCDispatcher) HandleRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 工业级容灾边界：全捕获 RPC 调用内部可能的 panic，杜绝崩溃并回吐标准化错误响应
	defer func() {
		if rec := recover(); rec != nil {
			logger.SugaredLogger.Errorf("[RPCDispatcher] Panic in RPC call %s: %v", r.URL.Path, rec)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":  500,
				"error": fmt.Sprintf("Internal RPC server panic: %v", rec),
			})
		}
	}()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]any{"code": 405, "error": "Method Not Allowed, use POST"})
		return
	}

	// 从 URL 路径中提取方法名，例如 /api/App/GetStockList -> GetStockList
	path := strings.TrimPrefix(r.URL.Path, "/api/App/")
	path = strings.Trim(path, "/")

	entry, ok := d.methods[path]
	if !ok {
		logger.SugaredLogger.Warnf("[RPCDispatcher] Method not found: %s", path)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"code":  404,
			"error": fmt.Sprintf("RPC method '%s' not found", path),
		})
		return
	}

	// 解析请求体中的参数数组 [arg0, arg1, ...]
	var rawArgs []json.RawMessage
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&rawArgs)
	}

	// 准备调用入参（第一个是 receiver）
	mType := entry.method.Type
	numIn := mType.NumIn()
	callArgs := make([]reflect.Value, numIn)
	callArgs[0] = entry.receiver

	argIdx := 0
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()

	for i := 1; i < numIn; i++ {
		paramType := mType.In(i)

		// 如果入参是 context.Context，自动注入当前 HTTP 请求的上下文
		if paramType.Implements(ctxType) {
			callArgs[i] = reflect.ValueOf(r.Context())
			continue
		}

		// 如果传入的参数足够，进行 JSON 反序列化
		if argIdx < len(rawArgs) {
			paramVal := reflect.New(paramType)
			if err := json.Unmarshal(rawArgs[argIdx], paramVal.Interface()); err != nil {
				logger.SugaredLogger.Errorf("[RPCDispatcher] Param %d unmarshal error for %s: %v", argIdx, path, err)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{
					"code":  400,
					"error": fmt.Sprintf("Invalid argument #%d for method %s: %v", argIdx, path, err),
				})
				return
			}
			callArgs[i] = paramVal.Elem()
			argIdx++
		} else {
			// 参数不足时填充零值
			callArgs[i] = reflect.Zero(paramType)
		}
	}

	// 执行方法调用
	results := entry.method.Func.Call(callArgs)

	// 处理返回值
	d.handleResults(w, results)
}

func (d *RPCDispatcher) handleResults(w http.ResponseWriter, results []reflect.Value) {
	if len(results) == 0 {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": nil,
		})
		return
	}

	// 检查最后一个返回值是否是 error 接口
	last := results[len(results)-1]
	errType := reflect.TypeOf((*error)(nil)).Elem()

	if last.Type().Implements(errType) {
		if !last.IsNil() {
			err := last.Interface().(error)
			json.NewEncoder(w).Encode(map[string]any{
				"code":  500,
				"error": err.Error(),
			})
			return
		}
		// 如果只有 1 个返回值且为 nil error
		if len(results) == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": nil,
			})
			return
		}
		// 返回前一个数据结果
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": results[0].Interface(),
		})
		return
	}

	// 只有一个普通数据返回值
	json.NewEncoder(w).Encode(map[string]any{
		"code": 0,
		"data": results[0].Interface(),
	})
}
