package secstore

import (
	"crypto/sha256"
	"log"
	"netops/internal/common/crypto"
	"strings"
)

var key []byte
var warnedDefaultSeed bool

// Init 根据配置派生16字节SM4密钥
func Init(seed string) {
	if seed == "" {
		seed = "netops-default-at-rest-seed"
		if !warnedDefaultSeed {
			log.Println("[secstore] 警告：未配置敏感数据加密种子，请在初始化向导中设置独立密钥")
			warnedDefaultSeed = true
		}
	}
	sum := sha256.Sum256([]byte(seed))
	key = sum[:16]
}

// Seal 加密，已加密或空则原样返回
func Seal(plain string) string {
	if plain == "" || strings.HasPrefix(plain, "enc:") {
		return plain
	}
	out, err := crypto.SM4Encrypt(key, []byte(plain))
	if err != nil {
		log.Println("[secstore] 加密失败，拒绝明文落库:", err)
		return ""
	}
	return "enc:" + out
}

// Open 解密，非密文原样返回
func Open(cipher string) string {
	if !strings.HasPrefix(cipher, "enc:") {
		return cipher
	}
	out, err := crypto.SM4Decrypt(key, strings.TrimPrefix(cipher, "enc:"))
	if err != nil {
		return cipher
	}
	return string(out)
}
