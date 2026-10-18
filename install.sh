#!/bin/bash
set -e
APP_DIR=/opt/netops-platform
CUR=$(pwd)
# 已在部署目录内（或解压到了 /opt/netops-platform）则跳过拷贝，避免 cp 自拷贝报错
if [ "$CUR" != "$APP_DIR" ]; then
    mkdir -p $APP_DIR
    cp -rf ./* $APP_DIR/
fi
cd $APP_DIR
chmod +x netops-server
cat > /etc/systemd/system/netops.service <<EOF
[Unit]
Description=NetOps Platform
After=network.target

[Service]
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/netops-server
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable netops
systemctl restart netops
sleep 2
echo "NetOps installed, visit http://$(hostname -I | awk '{print $1}'):40000"
