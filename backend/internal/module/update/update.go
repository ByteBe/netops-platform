// Package update 系统在线更新
package update

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

var buildVersion = "v1.8.6"
var buildTime = "2026-10-08"

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/version", func(c *gin.Context) {
		response.OK(c, gin.H{
			"version": buildVersion, "build": buildTime,
			"os": runtime.GOOS, "arch": runtime.GOARCH,
		})
	})

	// GET /check 检查 GitHub/Gitee 最新版本
	g.GET("/check", func(c *gin.Context) {
		type asset struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}
		type rel struct {
			TagName string  `json:"tag_name"`
			Name    string  `json:"name"`
			Body    string  `json:"body"`
			Assets  []asset `json:"assets"`
		}
		osTag := runtime.GOOS
		if osTag == "darwin" {
			osTag = "mac"
		}
		ext := ".zip"
		wantName := fmt.Sprintf("netops-platform-%s-%s%s", osTag, runtime.GOARCH, ext)

		// 依次 GitHub -> Gitee
		urls := []string{
			"https://gitee.com/api/v5/repos/ergn/netops-platform/releases/latest",
			"https://api.github.com/repos/ByteBe/netops-platform/releases/latest",
		}
		cli := &http.Client{ Timeout: 10 * time.Second }
		for _, u := range urls {
			req, _ := http.NewRequest("GET", u, nil)
			req.Header.Set("User-Agent", "netops-updater")
			resp, err := cli.Do(req)
			if err != nil || resp.StatusCode != 200 {
				if resp != nil { resp.Body.Close() }
				continue
			}
			var r rel
			if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()
			var dl string
			for _, ar := range r.Assets {
				if ar.Name == wantName {
					dl = ar.BrowserDownloadURL
					break
				}
			}
			if dl == "" {
				continue
			}
			response.OK(c, gin.H{
				"current": buildVersion,
				"latest":  r.TagName,
				"notes":   r.Body,
				"download_url": dl,
				"asset": wantName,
				"source": u,
				"has_update": r.TagName != buildVersion,
			})
			return
		}
		response.OK(c, gin.H{"current": buildVersion, "latest": "", "has_update": false, "msg": "未找到更新源"})
	})

	// POST /online 下载并应用更新
	g.POST("/online", func(c *gin.Context) {
		var req struct {
			URL string `json:"url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
			response.Bad(c, "参数错误")
			return
		}
		tmpZip := filepath.Join(os.TempDir(), "netops-update-"+time.Now().Format("20060102150405")+".zip")
		f, err := os.Create(tmpZip)
		if err != nil {
			response.Err(c, err)
			return
		}
		resp, err := http.Get(req.URL)
		if err != nil {
			f.Close()
			response.Bad(c, "下载失败: "+err.Error())
			return
		}
		if resp.StatusCode != 200 {
			f.Close()
			os.Remove(tmpZip)
			response.Bad(c, "下载失败 HTTP "+fmt.Sprint(resp.StatusCode))
			return
		}
		io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()

		// 解压到临时目录
		dst := tmpZip + ".d"
		os.RemoveAll(dst)
		if err := unzip(tmpZip, dst); err != nil {
			response.Bad(c, "解压失败: "+err.Error())
			return
		}
		exeName := "netops-server"
		if runtime.GOOS == "windows" {
			exeName = "netops-server.exe"
		}
		newExe := filepath.Join(dst, exeName)
		if _, err := os.Stat(newExe); err != nil {
			// 可能在子目录
			_ = filepath.Walk(dst, func(p string, info os.FileInfo, e error) error {
				if e == nil && info.Name() == exeName {
					newExe = p
				}
				return nil
			})
		}
		if _, err := os.Stat(newExe); err != nil {
			response.Bad(c, "更新包中未找到 "+exeName)
			return
		}

		exePath, _ := os.Executable()
		exePath, _ = filepath.EvalSymlinks(exePath)
		dir := filepath.Dir(exePath)
		backup := exePath + ".bak"
		os.Remove(backup)
		os.Rename(exePath, backup)
		if err := copyFile(newExe, exePath); err != nil {
			os.Rename(backup, exePath)
			response.Bad(c, "替换失败: "+err.Error())
			return
		}
		os.Chmod(exePath, 0755)

		// 同步 web 目录
		newWeb := filepath.Join(dst, "web")
		if _, err := os.Stat(newWeb); err == nil {
			oldWeb := filepath.Join(dir, "web")
			os.Rename(oldWeb, oldWeb+".bak")
			if err := copyDir(newWeb, oldWeb); err != nil {
				os.Rename(oldWeb+".bak", oldWeb)
			}
		}

		response.OK(c, gin.H{"ok": true, "message": "更新完成，3秒后重启"})
		go func() {
			time.Sleep(3 * time.Second)
			cmd := exec.Command(exePath, os.Args[1:]...)
			cmd.Dir = dir
			cmd.Start()
			os.Exit(0)
		}()
	})

	// POST /upload 上传更新包
	g.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			response.Bad(c, "未收到文件")
			return
		}
		tmpZip := filepath.Join(os.TempDir(), "netops-update-"+time.Now().Format("20060102150405")+".zip")
		if err := c.SaveUploadedFile(file, tmpZip); err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, gin.H{
			"tmp_path": tmpZip,
			"filename": file.Filename,
			"size":     file.Size,
		})
	})

	// POST /apply 应用更新并重启
	g.POST("/apply", func(c *gin.Context) {
		var req struct{ TmpPath string `json:"tmp_path"` }
		if err := c.ShouldBindJSON(&req); err != nil || req.TmpPath == "" {
			response.Bad(c, "参数错误")
			return
		}
		dst := req.TmpPath + ".d"
		os.RemoveAll(dst)
		if err := unzip(req.TmpPath, dst); err != nil {
			response.Bad(c, "解压失败: "+err.Error())
			return
		}
		exeName := "netops-server"
		if runtime.GOOS == "windows" { exeName = "netops-server.exe" }
		newExe := filepath.Join(dst, exeName)
		if _, err := os.Stat(newExe); err != nil {
			_ = filepath.Walk(dst, func(p string, info os.FileInfo, e error) error {
				if e == nil && info.Name() == exeName { newExe = p }
				return nil
			})
		}
		if _, err := os.Stat(newExe); err != nil {
			response.Bad(c, "更新包中未找到 "+exeName)
			return
		}
		exePath, _ := os.Executable()
		exePath, _ = filepath.EvalSymlinks(exePath)
		dir := filepath.Dir(exePath)
		backup := exePath + ".bak"
		os.Remove(backup)
		os.Rename(exePath, backup)
		if err := copyFile(newExe, exePath); err != nil {
			os.Rename(backup, exePath)
			response.Bad(c, "替换失败: "+err.Error())
			return
		}
		os.Chmod(exePath, 0755)
		newWeb := filepath.Join(dst, "web")
		if _, err := os.Stat(newWeb); err == nil {
			oldWeb := filepath.Join(dir, "web")
			os.Rename(oldWeb, oldWeb+".bak")
			if err := copyDir(newWeb, oldWeb); err != nil {
				os.Rename(oldWeb+".bak", oldWeb)
			}
		}
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

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	os.MkdirAll(dst, 0755)
	for _, f := range r.File {
		p := filepath.Join(dst, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(p, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(p), 0755)
		out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil { return err }
		rc, err := f.Open()
		if err != nil { return err }
		io.Copy(out, rc)
		out.Close(); rc.Close()
	}
	return nil
}
func copyFile(src, dst string) error {
	in, err := os.Open(src); if err != nil { return err }
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755); if err != nil { return err }
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil { return err }
		rel, _ := filepath.Rel(src, p)
		t := filepath.Join(dst, rel)
		if info.IsDir() { return os.MkdirAll(t, 0755) }
		return copyFile(p, t)
	})
}
