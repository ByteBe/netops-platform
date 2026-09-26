// Package password 密码强度校验（大于12位：大写+小写+数字+符号）
package password

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"unicode"
)

// Validate 校验密码强度
// 规则：长度 > 12，且同时包含大写字母、小写字母、数字、符号
func Validate(pwd string) error {
	if len(pwd) <= 12 {
		return fmt.Errorf("密码长度必须大于12位（当前%d位）", len(pwd))
	}
	hasUpper, hasLower, hasDigit, hasSymbol := false, false, false, false
	for _, r := range pwd {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r) || r == ' ':
			hasSymbol = true
		}
	}
	if !hasUpper {
		return fmt.Errorf("密码必须包含大写字母")
	}
	if !hasLower {
		return fmt.Errorf("密码必须包含小写字母")
	}
	if !hasDigit {
		return fmt.Errorf("密码必须包含数字")
	}
	if !hasSymbol {
		return fmt.Errorf("密码必须包含符号（如 !@#$%%^&*）")
	}
	return nil
}

// Strength 返回密码强度等级（前端展示用）
func Strength(pwd string) int {
	if len(pwd) <= 12 {
		return 1
	}
	classes := 0
	for _, r := range pwd {
		switch {
		case unicode.IsUpper(r):
			classes |= 1
		case unicode.IsLower(r):
			classes |= 2
		case unicode.IsDigit(r):
			classes |= 4
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			classes |= 8
		}
	}
	n := 0
	for classes > 0 {
		n += classes & 1
		classes >>= 1
	}
	return n // 2-4
}

// Generate 生成符合强度规则的随机密码（14位，管理员创建用户/重置密码用）
func Generate() string {
	const (
		upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		lower   = "abcdefghijkmnopqrstuvwxyz"
		digits  = "23456789"
		symbols = "!@#$%^&*_-+="
	)
	chars := upper + lower + digits + symbols
	pwd := make([]byte, 14)
	// 确保每类至少一个
	sets := []string{upper, lower, digits, symbols}
	for i := 0; i < 4; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sets[i]))))
		pwd[i] = sets[i][n.Int64()]
	}
	for i := 4; i < 14; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		pwd[i] = chars[n.Int64()]
	}
	// Fisher-Yates 打乱
	for i := len(pwd) - 1; i > 0; i-- {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		j := int(n.Int64())
		pwd[i], pwd[j] = pwd[j], pwd[i]
	}
	return string(pwd)
}
