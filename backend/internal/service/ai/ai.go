// Package ai AI 接入服务：云端API(OpenAI兼容) + 本地 Ollama + 主流大模型，支持多AI调度
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider 模型提供方配置
type Provider struct {
	Name        string
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
}

// Message 对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 对话请求
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调用 OpenAI 兼容接口（覆盖 OpenAI/DeepSeek/Qwen/GLM/本地Ollama等）
func Chat(p Provider, messages []Message) (string, error) {
	if p.BaseURL == "" {
		return "", fmt.Errorf("AI提供方 %s 未配置 BaseURL", p.Name)
	}
	base := strings.TrimRight(p.BaseURL, "/")
	// Ollama 原生端点兼容 /v1/chat/completions
	url := base + "/chat/completions"
	if strings.Contains(base, "/v1") {
		url = base + "/chat/completions"
	}
	reqBody := ChatRequest{Model: p.Model, Messages: messages, Temperature: p.Temperature}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("AI服务返回 %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	var cr ChatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("AI响应解析失败: %v", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("AI错误: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("AI未返回内容")
	}
	return cr.Choices[0].Message.Content, nil
}

// Test 测试连通性
func Test(p Provider) error {
	_, err := Chat(p, []Message{{Role: "user", Content: "ping"}})
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
