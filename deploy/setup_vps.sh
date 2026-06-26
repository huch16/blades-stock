#!/bin/bash
set -e

# ================================================================
# VPS 一键初始化脚本 (Ubuntu/Debian)
# 用法: ssh root@your-vps 'bash -s' < setup_vps.sh
# ================================================================

APP_USER=deploy
APP_DIR=/opt/ai-assistant-web
APP_PORT=18081
DOMAIN=${1:-"your-domain.com"}  # 传参或手动修改

echo ">>> 创建系统用户 $APP_USER"
id -u $APP_USER &>/dev/null || useradd -m -s /bin/bash $APP_USER

echo ">>> 创建应用目录 $APP_DIR"
mkdir -p $APP_DIR/data $APP_DIR/logs
chown -R $APP_USER:$APP_USER $APP_DIR

echo ">>> 安装系统依赖"
apt-get update
apt-get install -y nginx certbot python3-certbot-nginx

echo ">>> 创建 systemd service"
cat > /etc/systemd/system/ai-assistant-web.service << 'SERVICE'
[Unit]
Description=ai-assistant-web
After=network.target

[Service]
Type=simple
User=deploy
Group=deploy
WorkingDirectory=/opt/ai-assistant-web
ExecStart=/opt/ai-assistant-web/ai-assistant-web-server
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload

echo ">>> 创建 nginx 反向代理配置"
cat > /etc/nginx/sites-available/ai-assistant-web << 'NGINX'
server {
    listen 80;
    server_name _;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:18081;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_cache_bypass $http_upgrade;

        # SSE 支持（流式输出）
        proxy_set_header Cache-Control no-cache;
        proxy_read_timeout 86400s;
    }
}
NGINX

ln -sf /etc/nginx/sites-available/ai-assistant-web /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx

echo ""
echo "==========================================================="
echo "  VPS 初始化完成！"
echo ""
echo "  接下来:"
echo "  1. 设置域名: 修改 /etc/nginx/sites-available/ai-assistant-web 中的 server_name"
echo "  2. 配置 HTTPS: sudo certbot --nginx -d your-domain.com"
echo "  3. 在 GitHub 仓库设置 Secrets:"
echo "     - VPS_HOST:    你的服务器 IP"
echo "     - VPS_USER:    $APP_USER"
echo "     - VPS_SSH_KEY: deploy 用户的私钥"
echo "     - VPS_PORT:    22 (可省略)"
echo ""
echo "  4. 手动启动: sudo systemctl enable --now ai-assistant-web"
echo "  5. 检查状态: sudo systemctl status ai-assistant-web"
echo "==========================================================="
