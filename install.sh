#!/bin/bash
set -e
APP_DIR=/opt/netops-platform
mkdir -p $APP_DIR
cp -rf ./* $APP_DIR/
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
echo "NetOps installed, visit http://$(hostname -I | awk '{print $1}'):30821"
