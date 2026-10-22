package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// 飞书加签：X-Lark-Signature 必须等于用 timestamp+"\n"+secret 对请求体做 HMAC-SHA256 的 base64
func TestSignFeishu(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"msg_type":"text","content":{"text":"hello"}}`)
	req, _ := http.NewRequest("POST", "https://open.feishu.cn/open-apis/bot/v2/hook/x", strings.NewReader(string(body)))
	signFeishu(req, body, secret)

	ts := req.Header.Get("X-Lark-Timestamp")
	if ts == "" {
		t.Fatal("缺少 X-Lark-Timestamp")
	}
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		t.Fatalf("X-Lark-Timestamp 非秒级时间戳: %s", ts)
	}
	mac := hmac.New(sha256.New, []byte(ts+"\n"+secret))
	mac.Write(body)
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if got := req.Header.Get("X-Lark-Signature"); got != want {
		t.Fatalf("签名不匹配:\n got=%s\nwant=%s", got, want)
	}
}

// 钉钉加签：URL 追加 timestamp(毫秒) 与 sign=urlencode(base64(hmac(secret, ts+"\n"+secret)))
func TestSignedDingTalkURL(t *testing.T) {
	secret := "test-secret"
	raw := "https://oapi.dingtalk.com/robot/send?access_token=abc"
	got := signedDingTalkURL(raw, secret)

	if !strings.HasPrefix(got, raw+"&") {
		t.Fatalf("应在原 URL 后以 & 追加参数: %s", got)
	}
	qs := got[strings.Index(got, "?")+1:]
	parts := map[string]string{}
	for _, kv := range strings.Split(qs, "&") {
		p := strings.SplitN(kv, "=", 2)
		parts[p[0]] = p[1]
	}
	ts := parts["timestamp"]
	if ts == "" {
		t.Fatal("缺少 timestamp")
	}
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		t.Fatalf("timestamp 非毫秒时间戳: %s", ts)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "\n" + secret))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if parts["sign"] != strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(want, "+", "%2B"), "/", "%2F"), "=", "%3D") {
		t.Fatalf("sign 未按 URL 编码:\n got=%s\nwant=%s", parts["sign"], want)
	}
	// 无 secret 时不签名
	if out := signedDingTalkURL(raw, ""); out != raw {
		t.Fatalf("空 secret 应保持 URL 原样: %s", out)
	}
	// 已含 ? 的 URL 用 & 追加
	if out := signedDingTalkURL(raw, secret); !strings.Contains(out, "&timestamp=") {
		t.Fatalf("已有 ? 的 URL 应追加 &: %s", out)
	}
}

// SendText 到本地假服务器：2xx 算成功、非 2xx 报错带平台错误码、钉钉加签 URL 参数正确传递
func TestSendTextStatus(t *testing.T) {
	srv := newTestServer(t)
	// 200 成功
	if err := SendText("wecom", srv.URL+"/ok", "", "hi"); err != nil {
		t.Fatalf("200 应成功: %v", err)
	}
	// 非 2xx 返回含状态码
	if err := SendText("dingtalk", srv.URL+"/err", "", "hi"); err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("非 2xx 应报错并带状态码: %v", err)
	}
	// 钉钉加签 URL 携带 timestamp/sign
	if err := SendText("dingtalk", srv.URL+"/ok", "secret", "hi"); err != nil {
		t.Fatalf("钉钉加签发送失败: %v", err)
	}
}

func newTestServer(t *testing.T) *testHTTPServer {
	t.Helper()
	srv := &testHTTPServer{requests: make(chan *http.Request, 8)}
	hs := &http.Server{Addr: "127.0.0.1:0", Handler: srv}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv.ln = ln
	srv.hs = hs
	srv.URL = "http://" + ln.Addr().String()
	go hs.Serve(ln)
	t.Cleanup(func() { hs.Close() })
	return srv
}

type testHTTPServer struct {
	requests chan *http.Request
	ln       net.Listener
	hs       *http.Server
	URL      string
}

func (s *testHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.requests <- r
	switch r.URL.Path {
	case "/ok":
		w.WriteHeader(200)
		w.Write([]byte(`{"errcode":0}`))
	case "/err":
		w.WriteHeader(500)
		w.Write([]byte(`{"errcode":1,"errmsg":"internal"}`))
	default:
		w.WriteHeader(404)
	}
}
