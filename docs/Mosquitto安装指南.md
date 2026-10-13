apt update && apt install -y mosquitto mosquitto-clients
systemctl enable --now mosquitto
# 允许匿名（或加密码）
mosquitto_passwd -c /etc/mosquitto/passwd admin
# 输入密码两次，比如 tssbs123
cat > /etc/mosquitto/conf.d/netops.conf <<EOF
listener 1883
allow_anonymous false
password_file /etc/mosquitto/passwd
EOF
systemctl restart mosquitto