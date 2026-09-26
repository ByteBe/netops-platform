// Package email 邮件发送服务（通过用户填写的邮箱发送告警与巡检报告）
package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// Config 邮件配置
type Config struct {
	Name    string
	SMTPHost string
	SMTPPort int
	User    string
	Password string
	UseSSL  bool
	Enable  bool
	From    string
}

// Send 发送邮件（支持HTML正文与多个收件人）
func Send(cfg Config, to []string, subject, bodyHTML string) error {
	if !cfg.Enable {
		return fmt.Errorf("邮箱功能未启用（%s）", cfg.Name)
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 465
	}
	from := cfg.User
	if cfg.From != "" {
		from = cfg.From
	}
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		from, strings.Join(to, ","), subject)
	msg := []byte(header + bodyHTML)

	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.SMTPHost)
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	if cfg.UseSSL {
		return sendSSL(addr, from, to, msg, cfg, auth)
	}
	return smtp.SendMail(addr, auth, from, to, msg)
}

func sendSSL(addr, from string, to []string, msg []byte, cfg Config, auth smtp.Auth) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.SMTPHost, InsecureSkipVerify: false})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, t := range to {
		if err := client.Rcpt(t); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
