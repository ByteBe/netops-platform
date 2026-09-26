// Package update 系统在线更新：上传二进制包
package update

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/modreg"
)

var buildVersion = "v1.0.0"
var buildTime = "2026-09-26"

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/update/version", func(c *gin.Context) {
		response.OK(c, gin.H{
			"version": buildVersion, "build": buildTime,
			"os": runtime.GOOS, "arch": runtime.GOARCH,
		})
	})

	g.POST("/update/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			response.Bad(c, "请选择文件")
			return
		}
		tmpPath := filepath.Join(os.TempDir(), "netops-up-"+time.Now().Format("20060102150405"))
		if err := c.SaveUploadedFile(file, tmpPath); err != nil {
			response.Err(c, err)
			return
		}
		f, _ := os.Open(tmpPath)
		h := sha256.New()
		io.Copy(h, f)
		f.Close()
		response.OK(c, gin.H{
			"ok": true, "filename": file.Filename, "size": file.Size,
			"sha256": hex.EncodeToString(h.Sum(nil)), "tmp_path": tmpPath,
		})
	})

	g.POST("/update/apply", func(c *gin.Context) {
		var req struct {
			TmpPath string `json:"tmp_path"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.TmpPath == "" {
			response.Bad(c, "参数错误")
			return
		}
		exePath, err := os.Executable()
		if err != nil {
			response.Err(c, err)
			return
		}
		exePath, _ = filepath.EvalSymlinks(exePath)
		dir := filepath.Dir(exePath)
		backup := exePath + ".bak"
		os.Remove(backup)
		os.Rename(exePath, backup)
		if err := os.Rename(req.TmpPath, exePath); err != nil {
			os.Rename(backup, exePath)
			response.Bad(c, "替换失败: "+err.Error())
			return
		}
		os.Chmod(exePath, 0755)
		response.OK(c, gin.H{"ok": true, "message": "更新完成，3秒后重启"})
		go func() {
			time.Sleep(3 * time.Second)
			cmd := exec.Command(exePath, os.Args[1:]...)
			cmd.Dir = dir
			cmd.Start()
			os.Exit(0)
		}()
	})
}

func init() {
	modreg.RegisterProtected("update", RegisterProtected)
}
