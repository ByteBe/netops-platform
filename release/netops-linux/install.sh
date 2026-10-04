#!/bin/bash
set -e
APP=/opt/netops-platform
BIN=$APP/netops-server
echo "[1/4] stop..."
systemctl stop netops 2>/dev/null || true
pkill -f netops-server 2>/dev/null || true
sleep 1
echo "[2/4] chmod..."
chmod +x $BIN
echo "[3/4] systemd..."
cat > /etc/systemd/system/netops.service <<EOF
[Unit]
Description=NetOps Platform
After=network.target

[Service]
Type=simple
WorkingDirectory=$APP
ExecStart=$BIN
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable netops
echo "[4/4] start..."
systemctl start netops
sleep 2
systemctl status netops --no-pager
echo "done"