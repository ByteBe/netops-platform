// Package main 部署验证客户端：对当前运行实例做国密全链路 + 各模块接口验证
// 用法: go run ./cmd/verify -u http://127.0.0.1:30821 -p "Abc#2026Net!x"
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tjfoc/gmsm/sm2"

	"netops/internal/common/crypto"
)

var baseURL, password string

func main() {
	flag.StringVar(&baseURL, "u", "http://127.0.0.1:30821", "服务地址")
	flag.StringVar(&password, "p", "Abc#2026Net!x", "管理员密码")
	flag.Parse()

	fmt.Println("== 1. 获取SM2公钥 ==")
	pubHex := getPublicKey()
	fmt.Println("public_key:", pubHex[:24]+"...")

	fmt.Println("\n== 2. 国密登录 ==")
	sessionKey := make([]byte, 16)
	rand.Read(sessionKey)
	token, mustChange := login(pubHex, sessionKey, "admin", password)
	fmt.Println("token:", token[:24]+"...", "must_change_pwd:", mustChange)

	fmt.Println("\n== 3. 个人信息 ==")
	me := callProtected("GET", "/api/v1/auth/me", token, sessionKey, nil)
	fmt.Println("me:", string(me))

	fmt.Println("\n== 4. 模块列表（路由自动注册） ==")
	mods := callProtected("GET", "/api/v1/system/modules", token, sessionKey, nil)
	fmt.Println("modules:", string(mods))
	var modBody struct {
		Data []string `json:"data"`
	}
	json.Unmarshal(mods, &modBody)
	want := []string{"auth", "containermon", "dashboard", "dbmonitor", "ipam", "linkdetect", "monitor", "report", "resource", "scriptgen", "setup", "system", "topology", "traffic", "user"}
	missing := []string{}
	got := map[string]bool{}
	for _, m := range modBody.Data {
		got[m] = true
	}
	for _, w := range want {
		if !got[w] {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		panic("模块缺失: " + strings.Join(missing, ","))
	}
	fmt.Printf("15 个模块全部注册 ✓ (%d)\n", len(modBody.Data))

	fmt.Println("\n== 5. 各模块业务接口抽查 ==")
	checks := map[string]string{
		"链路检测":     "GET:/api/v1/linkdetect/tasks",
		"网络拓扑":     "GET:/api/v1/topology/graph",
		"设备监控":     "GET:/api/v1/monitor/devices",
		"资源管理":     "GET:/api/v1/resource/groups",
		"流量监控":     "GET:/api/v1/traffic/rules",
		"容器监控":     "GET:/api/v1/containermon/config",
		"数据库监控":    "GET:/api/v1/dbmonitor/types",
		"巡检报告":     "GET:/api/v1/report/list",
		"脚本生成器":    "GET:/api/v1/scriptgen/categories",
		"IP地址管理":   "GET:/api/v1/ipam/subnets",
		"IPAM绑定设置":  "GET:/api/v1/ipam/settings",
		"系统AI":     "GET:/api/v1/system/ai/list",
		"系统MCP":    "GET:/api/v1/system/mcp/agents",
		"系统邮箱":     "GET:/api/v1/system/email/list",
		"系统参数":     "GET:/api/v1/system/settings",
		"用户管理":     "GET:/api/v1/user/users",
	}
	for name, spec := range checks {
		parts := strings.SplitN(spec, ":", 2)
		method, path := parts[0], parts[1]
		resp := callProtected(method, path, token, sessionKey, nil)
		// 加密信封已解密为 JSON，仅需 code==0
		var r struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(resp, &r); err != nil || r.Code != 0 {
			panic(fmt.Sprintf("%s 接口异常: %s", name, truncate(string(resp), 160)))
		}
		fmt.Printf("  ✓ %s\n", name)
	}

	fmt.Println("\n== 6. 前端页面（30821 静态托管） ==")
	resp := do("GET", "/", "", nil, nil)
	fmt.Println("index.html 长度:", len(resp))

	fmt.Println("\n全部验证通过")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func getPublicKey() string {
	resp := do("GET", "/api/v1/crypto/public-key", "", nil, nil)
	var out struct {
		Data struct {
			PublicKey string `json:"public_key"`
		} `json:"data"`
	}
	must(json.Unmarshal(resp, &out))
	if out.Data.PublicKey == "" {
		panic("公钥为空")
	}
	return out.Data.PublicKey
}

func login(pubHex string, sessionKey []byte, username, password string) (string, bool) {
	pubBytes, _ := hex.DecodeString(pubHex)
	curve := sm2.P256Sm2()
	pub := &sm2.PublicKey{Curve: curve}
	pub.X, pub.Y = curve.ScalarBaseMult(nil)
	if len(pubBytes) == 65 && pubBytes[0] == 0x04 {
		pub.X = new(big.Int).SetBytes(pubBytes[1:33])
		pub.Y = new(big.Int).SetBytes(pubBytes[33:])
	} else {
		panic("SM2公钥格式错误")
	}
	encKey, err := sm2.Encrypt(pub, sessionKey, rand.Reader, sm2.C1C3C2)
	must(err)
	payload, _ := json.Marshal(map[string]string{"username": username, "password": password})
	dataEnv, err := crypto.SM4Encrypt(sessionKey, payload)
	must(err)
	reqBody, _ := json.Marshal(map[string]string{"key": hex.EncodeToString(encKey), "data": dataEnv})

	resp := do("POST", "/api/v1/auth/login", "", bytes.NewReader(reqBody), nil)
	var env struct {
		Enc string `json:"enc"`
	}
	if err := json.Unmarshal(resp, &env); err != nil || env.Enc == "" {
		panic("登录响应非加密信封: " + string(resp))
	}
	plain, err := crypto.SM4Decrypt(sessionKey, env.Enc)
	must(err)
	var body struct {
		Code int `json:"code"`
		Data struct {
			Token      string `json:"token"`
			MustChange bool   `json:"must_change_pwd"`
		} `json:"data"`
	}
	must(json.Unmarshal(plain, &body))
	if body.Data.Token == "" {
		panic("登录失败: " + string(plain))
	}
	return body.Data.Token, body.Data.MustChange
}

func callProtected(method, path, token string, sessionKey []byte, body any) []byte {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		env, err := crypto.SM4Encrypt(sessionKey, raw)
		must(err)
		rd = strings.NewReader(env)
	}
	resp := do(method, path, token, rd, sessionKey)
	var env struct {
		Enc string `json:"enc"`
	}
	if err := json.Unmarshal(resp, &env); err == nil && env.Enc != "" {
		plain, err := crypto.SM4Decrypt(sessionKey, env.Enc)
		if err == nil {
			return plain
		}
	}
	return resp
}

func do(method, path, token string, body io.Reader, sessionKey []byte) []byte {
	req, err := http.NewRequest(method, baseURL+path, body)
	must(err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionKey != nil && body != nil {
		req.Header.Set("X-Enc", "sm4")
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	must(err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	must(err)
	return out
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}
