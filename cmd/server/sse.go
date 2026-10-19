package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"blades-stock/backend/logger"
)

// SSEHub 负责管理所有活跃的 Server-Sent Events 长连接客户端
type SSEHub struct {
	clientsMu sync.RWMutex
	clients   map[chan []byte]bool
}

var globalSSEHub = NewSSEHub()

// NewSSEHub 创建 SSE 连接管理器
func NewSSEHub() *SSEHub {
	hub := &SSEHub{
		clients: make(map[chan []byte]bool),
	}
	// 启动定期心跳协程，防止代理（如 Nginx / Cloudflare）因空闲切断长连接
	go hub.startHeartbeat()
	return hub
}

func (h *SSEHub) addClient(ch chan []byte) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[ch] = true
	logger.SugaredLogger.Infof("[SSE] Client connected, total active clients: %d", len(h.clients))
}

func (h *SSEHub) removeClient(ch chan []byte) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
		logger.SugaredLogger.Infof("[SSE] Client disconnected, remaining: %d", len(h.clients))
	}
}

// Broadcast 向所有在线客户端广播结构化事件。
// 根据 W3C EventSource 规范，若包含自定义 event: 名称，浏览器的 onmessage 回调将不会被触发。
// 故采用标准 data: {"event": "...", "data": ...}\n\n 格式，确保客户端 onmessage 100% 稳定接收并派发。
func (h *SSEHub) Broadcast(event string, data any) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	if len(h.clients) == 0 {
		return
	}

	payload := map[string]any{
		"event": event,
		"data":  data,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	msg := []byte(fmt.Sprintf("data: %s\n\n", string(bytes)))

	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// 关键事件（如智能体消息、状态变更、结束信号等）进行缓冲投递，绝不允许随意丢弃
			go func(c chan []byte, m []byte) {
				select {
				case c <- m:
				case <-time.After(1 * time.Second):
					// 超时 1s 后放弃，防止 goroutine 泄漏
				}
			}(ch, msg)
		}
	}
}

// startHeartbeat 每 15 秒向客户端发送一次保活注释行
func (h *SSEHub) startHeartbeat() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.clientsMu.RLock()
		if len(h.clients) > 0 {
			ping := []byte(": ping\n\n")
			for ch := range h.clients {
				select {
				case ch <- ping:
				default:
				}
			}
		}
		h.clientsMu.RUnlock()
	}
}

// HandleSSE 处理客户端 /api/events/stream 请求
func HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 禁用此长连接上的任何写超时限制（Go 1.20+ ResponseController）
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientChan := make(chan []byte, 1024)
	globalSSEHub.addClient(clientChan)
	defer globalSSEHub.removeClient(clientChan)

	// 握手确认
	if _, err := w.Write([]byte("data: {\"event\":\"ready\",\"data\":{\"status\":\"connected\"}}\n\n")); err != nil {
		return
	}
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			if _, err := w.Write(msg); err != nil {
				// 客户端断开连接或网络写入失败，立即退出释放连接
				return
			}
			flusher.Flush()
		}
	}
}
