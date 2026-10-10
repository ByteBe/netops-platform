#!/bin/bash
set -e
cd "$(dirname "$0")"
case "$1" in
  install)
    chmod +x netops-server
    cat > /etc/systemd/system/netops.service <<'EOF'
[Unit]
Description=NetOps Platform
After=network.target
[Service]
WorkingDirectory=/opt/netops-platform
ExecStart=/opt/netops-platform/netops-server
Restart=always
[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable netops
    systemctl start netops
    echo "NetOps installed, visit http://$(hostname -I | awk '{print $1}'):30821"
    ;;
  *) echo "Usage: $0 install";;
esac
