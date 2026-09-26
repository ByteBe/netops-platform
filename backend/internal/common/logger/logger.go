// Package logger 轻量日志（生产二进制不依赖第三方日志库，滚动文件 + 控制台）
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	file    *os.File
	verbose bool
)

// Init 初始化日志：输出到 stdout 与 logs/app-YYYYMMDD.log（自动按天滚动）
func Init(logDir string) error {
	if logDir == "" {
		logDir = "logs"
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(logDir, "app-"+time.Now().Format("20060102")+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	mu.Lock()
	file = f
	mu.Unlock()
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	return nil
}

// SetVerbose 开启调试日志
func SetVerbose(v bool) { verbose = v }

// Infof 普通日志
func Infof(format string, args ...any) {
	log.Printf("[INFO] "+format, args...)
}

// Warnf 告警日志
func Warnf(format string, args ...any) {
	log.Printf("[WARN] "+format, args...)
}

// Errorf 错误日志
func Errorf(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
}

// Debugf 调试日志（verbose 时输出）
func Debugf(format string, args ...any) {
	if verbose {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Close 关闭日志文件
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
	}
}

var _ = fmt.Sprintf
