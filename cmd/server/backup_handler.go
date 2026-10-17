package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"
)

// HandleBackupDownload 针对 Web SaaS 模式的直接安全下载备份文件流
func HandleBackupDownload(w http.ResponseWriter, r *http.Request) {
	// 验证公网 Token 鉴权 (支持 Header 和 Query ?token=)
	serverToken := getAuthToken()
	if serverToken != "" {
		clientToken := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			clientToken = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}
		if clientToken == "" {
			clientToken = strings.TrimSpace(r.URL.Query().Get("token"))
		}
		if subtle.ConstantTimeCompare([]byte(clientToken), []byte(serverToken)) != 1 {
			http.Error(w, "未授权下载", http.StatusUnauthorized)
			return
		}
	}

	rawFile := strings.TrimSpace(r.URL.Query().Get("file"))
	if rawFile == "" {
		http.Error(w, "缺少 file 参数", http.StatusBadRequest)
		return
	}

	// 严防目录穿越攻击
	cleanName := filepath.Base(rawFile)
	backupDir := data.EnsureBackupDir()
	filePath := filepath.Join(backupDir, cleanName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "备份文件不存在: "+cleanName, http.StatusNotFound)
		return
	}

	contentType := "application/octet-stream"
	switch {
	case strings.HasSuffix(cleanName, ".json"):
		contentType = "application/json; charset=utf-8"
	case strings.HasSuffix(cleanName, ".csv"):
		contentType = "text/csv; charset=utf-8"
	case strings.HasSuffix(cleanName, ".db"):
		contentType = "application/x-sqlite3"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", cleanName))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, filePath)
}

// HandleBackupUpload 针对 Web SaaS 模式的本地资产包上传并执行安全恢复
func HandleBackupUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 验证权限
	serverToken := getAuthToken()
	if serverToken != "" {
		clientToken := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			clientToken = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}
		if subtle.ConstantTimeCompare([]byte(clientToken), []byte(serverToken)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"code": 401, "error": "未授权操作"})
			return
		}
	}

	// 读取上传内容，支持 multipart form 与 application/json
	var content string
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(50 << 20); err != nil { // 最大 50MB
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"code": 400, "error": "解析上传表单失败: " + err.Error()})
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"code": 400, "error": "未找到上传的文件: " + err.Error()})
			return
		}
		defer file.Close()
		bytes, err := io.ReadAll(file)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"code": 500, "error": "读取文件内容失败: " + err.Error()})
			return
		}
		content = string(bytes)
	} else {
		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"code": 400, "error": "读取请求正文失败: " + err.Error()})
			return
		}
		content = string(bytes)
	}

	if strings.TrimSpace(content) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"code": 400, "error": "上传内容为空"})
		return
	}

	result, err := data.NewBackupManager().ImportIntelligentBackup(content)
	if err != nil {
		logger.SugaredLogger.Errorf("HandleBackupUpload import error: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"code":  400,
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"code":    200,
		"message": result,
	})
}
