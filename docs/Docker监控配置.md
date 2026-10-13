开启监控
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo tee /etc/systemd/system/docker.service.d/override.conf <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock -H tcp://0.0.0.0:2375
EOF

重启docker
sudo systemctl daemon-reload
sudo systemctl restart docker

验证
sudo ss -tlnp | grep 2375
curl http://localhost:2375/version