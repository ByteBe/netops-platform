// Package webhook 飞书/钉钉/企业微信机器人 Webhook 发送，自动处理平台加签
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SendText 发送纯文本消息到机器人 Webhook。
// typ: feishu / dingtalk / wecom（其它类型按钉钉消息格式发送）
// secret: 平台加签密钥，为空则不签名
// 仅 HTTP 2xx 视为送达，否则返回包含平台错误码的 error
func SendText(typ, rawURL, secret, text string) error {
	var out []byte
	if typ == "feishu" {
		out, _ = json.Marshal(map[string]any{"msg_type": "text", "content": map[string]string{"text": text}})
	} else {
		out, _ = json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": text}})
	}
	// 钉钉加签走 URL 参数（毫秒时间戳 + sign）
	if typ == "dingtalk" && secret != "" {
		rawURL = signedDingTalkURL(rawURL, secret)
	}
	req, err := http.NewRequest("POST", rawURL, bytes.NewReader(out))
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 飞书加签走请求头（秒时间戳 + 对请求体 HMAC 签名）
	if typ == "feishu" && secret != "" {
		signFeishu(req, out, secret)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("返回 %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return nil
}

// signFeishu 飞书自定义机器人加签：
// X-Lark-Signature = base64(HMAC-SHA256(key=timestamp+"\n"+secret, msg=请求体))
func signFeishu(req *http.Request, body []byte, secret string) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(ts+"\n"+secret))
	mac.Write(body)
	req.Header.Set("X-Lark-Timestamp", ts)
	req.Header.Set("X-Lark-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

// signedDingTalkURL 钉钉机器人加签：
// 在 URL 追加 timestamp(毫秒) 与 sign = URLEncode(base64(HMAC-SHA256(key=secret, msg=timestamp+"\n"+secret)))
func signedDingTalkURL(rawURL, secret string) string {
	if secret == "" {
		return rawURL
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "\n" + secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + "timestamp=" + ts + "&sign=" + url.QueryEscape(sign)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
