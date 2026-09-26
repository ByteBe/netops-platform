// Package crypto 国密算法封装（SM2 非对称 / SM3 摘要 / SM4 对称）
// 加密流程：
//   1. 前端生成随机 SM4 会话密钥，用后端 SM2 公钥加密后随登录请求提交
//   2. 后端用 SM2 私钥解密得到会话密钥，绑定到登录令牌
//   3. 后续请求/响应体均使用 SM4-CBC 加密（IV 随机，随密文传输）
package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/sm3"
	"github.com/tjfoc/gmsm/sm4"
)

// SM2Key SM2 密钥对（16进制）
type SM2Key struct {
	PrivateHex string `json:"private_hex"`
	PublicHex  string `json:"public_hex"`
}

// GenerateSM2Key 生成 SM2 密钥对（公钥为 04||X||Y 共65字节，兼容 sm-crypto）
func GenerateSM2Key() (*SM2Key, error) {
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	size := (priv.Curve.Params().BitSize + 7) / 8
	pub := make([]byte, 1+2*size)
	pub[0] = 0x04
	priv.PublicKey.X.FillBytes(pub[1 : 1+size])
	priv.PublicKey.Y.FillBytes(pub[1+size:])
	return &SM2Key{
		PrivateHex: hex.EncodeToString(priv.D.Bytes()),
		PublicHex:  hex.EncodeToString(pub),
	}, nil
}

// SM2Decrypt 使用 SM2 私钥解密（兼容 sm-crypto doEncrypt 产物）
// 兼容格式：
//   - tjfoc/gmsm 标准密文：C1 为未压缩点(04||X||Y,65字节)或压缩点(02/03||X,33字节)
//   - sm-crypto 默认产物：C1 为无前缀点(X||Y,64字节)，需补 04 前缀
//   顺序同时兼容 C1C3C2 与 C1C2C3
func SM2Decrypt(privateHex, cipherHex string) ([]byte, error) {
	d, err := hex.DecodeString(privateHex)
	if err != nil || len(d) == 0 {
		return nil, errors.New("非法SM2私钥")
	}
	priv := new(sm2.PrivateKey)
	priv.Curve = sm2.P256Sm2()
	priv.D = new(big.Int).SetBytes(d)
	priv.PublicKey.Curve = priv.Curve
	priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(d)

	cipher, err := hex.DecodeString(cipherHex)
	if err != nil {
		return nil, errors.New("非法SM2密文")
	}
	out, err := trySM2Decrypt(priv, cipher)
	if err == nil {
		return out, nil
	}
	// 无前缀 C1（sm-crypto 的 X||Y，64字节）：补 04 前缀后重试
	if len(cipher) > 64 && cipher[0] != 0x04 && cipher[0] != 0x02 && cipher[0] != 0x03 {
		patched := make([]byte, 1+len(cipher))
		patched[0] = 0x04
		copy(patched[1:], cipher)
		out, err = trySM2Decrypt(priv, patched)
		if err == nil {
			return out, nil
		}
	}
	return nil, errors.New("SM2解密失败")
}

func trySM2Decrypt(priv *sm2.PrivateKey, cipher []byte) ([]byte, error) {
	out, err := sm2.Decrypt(priv, cipher, sm2.C1C3C2)
	if err == nil {
		return out, nil
	}
	return sm2.Decrypt(priv, cipher, sm2.C1C2C3)
}

// SM3Hex SM3 摘要（16进制）
func SM3Hex(data []byte) string {
	return hex.EncodeToString(sm3.Sm3Sum(data))
}

// SM3Hash SM3 摘要（原始字节）
func SM3Hash(data []byte) []byte {
	return sm3.Sm3Sum(data)
}

// SM4Encrypt SM4-CBC 加密，返回 base64(IV(16) + 密文)
func SM4Encrypt(key, plaintext []byte) (string, error) {
	if len(key) != 16 {
		return "", fmt.Errorf("SM4密钥必须为16字节，当前%d", len(key))
	}
	block, err := sm4.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := make([]byte, sm4.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	data := pkcs7Pad(plaintext, sm4.BlockSize)
	out := make([]byte, len(data))
	cbcEncrypt(block, iv, data, out)
	return base64.StdEncoding.EncodeToString(append(iv, out...)), nil
}

// SM4Decrypt 解密 base64(IV + 密文)
func SM4Decrypt(key []byte, envelope string) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("SM4密钥必须为16字节")
	}
	raw, err := base64.StdEncoding.DecodeString(envelope)
	if err != nil {
		return nil, errors.New("非法SM4密文格式")
	}
	if len(raw) < sm4.BlockSize*2 {
		return nil, errors.New("SM4密文长度不足")
	}
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := raw[:sm4.BlockSize]
	data := raw[sm4.BlockSize:]
	out := make([]byte, len(data))
	cbcDecrypt(block, iv, data, out)
	out, err = pkcs7Unpad(out, sm4.BlockSize)
	return out, err
}

// SM4KeyFromHex 将前端提交的32位16进制字符串转为16字节密钥
func SM4KeyFromHex(hexStr string) ([]byte, error) {
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, errors.New("会话密钥必须为16进制字符串")
	}
	if len(b) != 16 {
		return nil, fmt.Errorf("会话密钥长度错误: %d", len(b))
	}
	return b, nil
}

// PasswordHash SM3 加盐密码哈希
func PasswordHash(password, salt string) string {
	return SM3Hex([]byte(salt + ":" + password))
}

// GenerateSalt 生成随机盐
func GenerateSalt() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("非法填充")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize {
		return nil, errors.New("非法填充")
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, errors.New("非法填充")
		}
	}
	return data[:len(data)-pad], nil
}

func cbcEncrypt(block cipher.Block, iv, src, dst []byte) {
	prev := iv
	for i := 0; i < len(src); i += block.BlockSize() {
		xorBlock := make([]byte, block.BlockSize())
		for j := 0; j < block.BlockSize(); j++ {
			xorBlock[j] = src[i+j] ^ prev[j]
		}
		block.Encrypt(dst[i:i+block.BlockSize()], xorBlock)
		prev = dst[i : i+block.BlockSize()]
	}
}

func cbcDecrypt(block cipher.Block, iv, src, dst []byte) {
	prev := iv
	for i := 0; i < len(src); i += block.BlockSize() {
		block.Decrypt(dst[i:i+block.BlockSize()], src[i:i+block.BlockSize()])
		for j := 0; j < block.BlockSize(); j++ {
			dst[i+j] ^= prev[j]
		}
		prev = src[i : i+block.BlockSize()]
	}
}
