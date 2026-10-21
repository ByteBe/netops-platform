package certsvc

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/sm3"
)

// 每个测试用独立私钥路径，避免相互覆盖
func useTempKeyDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	PrivateKeyPath = filepath.Join(dir, "private.pem")
	t.Cleanup(func() { PrivateKeyPath = "data/certs/private.pem" })
}

func genKey(t *testing.T) *sm2.PrivateKey {
	t.Helper()
	key, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	return key
}

func TestSelfSignAndFingerprint(t *testing.T) {
	useTempKeyDir(t)
	key := genKey(t)
	pemCert, der, err := SelfSign(key, "node-a", "总部")
	if err != nil {
		t.Fatalf("SelfSign: %v", err)
	}
	fp, err := FingerprintPEM(pemCert)
	if err != nil {
		t.Fatalf("FingerprintPEM: %v", err)
	}
	if len(fp) != 64 {
		t.Fatalf("fingerprint len=%d, want 64", len(fp))
	}
	if fp != FingerprintDER(der) {
		t.Fatal("PEM/DER 指纹不一致")
	}
	// 自签校验 + 身份信息
	cert, err := Validate(pemCert)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cert.Subject.CommonName != "总部" || cert.Subject.SerialNumber != "node-a" {
		t.Fatalf("证书身份信息异常: CN=%s SN=%s", cert.Subject.CommonName, cert.Subject.SerialNumber)
	}
	// 有效期约 1 年
	if cert.NotAfter.Sub(cert.NotBefore) < 360*24*time.Hour {
		t.Fatalf("有效期异常: %v", cert.NotAfter.Sub(cert.NotBefore))
	}
	// 篡改检测：改动 base64 数据区一个字符后校验必须失败
	broken := pemCert[:100] + "XX" + pemCert[102:]
	if _, err := Validate(broken); err == nil {
		t.Fatal("篡改证书应校验失败")
	}
}

func TestChallengeSignVerify(t *testing.T) {
	useTempKeyDir(t)
	// A 的密钥对与证书
	keyA := genKey(t)
	pemA, _, err := SelfSign(keyA, "node-a", "总部")
	if err != nil {
		t.Fatal(err)
	}
	// B 的密钥对与证书
	keyB := genKey(t)
	pemB, _, err := SelfSign(keyB, "node-b", "分部")
	if err != nil {
		t.Fatal(err)
	}
	// 本机私钥文件 = A 的私钥（模拟 A 侧环境）
	if err := writePrivateKey(keyA); err != nil {
		t.Fatalf("write key: %v", err)
	}
	challenge := []byte("4h-auth-challenge-123456")
	// A 用自己私钥签名（SignChallenge 读本机私钥）
	sig, err := SignChallenge(challenge)
	if err != nil {
		t.Fatalf("SignChallenge: %v", err)
	}
	if len(sig) == 0 {
		t.Fatal("签名结果为空")
	}
	// A 的公证书验证自己签名 → 通过
	if err := VerifyChallenge(pemA, challenge, sig); err != nil {
		t.Fatalf("本机签名自证失败: %v", err)
	}
	// B 的公证书验证 A 的签名 → 应失败（签名者不匹配）
	if err := VerifyChallenge(pemB, challenge, sig); err == nil {
		t.Fatal("用 B 的公证书验证 A 的签名不应通过")
	}
	// B 用自己的私钥签，A 的公证书验证 → 应失败
	sigB, err := sm2Sign(keyB, challenge)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChallenge(pemA, challenge, sigB); err == nil {
		t.Fatal("A 的公证书验证 B 的签名不应通过")
	}
	// 篡改挑战数据 → 验证失败
	if err := VerifyChallenge(pemA, []byte("tampered"), sig); err == nil {
		t.Fatal("篡改挑战数据应验证失败")
	}
}

func TestPrivateKeyFilePermissions(t *testing.T) {
	useTempKeyDir(t)
	key := genKey(t)
	if err := writePrivateKey(key); err != nil {
		t.Fatalf("writePrivateKey: %v", err)
	}
	// 文件存在且非空
	b, err := os.ReadFile(PrivateKeyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("私钥文件为空")
	}
	// 加载回读可用（密钥对一致）
	loaded, err := loadPrivateKey()
	if err != nil {
		t.Fatalf("loadPrivateKey: %v", err)
	}
	if !loaded.PublicKey.Equal(&key.PublicKey) {
		t.Fatal("回读私钥与生成公钥不一致")
	}
	// 回读私钥可正常 SM3 签名验证
	challenge := []byte("perm-check")
	sig, err := sm2Sign(loaded, challenge)
	if err != nil {
		t.Fatal(err)
	}
	digest := sm3.Sum(challenge)
	if !sm2.VerifyASN1(&loaded.PublicKey, digest[:], sig) {
		t.Fatal("回读私钥签名验证失败")
	}
}
