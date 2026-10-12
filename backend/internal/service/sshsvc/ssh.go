// Package sshsvc SSH 远程登录服务（IPAM 地址绑定命令下发，区别于 SNMP 监控）
package sshsvc

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Target SSH 目标
type Target struct {
	Host       string
	Port       int
	User       string
	AuthType   string
	Credential string
}

// Run 非交互执行单条命令（快，兼容不支持 PTY 的交换机）
func Run(t Target, cmd string) (string, error) {
	if t.Port == 0 {
		t.Port = 22
	}
	if t.User == "" {
		t.User = "admin"
	}
	var authMethods []ssh.AuthMethod
	if t.AuthType == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(t.Credential))
		if err != nil {
			return "", fmt.Errorf("解析私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = t.Credential
			}
			return answers, nil
		}))
		authMethods = append(authMethods, ssh.Password(t.Credential))
	}
	cfg := &ssh.ClientConfig{
		User:              t.User,
		Auth:              authMethods,
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{"ssh-rsa", "rsa-sha2-256", "rsa-sha2-512"},
		Timeout:           10 * time.Second,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", t.Host, t.Port), cfg)
	if err != nil {
		return "", fmt.Errorf("SSH连接失败: %w", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建会话失败: %w", err)
	}
	defer session.Close()
	out, err := session.Output(cmd)
	return string(out), err
}

// Test 简单连接测试（非交互，兼容不支持 PTY 的交换机）
func Test(t Target) (string, error) {
	if t.Port == 0 {
		t.Port = 22
	}
	if t.User == "" {
		t.User = "admin"
	}
	var authMethods []ssh.AuthMethod
	if t.AuthType == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(t.Credential))
		if err != nil {
			return "", fmt.Errorf("解析私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = t.Credential
			}
			return answers, nil
		}))
		authMethods = append(authMethods, ssh.Password(t.Credential))
	}
	cfg := &ssh.ClientConfig{
		User:              t.User,
		Auth:              authMethods,
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{"ssh-rsa", "rsa-sha2-256", "rsa-sha2-512"},
		Timeout:           10 * time.Second,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", t.Host, t.Port), cfg)
	if err != nil {
		return "", fmt.Errorf("SSH连接失败: %w", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建会话失败: %w", err)
	}
	defer session.Close()
	out, err := session.Output("display version | include version")
	if err != nil {
		return string(out), fmt.Errorf("命令执行失败: %w", err)
	}
	return string(out), nil
}

// Exec 执行命令并返回输出（交互式 shell 模式，兼容 H3C/华为交换机）
func Exec(t Target, commands []string) (string, error) {
	if t.Port == 0 {
		t.Port = 22
	}
	if t.User == "" {
		t.User = "admin"
	}

	var authMethods []ssh.AuthMethod
	if t.AuthType == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(t.Credential))
		if err != nil {
			return "", fmt.Errorf("解析私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = t.Credential
			}
			return answers, nil
		}))
		authMethods = append(authMethods, ssh.Password(t.Credential))
	}

	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{
			"ssh-rsa", "rsa-sha2-256", "rsa-sha2-512",
		},
		Timeout: 15 * time.Second,
		Config: ssh.Config{
			KeyExchanges: []string{
				"diffie-hellman-group1-sha1",
				"diffie-hellman-group14-sha1",
				"diffie-hellman-group14-sha256",
				"curve25519-sha256",
			},
			Ciphers: []string{
				"aes128-cbc", "aes128-ctr", "aes192-ctr", "aes256-ctr", "3des-cbc",
			},
			MACs: []string{
				"hmac-sha1", "hmac-sha1-96", "hmac-sha2-256",
			},
		},
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", t.Host, t.Port), cfg)
	if err != nil {
		return "", fmt.Errorf("SSH连接失败: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建会话失败: %w", err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	// PTY 分配失败时降级为无 PTY 模式（华三/部分交换机不支持）
	if err := session.RequestPty("vt100", 24, 100, modes); err != nil {
		// 忽略，继续无 PTY 模式
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("获取stdin失败: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("获取stdout失败: %w", err)
	}
	session.Stderr = session.Stdout

	if err := session.Shell(); err != nil {
		return "", fmt.Errorf("启动shell失败: %w", err)
	}

	var mu sync.Mutex
	var output bytes.Buffer
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 65536)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				mu.Lock()
				output.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				close(done)
				return
			}
		}
	}()
	readSnapshot := func() string {
		mu.Lock()
		defer mu.Unlock()
		return output.String()
	}
	waitQuiet := func(max time.Duration) {
		deadline := time.Now().Add(max)
		for time.Now().Before(deadline) {
			time.Sleep(250 * time.Millisecond)
			if bytes.Contains(output.Bytes(), []byte("-- More")) {
				stdin.Write([]byte(" "))
			}
		}
	}
	_ = readSnapshot

	// 关闭分页（H3C/华为）
	stdin.Write([]byte("screen-length disable\n"))
	waitQuiet(500 * time.Millisecond)

	for _, cmd := range commands {
		stdin.Write([]byte(cmd + "\n"))
		waitQuiet(2500 * time.Millisecond)
	}

	stdin.Write([]byte("quit\n"))
	waitQuiet(500 * time.Millisecond)
	stdin.Close()
	session.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	mu.Lock()
	defer mu.Unlock()
	return output.String(), nil
}
