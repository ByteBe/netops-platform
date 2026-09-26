// Package sshsvc SSH 远程登录服务（IPAM 地址绑定命令下发，区别于 SNMP 监控）
package sshsvc

import (
	"bytes"
	"fmt"
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
	if err := session.RequestPty("vt100", 24, 100, modes); err != nil {
		return "", fmt.Errorf("分配终端失败: %w", err)
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

	var output bytes.Buffer
	time.Sleep(1500 * time.Millisecond)
	buf := make([]byte, 65536)
	// 读取初始输出
	readAll := func() {
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if err != nil || n < len(buf) {
				break
			}
		}
	}
	readAll()

	// 关闭分页（H3C/华为）
	stdin.Write([]byte("screen-length disable\n"))
	time.Sleep(1 * time.Second)
	readAll()

	for _, cmd := range commands {
		output.WriteString(fmt.Sprintf("\n$ %s\n", cmd))
		stdin.Write([]byte(cmd + "\n"))
		// 持续读取直到没有新数据
		emptyRounds := 0
		for i := 0; i < 60; i++ {
			before := output.Len()
			time.Sleep(1 * time.Second)
			readAll()
			after := output.Len()
			// 检查是否有 --More-- 分页需要按空格
			if bytes.Contains(output.Bytes(), []byte("-- More")) {
				stdin.Write([]byte(" "))
				time.Sleep(500 * time.Millisecond)
				readAll()
				emptyRounds = 0
				continue
			}
			if after == before {
				emptyRounds++
				if emptyRounds >= 3 {
					break // 连续3秒无新数据，认为输出完成
				}
			} else {
				emptyRounds = 0
			}
		}
	}

	stdin.Write([]byte("quit\n"))
	time.Sleep(500 * time.Millisecond)
	session.Close()

	return output.String(), nil
}