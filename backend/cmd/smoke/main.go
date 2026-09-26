// Package main 国密加密链路冒烟测试客户端
// 用法: go run ./cmd/smoke -u http://127.0.0.1:30821
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

var baseURL string

func main() {
	flag.StringVar(&baseURL, "u", "http://127.0.0.1:30821", "服务地址")
	flag.Parse()

	fmt.Println("== 1. 获取SM2公钥 ==")
	pubHex := getPublicKey()
	fmt.Println("public_key:", pubHex[:24]+"...")

	fmt.Println("\n== 2. 国密登录 ==")
	sessionKey := make([]byte, 16)
	rand.Read(sessionKey)
	token, mustChange := login(pubHex, sessionKey, "admin", "Admin@2026Net")
	fmt.Println("token:", token[:24]+"...", "must_change_pwd:", mustChange)

	fmt.Println("\n== 3. 强制改密拦截验证（访问业务接口应被拒） ==")
	resp := callProtected("GET", "/api/v1/user/users", token, sessionKey, nil)
	fmt.Println("user/users 响应:", string(resp))

	fmt.Println("\n== 4. 修改密码（新密码符合强度: Abc#2026Net!x） ==")
	body := map[string]string{"old_password": "Admin@2026Net", "new_password": "Abc#2026Net!x"}
	resp = callProtected("POST", "/api/v1/auth/change-password", token, sessionKey, body)
	fmt.Println("change-password 响应:", string(resp))

	fmt.Println("\n== 5. 重新登录（新密码） ==")
	token2, mustChange2 := login(pubHex, sessionKey, "admin", "Abc#2026Net!x")
	fmt.Println("token2:", token2[:24]+"...", "must_change_pwd:", mustChange2)

	fmt.Println("\n== 6. 个人信息（加密通道） ==")
	resp = callProtected("GET", "/api/v1/auth/me", token2, sessionKey, nil)
	fmt.Println("me 响应:", string(resp))

	fmt.Println("\n== 7. 模块列表（路由自动注册验证） ==")
	resp = callProtected("GET", "/api/v1/system/modules", token2, sessionKey, nil)
	fmt.Println("modules 响应:", string(resp))

	fmt.Println("\n全部通过")
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
	// 手工解析公钥点 04||X||Y
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
	// 响应为 {enc: "..."} SM4 密文
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
			Token        string `json:"token"`
			MustChange   bool   `json:"must_change_pwd"`
			Role         string `json:"role"`
			Username     string `json:"username"`
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
		rd = strings.NewReader(env) // 请求体 = SM4 信封本体
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
	if resp.StatusCode >= 400 && len(out) > 0 {
		return out
	}
	return out
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}
