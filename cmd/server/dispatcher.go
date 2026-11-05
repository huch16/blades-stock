package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"blades-stock/backend/logger"
	"blades-stock/backend/services"
)

var globalDispatcher *RPCDispatcher

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

	globalDispatcher = d
	return d
}

// HasMethod 检查指定 RPC 方法是否已注册
func (d *RPCDispatcher) HasMethod(name string) bool {
	if d == nil {
		return false
	}
	_, ok := d.methods[name]
	return ok
}

// GetMethodNames 获取所有已注册的 RPC 方法名列表
func (d *RPCDispatcher) GetMethodNames() []string {
	if d == nil {
		return nil
	}
	names := make([]string, 0, len(d.methods))
	for name := range d.methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// goTypeToOpenAPISchema 将 Go reflect.Type 反射映射为标准 OpenAPI 3.0 数据结构 Schema
func goTypeToOpenAPISchema(t reflect.Type, depth int) map[string]any {
	if t == nil || depth > 3 {
		return map[string]any{"type": "object"}
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number", "format": "double"}
	case reflect.Slice, reflect.Array:
		elemSchema := goTypeToOpenAPISchema(t.Elem(), depth+1)
		return map[string]any{
			"type":  "array",
			"items": elemSchema,
		}
	case reflect.Map:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		}
	case reflect.Struct:
		fields := make(map[string]any)
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" { // 未导出的内部私有字段跳过
				continue
			}
			jsonTag := f.Tag.Get("json")
			fieldName := f.Name
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] == "-" {
					continue
				}
				if parts[0] != "" {
					fieldName = parts[0]
				}
			}
			if f.Type == t || (f.Type.Kind() == reflect.Ptr && f.Type.Elem() == t) {
				fields[fieldName] = map[string]any{"type": "object"}
			} else {
				fields[fieldName] = goTypeToOpenAPISchema(f.Type, depth+1)
			}
		}
		schema := map[string]any{
			"type": "object",
		}
		if t.Name() != "" {
			schema["title"] = t.Name()
		}
		if len(fields) > 0 {
			schema["properties"] = fields
		}
		return schema
	default:
		return map[string]any{"type": "object", "description": t.String()}
	}
}

// GetOpenAPISpec 动态生成机器可读的 OpenAPI 3.0 契约文档规范 (P0-2, P1-7 深度重构补齐 Schema 与鉴权)
func (d *RPCDispatcher) GetOpenAPISpec() map[string]any {
	paths := make(map[string]any)
	errType := reflect.TypeOf((*error)(nil)).Elem()

	for name, entry := range d.methods {
		m := entry.method
		numIn := m.Type.NumIn()

		var argDescriptions []string
		var argItems []map[string]any
		for j := 1; j < numIn; j++ {
			pType := m.Type.In(j)
			pSchema := goTypeToOpenAPISchema(pType, 0)
			typeName := pType.String()
			pSchema["description"] = fmt.Sprintf("arg%d (%s)", j-1, typeName)
			argItems = append(argItems, pSchema)
			argDescriptions = append(argDescriptions, fmt.Sprintf("arg%d: %s", j-1, typeName))
		}

		reqDesc := "RPC positional arguments [arg0, arg1, ...]"
		if len(argDescriptions) > 0 {
			reqDesc = fmt.Sprintf("RPC positional arguments: [%s]", strings.Join(argDescriptions, ", "))
		}

		reqSchema := map[string]any{
			"type":        "array",
			"description": reqDesc,
		}
		if len(argItems) == 1 {
			reqSchema["items"] = argItems[0]
		} else if len(argItems) > 1 {
			reqSchema["items"] = map[string]any{
				"oneOf": argItems,
			}
		} else {
			reqSchema["items"] = map[string]any{}
		}

		// 解析方法返回值 Schema
		dataSchema := map[string]any{"description": "Method return payload"}
		numOut := m.Type.NumOut()
		if numOut > 0 {
			outType := m.Type.Out(0)
			if outType.Implements(errType) {
				dataSchema = map[string]any{"type": "object", "description": "Status or error details"}
			} else {
				dataSchema = goTypeToOpenAPISchema(outType, 0)
			}
		}

		paths["/api/App/"+name] = map[string]any{
			"post": map[string]any{
				"summary":     "RPC method " + name,
				"operationId": name,
				"tags":        []string{"AppRPC"},
				"security": []map[string]any{
					{"bearerAuth": []any{}},
					{"cookieAuth": []any{}},
				},
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": reqSchema,
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Successful RPC response",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"code":    map[string]any{"type": "integer", "example": 0},
										"data":    dataSchema,
										"message": map[string]any{"type": "string", "example": "success"},
									},
									"required": []string{"code"},
								},
							},
						},
					},
					"401": map[string]any{
						"description": "未授权访问",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"code":  map[string]any{"type": "integer", "example": 401},
										"error": map[string]any{"type": "string", "example": "未授权访问：请提供正确的访问令牌"},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	return map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":       "Blades Stock RPC API",
			"version":     "3.0.0",
			"description": "Deterministic Multi-Agent Quantitative Research & Trading RPC Specification (250 Endpoints with strict schemas)",
		},
		"security": []map[string]any{
			{"bearerAuth": []any{}},
			{"cookieAuth": []any{}},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "Token",
					"description":  "Blades Stock 统一访问凭据口令 (Authorization: Bearer <token>)",
				},
				"cookieAuth": map[string]any{
					"type":        "apiKey",
					"in":          "cookie",
					"name":        "blades_token",
					"description": "安全 HttpOnly Cookie 会话凭据",
				},
			},
			"schemas": map[string]any{
				"StandardResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code":    map[string]any{"type": "integer", "example": 0},
						"message": map[string]any{"type": "string"},
						"data":    map[string]any{"description": "Method return payload"},
					},
					"required": []string{"code"},
				},
				"ErrorResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code":  map[string]any{"type": "integer", "example": 401},
						"error": map[string]any{"type": "string"},
					},
					"required": []string{"code", "error"},
				},
			},
		},
		"paths": paths,
	}
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

	// 工业级容灾边界：全捕获 RPC 调用内部可能的 panic，杜绝崩溃并回吐标准化错误响应（脱敏内部堆栈与类型细节）
	defer func() {
		if rec := recover(); rec != nil {
			logger.SugaredLogger.Errorf("[RPCDispatcher] Panic in RPC call %s: %v", r.URL.Path, rec)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    500,
				"error":   "Internal RPC server error",
				"message": "服务内部处理异常，已触发安全降级保护",
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
				// 容错增强：若目标是 string 类型，但前端传入了非 string 的 JSON（例如空对象 {}、布尔值或数值），优雅降级为空字符串，杜绝 400 崩溃
				if paramType.Kind() == reflect.String {
					logger.SugaredLogger.Warnf("[RPCDispatcher] Param %d for %s expected string, got %s, defaulting to empty string", argIdx, path, string(rawArgs[argIdx]))
					paramVal.Elem().SetString("")
					callArgs[i] = paramVal.Elem()
					argIdx++
					continue
				}
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
