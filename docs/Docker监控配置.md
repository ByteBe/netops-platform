# Docker 监控配置

NetOps 平台通过 Docker Remote API（2375 端口）采集容器状态。

## 一、开启 Docker TCP 监控

```bash
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo tee /etc/systemd/system/docker.service.d/override.conf <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock -H tcp://0.0.0.0:2375
EOF
```

## 二、重启 Docker

```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

## 三、验证

```bash
sudo ss -tlnp | grep 2375
curl http://localhost:2375/version
```

返回 JSON 即表示成功。

## 四、在 NetOps 平台添加

1. 进入「容器监控」→「添加主机」
2. 地址填 `http://服务器IP:2375`
3. 名称自定义，保存
4. 平台自动采集容器状态、CPU、内存

> 生产环境建议仅对内网开放 2375 端口，或使用 TLS 加密。
