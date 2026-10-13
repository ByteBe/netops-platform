// Package update 系统在线更新
package update

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/modreg"
)

var buildVersion = "v2.1.4"
var buildTime = "2026-10-13"

const maxUpdateSize = 500 * 1024 * 1024

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
		wantName := fmt.Sprintf("netops-%s-%s.zip", osTag, runtime.GOARCH)

		urls := []string{
			"https://gitee.com/api/v5/repos/ergn/netops-platform/releases/latest",
			"https://api.github.com/repos/ByteBe/netops-platform/releases/latest",
		}
		cli := &http.Client{Timeout: 10 * time.Second}
		for _, u := range urls {
			req, _ := http.NewRequest("GET", u, nil)
			req.Header.Set("User-Agent", "netops-updater")
			resp, err := cli.Do(req)
			if err != nil || resp.StatusCode != 200 {
				if resp != nil {
					resp.Body.Close()
				}
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
				"current":      buildVersion,
				"latest":       r.TagName,
				"notes":        r.Body,
				"download_url": dl,
				"asset":        wantName,
				"source":       u,
				"has_update":   r.TagName != buildVersion,
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
		if !isAllowedUpdateURL(req.URL) {
			response.Bad(c, "更新地址必须来自 GitHub/Gitee 官方发布地址")
			return
		}

		tmpZip := newUpdateTempPath()
		f, err := os.Create(tmpZip)
		if err != nil {
			response.Err(c, err)
			return
		}
		resp, err := http.Get(req.URL)
		if err != nil {
			f.Close()
			os.Remove(tmpZip)
			response.Bad(c, "下载失败: "+err.Error())
			return
		}
		if resp.StatusCode != 200 {
			f.Close()
			os.Remove(tmpZip)
			response.Bad(c, "下载失败 HTTP "+fmt.Sprint(resp.StatusCode))
			return
		}
		defer resp.Body.Close()
		if _, err := io.Copy(f, io.LimitReader(resp.Body, maxUpdateSize)); err != nil {
			f.Close()
			os.Remove(tmpZip)
			response.Bad(c, "下载失败: "+err.Error())
			return
		}
		f.Close()

		if err := applyUpdatePackage(tmpZip); err != nil {
			response.Bad(c, err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true, "message": "更新完成，3秒后重启"})
		go restartLater()
	})

	// POST /upload 上传更新包
	g.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			response.Bad(c, "未收到文件")
			return
		}
		if file.Size > maxUpdateSize {
			response.Bad(c, "更新包超过 500MB")
			return
		}
		tmpZip := newUpdateTempPath()
		if err := c.SaveUploadedFile(file, tmpZip); err != nil {
			response.Err(c, err)
			return
		}
		zr, err := zip.OpenReader(tmpZip)
		if err != nil {
			os.Remove(tmpZip)
			response.Bad(c, "更新包不是有效 ZIP："+err.Error())
			return
		}
		count := len(zr.File)
		zr.Close()
		if count == 0 {
			os.Remove(tmpZip)
			response.Bad(c, "更新包内容为空")
			return
		}
		response.OK(c, gin.H{
			"tmp_path": tmpZip,
			"filename": file.Filename,
			"size":     file.Size,
			"entries":  count,
			"valid":    true,
		})
	})

	// POST /apply 应用更新并重启
	g.POST("/apply", func(c *gin.Context) {
		var req struct {
			TmpPath string `json:"tmp_path"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.TmpPath == "" {
			response.Bad(c, "参数错误")
			return
		}
		if !isSafeUpdateTempPath(req.TmpPath) {
			response.Bad(c, "临时更新包路径非法")
			return
		}
		if err := applyUpdatePackage(req.TmpPath); err != nil {
			response.Bad(c, err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true, "message": "更新完成，3秒后重启"})
		go restartLater()
	})

	// POST /rollback 回滚到上一版本并重启
	g.POST("/rollback", func(c *gin.Context) {
		exePath, _ := os.Executable()
		exePath, _ = filepath.EvalSymlinks(exePath)
		if err := rollback(filepath.Dir(exePath)); err != nil {
			response.Bad(c, err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true, "message": "回滚完成，3秒后重启"})
		go restartLater()
	})
}

func init() {
	modreg.RegisterProtected("update", RegisterProtected)
}

func newUpdateTempPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("netops-update-%d.zip", time.Now().UnixNano()))
}

func isSafeUpdateTempPath(p string) bool {
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	tmpAbs, _ := filepath.Abs(os.TempDir())
	return strings.HasPrefix(abs, tmpAbs+string(filepath.Separator)) && strings.HasPrefix(filepath.Base(abs), "netops-update-")
}

func isAllowedUpdateURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "github.com" || host == "api.github.com" || host == "objects.githubusercontent.com" ||
		host == "gitee.com" || host == "api.gitee.com" || strings.HasSuffix(host, ".gitee.com")
}

func applyUpdatePackage(zipPath string) error {
	dst := zipPath + ".d"
	os.RemoveAll(dst)
	if err := unzip(zipPath, dst); err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}
	exeName := "netops-server"
	if runtime.GOOS == "windows" {
		exeName = "netops-server.exe"
	}
	newExe := filepath.Join(dst, exeName)
	if _, err := os.Stat(newExe); err != nil {
		_ = filepath.Walk(dst, func(p string, info os.FileInfo, e error) error {
			if e == nil && info.Name() == exeName {
				newExe = p
			}
			return nil
		})
	}
	if _, err := os.Stat(newExe); err != nil {
		return fmt.Errorf("更新包中未找到 %s", exeName)
	}

	exePath, _ := os.Executable()
	exePath, _ = filepath.EvalSymlinks(exePath)
	dir := filepath.Dir(exePath)
	ts := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(dir, "backups", ts)
	os.MkdirAll(backupDir, 0755)
	oldWeb := filepath.Join(dir, "web")

	// 永久备份本次被替换的版本，用于长期回滚。
	if err := copyFile(exePath, filepath.Join(backupDir, exeName)); err != nil {
		return fmt.Errorf("备份旧程序失败: %w", err)
	}
	if _, err := os.Stat(oldWeb); err == nil {
		_ = copyDir(oldWeb, filepath.Join(backupDir, "web"))
	}

	// 立即回滚用的临时备份。
	exeBak := exePath + ".bak"
	webBak := oldWeb + ".bak"
	os.Remove(exeBak)
	os.RemoveAll(webBak)
	if err := os.Rename(exePath, exeBak); err != nil {
		return fmt.Errorf("备份当前程序失败: %w", err)
	}
	if err := copyFile(newExe, exePath); err != nil {
		os.Rename(exeBak, exePath)
		return fmt.Errorf("替换失败: %w", err)
	}
	os.Chmod(exePath, 0755)

	newWeb := filepath.Join(dst, "web")
	if _, err := os.Stat(newWeb); err == nil {
		if _, err := os.Stat(oldWeb); err == nil {
			os.Rename(oldWeb, webBak)
			if err := copyDir(newWeb, oldWeb); err != nil {
				os.Rename(webBak, oldWeb)
				return fmt.Errorf("替换前端失败: %w", err)
			}
		} else {
			os.MkdirAll(oldWeb, 0755)
			_ = copyDir(newWeb, oldWeb)
		}
	}
	appendUpdateLog(dir, fmt.Sprintf("%s applied backup=%s", ts, backupDir))
	return nil
}

func appendUpdateLog(dir, text string) {
	f, _ := os.OpenFile(filepath.Join(dir, "update.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		defer f.Close()
		fmt.Fprintln(f, text)
	}
}

func rollback(dir string) error {
	exeName := "netops-server"
	if runtime.GOOS == "windows" {
		exeName = "netops-server.exe"
	}
	exePath := filepath.Join(dir, exeName)
	exeBak := exePath + ".bak"
	if _, err := os.Stat(exeBak); err == nil {
		os.Remove(exePath)
		if err := os.Rename(exeBak, exePath); err != nil {
			return err
		}
		web := filepath.Join(dir, "web")
		webBak := web + ".bak"
		if _, err := os.Stat(webBak); err == nil {
			os.RemoveAll(web)
			os.Rename(webBak, web)
		}
		appendUpdateLog(dir, time.Now().Format("20060102-150405")+" rolled back")
		return nil
	}

	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("没有可回滚的备份")
	}
	latest := entries[len(entries)-1]
	backupDir := filepath.Join(dir, "backups", latest.Name())
	oldExe := filepath.Join(backupDir, exeName)
	if _, err := os.Stat(oldExe); err != nil {
		return fmt.Errorf("备份目录中未找到可回滚程序")
	}
	os.Remove(exePath)
	if err := copyFile(oldExe, exePath); err != nil {
		return err
	}
	os.Chmod(exePath, 0755)
	if webDir := filepath.Join(backupDir, "web"); webDir != "" {
		if _, err := os.Stat(webDir); err == nil {
			os.RemoveAll(filepath.Join(dir, "web"))
			_ = copyDir(webDir, filepath.Join(dir, "web"))
		}
	}
	appendUpdateLog(dir, time.Now().Format("20060102-150405")+" rolled back from "+latest.Name())
	return nil
}

func restartLater() {
	time.Sleep(3 * time.Second)
	exePath, _ := os.Executable()
	exePath, _ = filepath.EvalSymlinks(exePath)
	dir := filepath.Dir(exePath)
	cmd := exec.Command(exePath, os.Args[1:]...)
	cmd.Dir = dir
	cmd.Start()
	os.Exit(0)
}

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	os.MkdirAll(dst, 0755)
	dstAbs, _ := filepath.Abs(dst)
	for _, f := range r.File {
		p := filepath.Join(dst, f.Name)
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		if abs != dstAbs && !strings.HasPrefix(abs, dstAbs+string(filepath.Separator)) {
			return fmt.Errorf("非法压缩包路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(p, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(p), 0755)
		out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(rc, maxUpdateSize)); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		t := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(t, 0755)
		}
		return copyFile(p, t)
	})
}
