// Package mcp MCP 接入：通过系统生成的接口将系统能力接入主流 Agent（服务器地址+接口形式）
// 系统暴露 /mcp/tools 与 /mcp/call 端点，供外部 Agent（支持MCP的智能体）发现与调用
package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Tool 工具定义（MCP 发现协议）
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Handler 工具执行函数（由模块注入）
type Handler func(args map[string]any) (map[string]any, error)

// Server MCP 服务器
type Server struct {
	tools   map[string]Tool
	handlers map[string]Handler
}

// NewServer 创建 MCP 服务器
func NewServer() *Server {
	return &Server{tools: map[string]Tool{}, handlers: map[string]Handler{}}
}

// RegisterTool 注册工具
func (s *Server) RegisterTool(name, desc string, schema map[string]any, h Handler) {
	s.tools[name] = Tool{Name: name, Description: desc, InputSchema: schema}
	s.handlers[name] = h
}

// ToolNames 工具名列表
func (s *Server) ToolNames() []string {
	names := make([]string, 0, len(s.tools))
	for n := range s.tools {
		names = append(names, n)
	}
	return names
}

// HandleHTTP 处理 /mcp 端点请求（POST JSON-RPC 风格）
func (s *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/mcp/tools":
		list := make([]Tool, 0, len(s.tools))
		for _, t := range s.tools {
			list = append(list, t)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tools": list})
	case r.URL.Path == "/mcp/call":
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Name string         `json:"name"`
			Args map[string]any `json:"args"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 400, "error": "参数错误"})
			return
		}
		h, ok := s.handlers[req.Name]
		if !ok {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 404, "error": "工具不存在: " + req.Name})
			return
		}
		result, err := h(req.Args)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 500, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "result": result})
	default:
		// MCP 服务信息（Agent 发现用）
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "NetOps-MCP", "version": "1.0.0",
			"server": r.Host, "tools_endpoint": "/mcp/tools", "call_endpoint": "/mcp/call",
			"description": "NetOps 网络运维平台能力接口（链路状态/设备监控/巡检报告/IPAM）",
		})
	}
}

// Agent 已注册的外部 Agent（服务器地址+接口形式接入）
type Agent struct {
	Name      string
	ServerURL string
	Endpoint  string
	AuthToken string
	Enable    bool
}

// CallAgent 调用外部 Agent 接口（主动通知/查询）
func CallAgent(a Agent, method, payload string) (string, error) {
	url := strings.TrimRight(a.ServerURL, "/") + "/" + strings.TrimLeft(a.Endpoint, "/")
	req, err := http.NewRequest(method, url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.AuthToken)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用Agent %s 失败: %w", a.Name, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return string(body), fmt.Errorf("Agent %s 返回 %d", a.Name, resp.StatusCode)
	}
	return string(body), nil
}
