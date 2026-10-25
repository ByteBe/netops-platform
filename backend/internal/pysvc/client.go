// Package pysvc Python 智能外围（FastAPI）统一调用层。
//
// 融合架构约定：
//   - Go 主程序负责设备采集、稳定服务与统一 API 网关；
//   - AI 巡检、脚本生成、报告渲染等"智能外围"功能由 Python（pyservice）实现；
//   - 前端/使用者仍调用 Go API，由本层统一转发到 Python，Python 不可用时按各模块
//     策略降级（回退内置 Go 实现或返回明确提示），使用者无感知。
package pysvc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client Python 智能外围客户端。
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New 创建客户端；baseURL 为空或非法时 Enabled() 返回 false（转发禁用，走内置实现）。
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Enabled 返回转发是否启用（配置了 python_service_url）。
func (c *Client) Enabled() bool { return c != nil && c.BaseURL != "" }

// PostJSON 向 Python 服务发送 JSON POST 并解码响应到 out（out 可为 nil）。
// path 为 Python 内部路由，如 "/internal/aiinspect/run"。
// timeout > 0 时覆盖默认 30s（如 LLM 生成需要更长超时）。
func (c *Client) PostJSON(path string, req, out any, timeout time.Duration) error {
	if !c.Enabled() {
		return fmt.Errorf("python svc disabled")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	client := c.HTTP
	if timeout > 0 {
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Post(c.BaseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("python svc %s status %d", path, resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return nil
}

// Ping 健康检查（GET /healthz）。
func (c *Client) Ping() error {
	if !c.Enabled() {
		return fmt.Errorf("python svc disabled")
	}
	resp, err := c.HTTP.Get(c.BaseURL + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("python svc healthz status %d", resp.StatusCode)
	}
	return nil
}
