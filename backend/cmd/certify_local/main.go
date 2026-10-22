package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"netops/internal/model"
	"netops/internal/service/certsvc"
)

// certify_local 一次性工具：为 Windows 本机 netops 生成私有证书并写入 MQTT 级联配置
// 用法：在 backend 目录下执行 go run ./cmd/certify_local
func main() {
	db, err := gorm.Open(sqlite.Open("data/netops.db"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Println("打开数据库失败:", err)
		os.Exit(1)
	}

	// 1. 读取本机节点身份
	uuid := getSetting(db, "self_node_uuid")
	name := getSetting(db, "self_node_name")
	if uuid == "" {
		fmt.Println("self_node_uuid 为空，拒绝执行")
		os.Exit(1)
	}
	fmt.Printf("本机节点: uuid=%s name=%s\n", uuid, name)

	// 2. 生成本机私有证书 + 公证书
	if err := certsvc.Ensure(db, uuid, name); err != nil {
		fmt.Println("生成本机证书失败:", err)
		os.Exit(1)
	}
	fmt.Println("本机证书已生成: fingerprint=" + certsvc.PrivateKeyFingerprint())

	// 3. 写入 MQTT 级联配置
	writeSetting(db, "mqtt_broker", "wss://82.156.100.53:8443")
	writeSetting(db, "deploy_mode", "distributed")
	if ca, err := os.ReadFile("../mqtt-wss-ca-53.crt"); err == nil {
		writeSetting(db, "mqtt_tls_ca", strings.TrimSpace(string(ca)))
		fmt.Println("mqtt_tls_ca 已写入 (53 CA)")
	} else {
		fmt.Println("未找到 mqtt-wss-ca-53.crt:", err)
	}
	if pc, err := os.ReadFile("../parent-cert-53.pem"); err == nil {
		writeSetting(db, "parent_cert", strings.TrimSpace(string(pc)))
		fmt.Println("parent_cert 已写入 (53 公证书)")
	} else {
		fmt.Println("未找到 parent-cert-53.pem:", err)
	}
	fmt.Println("完成")
}

func getSetting(db *gorm.DB, k string) string {
	var s model.SystemSetting
	if err := db.Where("`key` = ?", k).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

func writeSetting(db *gorm.DB, k, v string) {
	var s model.SystemSetting
	if err := db.Where("`key` = ?", k).First(&s).Error; err == nil {
		db.Model(&s).Update("value", v)
	} else {
		db.Create(&model.SystemSetting{Key: k, Value: v})
	}
	fmt.Printf("system_settings[%s] 已写入 (%d 字符)\n", k, len(v))
}
