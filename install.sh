#!/usr/bin/env bash
# NetOps 网络运维监控平台 - Linux 部署脚本
# 用法:
#   ./install.sh           安装并启动（开机自启 + 后台运行）
#   ./install.sh start     仅启动
#   ./install.sh stop      停止
#   ./install.sh restart   重启
#   ./install.sh status    查看状态
#   ./install.sh uninstall 卸载服务
#   ./install.sh logs      查看日志
set -e

APP_NAME="netops-server"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_DIR="$SCRIPT_DIR/backend"
APP="$APP_DIR/$APP_NAME"
LOG_DIR="$APP_DIR/logs"
PID_FILE="$APP_DIR/$APP_NAME.pid"
SERVICE_NAME="netops"

mkdir -p "$LOG_DIR"

is_running() {
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            return 0
        fi
    fi
    pgrep -f "$APP_NAME" >/dev/null 2>&1
}

get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE" 2>/dev/null
    else
        pgrep -f "$APP_NAME" 2>/dev/null | head -1
    fi
}

install() {
    if [ ! -x "$APP" ]; then
        echo "[错误] 未找到 $APP，请先编译"
        exit 1
    fi
    echo "[NetOps] 安装 systemd 服务..."
    cat > /tmp/netops.service <<EOF
[Unit]
Description=NetOps Network Operations Platform
After=network.target

[Service]
Type=simple
WorkingDirectory=$APP_DIR
ExecStart=$APP
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
    sudo cp /tmp/netops.service /etc/systemd/system/${SERVICE_NAME}.service
    sudo systemctl daemon-reload
    sudo systemctl enable ${SERVICE_NAME}
    sudo systemctl start ${SERVICE_NAME}
    echo "[NetOps] 服务已安装并启动"
    echo "[NetOps] 访问地址: http://$(hostname -I | awk '{print $1}'):30821"
    echo "[NetOps] 状态查看: sudo systemctl status ${SERVICE_NAME}"
}

start() {
    if is_running; then
        echo "[NetOps] 已在运行 (PID: $(get_pid))"
        return
    fi
    cd "$APP_DIR"
    nohup ./"$APP_NAME" >> "$LOG_DIR/server.log" 2>&1 &
    echo $! > "$PID_FILE"
    echo "[NetOps] 已启动: http://127.0.0.1:30821"
}

stop() {
    if is_running; then
        pid=$(get_pid)
        kill "$pid" 2>/dev/null || true
        sleep 2
        kill -9 "$pid" 2>/dev/null || true
        rm -f "$PID_FILE"
        echo "[NetOps] 已停止"
    else
        echo "[NetOps] 未在运行"
    fi
}

status() {
    if is_running; then
        echo "[NetOps] 运行中 (PID: $(get_pid))"
    else
        echo "[NetOps] 未运行"
    fi
}

case "${1:-install}" in
    install)   install ;;
    start)     start ;;
    stop)      stop ;;
    restart)   stop; sleep 2; start ;;
    status)    status ;;
    uninstall)
        sudo systemctl stop ${SERVICE_NAME} 2>/dev/null || true
        sudo systemctl disable ${SERVICE_NAME} 2>/dev/null || true
        sudo rm -f /etc/systemd/system/${SERVICE_NAME}.service
        sudo systemctl daemon-reload
        echo "[NetOps] 服务已卸载"
        ;;
    logs)      tail -f "$LOG_DIR/server.log" ;;
    *)
        echo "用法: $0 {install|start|stop|restart|status|uninstall|logs}"
        exit 1
        ;;
esac
