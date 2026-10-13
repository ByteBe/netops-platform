# Mosquitto MQTT Broker 安装指南

Mosquitto 是轻量 MQTT Broker，用于 NetOps 分布式级联部署。

## 一、安装

```bash
sudo apt update && sudo apt install -y mosquitto mosquitto-clients
sudo systemctl enable --now mosquitto
```

## 二、创建账号

```bash
sudo mosquitto_passwd -c /etc/mosquitto/passwd netops
# 按提示输入密码
```

## 三、配置

```bash
sudo tee /etc/mosquitto/conf.d/netops.conf <<'EOF'
listener 1883
allow_anonymous false
password_file /etc/mosquitto/passwd
EOF
```

## 四、重启生效

```bash
sudo systemctl restart mosquitto
```

## 五、验证

```bash
mosquitto_sub -h 127.0.0.1 -u netops -P 密码 -t test -d
# 另开终端：
mosquitto_pub -h 127.0.0.1 -u netops -P 密码 -t test -m "hello"
```

## 六、NetOps 初始化

分布式模式 MQTT 地址填 `服务器IP:1883`，账号 `netops`，密码同上。
