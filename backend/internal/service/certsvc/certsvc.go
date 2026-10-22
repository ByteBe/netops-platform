// Package certsvc 分布式节点证书服务（国密 SM2）
// 每个节点首次启动时生成 SM2 密钥对与 x509 自签公证书（有效期 365 天，SM2-SM3 签名）：
//   - 公证书（公有证书）可入库、可交换分发：上级把自己的公证书下发给下级，下级把自己的公证书上送上级
//   - 私钥（私有证书）仅存本机文件 data/certs/private.pem（0600），绝不入库、绝不外发
//   - 证书 SM3 指纹作为数据上报的特征标签（cert_fingerprint），配合 4 小时周期双向认证
package certsvc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/sm3"
	"github.com/emmansun/gmsm/smx509"

	"gorm.io/gorm"

	"netops/internal/common/logger"
	"netops/internal/model"
)

var (
	PrivateKeyPath = "data/certs/private.pem" // 本机私钥文件（相对工作目录）
	certValidity   = 365 * 24 * time.Hour     // 证书有效期 1 年
	// nodeInfoOID 自定义扩展 OID：证书内嵌本机参数（IP/MAC/系统版本等）
	nodeInfoOID = []int{1, 3, 6, 1, 4, 1, 53131, 1, 1}
)

// NodeInfo 证书内嵌的节点运行参数
type NodeInfo struct {
	Hostname  string   `json:"hostname"`
	IPs       []string `json:"ips"`
	MACs      []string `json:"macs"`
	OS        string   `json:"os"`
	OSVersion string   `json:"os_version"`
	Arch      string   `json:"arch"`
}

// CollectNodeInfo 采集本机参数：主机名、非回环 IPv4、非回环 MAC、操作系统与版本
func CollectNodeInfo() NodeInfo {
	info := NodeInfo{Hostname: hostname(), Arch: runtime.GOARCH, OS: runtime.GOOS}
	info.IPs, info.MACs = collectAddrs()
	info.OS, info.OSVersion = collectOSVersion()
	return info
}

func hostname() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}

func collectAddrs() (ips, macs []string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagUp == 0 {
			continue
		}
		if len(ifc.HardwareAddr) > 0 {
			macs = append(macs, ifc.HardwareAddr.String())
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				ips = append(ips, v4.String())
			}
		}
	}
	return dedup(ips), dedup(macs)
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// collectOSVersion 获取操作系统名称与版本：Linux 读 /etc/os-release，Windows 仅标记系统名
func collectOSVersion() (name, ver string) {
	if runtime.GOOS == "windows" {
		return "Windows", ""
	}
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				name = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
			if strings.HasPrefix(line, "VERSION_ID=") {
				ver = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
			}
		}
		if name != "" {
			return name, ver
		}
	}
	// 兜底：uname 取系统信息
	if out, err := exec.Command("uname", "-srm").Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) > 0 {
			name = parts[0]
		}
		if len(parts) > 1 {
			ver = strings.Join(parts[1:], " ")
		}
	}
	if name == "" {
		name = runtime.GOOS
	}
	return name, ver
}

// Ensure 确保本机密钥对与自签公证书存在：首次调用生成并持久化，之后复用
func Ensure(db *gorm.DB, nodeUUID, nodeName string) error {
	if err := os.MkdirAll(filepath.Dir(PrivateKeyPath), 0o700); err != nil {
		return fmt.Errorf("创建证书目录失败: %w", err)
	}
	// 已存在本机公证书且私钥文件可用 → 复用
	if self := SelfCert(db); self != nil {
		if _, err := os.Stat(PrivateKeyPath); err != nil {
			// 私钥文件丢失：证书与私钥不匹配，需重新生成
			logger.Warnf("[cert] 私钥文件丢失，重新生成证书（旧公证书将失效）")
			db.Where("node_uuid = ''").Delete(&model.Certificate{})
		} else if !HasNodeInfo(self.CertPEM) {
			// 旧证书不含节点参数（IP/MAC/系统版本）：自动重新生成，上报时自动与对方重新交换
			logger.Warnf("[cert] 当前证书不含节点参数（IP/MAC/系统版本），自动重新生成；新证书将在下次上报时自动完成交换")
			db.Where("node_uuid = ''").Delete(&model.Certificate{})
		} else {
			return nil
		}
	}
	// 生成 SM2 密钥对
	key, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("生成密钥失败: %w", err)
	}
	// 自签公证书（SM2-SM3）
	pemCert, der, err := SelfSign(key, nodeUUID, nodeName)
	if err != nil {
		return err
	}
	// 私钥写本机文件（0600）
	if err := writePrivateKey(key); err != nil {
		return err
	}
	cert, err := smx509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("解析自签证书失败: %w", err)
	}
	rec := model.Certificate{
		NodeUUID:    "",
		Name:        nodeName,
		CertType:    "public",
		CertPEM:     pemCert,
		Fingerprint: FingerprintDER(der),
		NotBefore:   cert.NotBefore,
		NotAfter:    cert.NotAfter,
		Status:      "valid",
	}
	if err := db.Create(&rec).Error; err != nil {
		return fmt.Errorf("保存本机证书失败: %w", err)
	}
	logger.Infof("[cert] 本机 SM2 证书已生成 fingerprint=%s", rec.Fingerprint)
	return nil
}

// SelfCert 返回本机公证书记录（NodeUUID=''）
func SelfCert(db *gorm.DB) *model.Certificate {
	var c model.Certificate
	if err := db.Where("node_uuid = '' AND cert_type = 'public'").Order("id desc").First(&c).Error; err != nil {
		return nil
	}
	return &c
}

// PublicPEM 返回本机公证书 PEM（用于下发给下级）
func PublicPEM(db *gorm.DB) string {
	if c := SelfCert(db); c != nil {
		return c.CertPEM
	}
	return ""
}

// SelfSign 生成自签名 x509 证书（SM2 密钥，SM2-SM3 签名算法）
// Subject: CN=节点名, SerialNumber=节点UUID（解析证书即可取得节点身份）
// 自定义扩展：证书内嵌本机运行参数（IP/MAC/系统版本等）
func SelfSign(key *sm2.PrivateKey, nodeUUID, nodeName string) (pemCert string, der []byte, err error) {
	now := time.Now()
	info := CollectNodeInfo()
	infoJSON, jerr := json.Marshal(info)
	if jerr != nil {
		return "", nil, fmt.Errorf("编码节点参数失败: %w", jerr)
	}
	tpl := &smx509.Certificate{
		SerialNumber:          new(big.Int).SetBytes([]byte(nodeUUID)),
		Subject:               pkixName(nodeName, nodeUUID),
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(certValidity),
		KeyUsage:              smx509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []smx509.ExtKeyUsage{smx509.ExtKeyUsageClientAuth, smx509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		ExtraExtensions: []pkix.Extension{
			{Id: nodeInfoOID, Critical: false, Value: infoJSON},
		},
	}
	der, err = smx509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return "", nil, fmt.Errorf("签发自签证书失败: %w", err)
	}
	pemCert = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return pemCert, der, nil
}

// ParsePEM 解析证书 PEM 基本信息（兼容国密 SM2 与标准证书），用于界面摘要展示（不返回明文）
func ParsePEM(pemCert string) (subject string, notBefore, notAfter time.Time, err error) {
	block, _ := pem.Decode([]byte(pemCert))
	if block == nil || block.Type != "CERTIFICATE" {
		return "", time.Time{}, time.Time{}, fmt.Errorf("无效的 PEM 证书")
	}
	cert, err := smx509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	return cert.Subject.CommonName, cert.NotBefore, cert.NotAfter, nil
}

// PrivateKeyFingerprint 返回本机私钥文件 SHA-256 指纹（私有证书标识，仅本机可见）
func PrivateKeyFingerprint() string {
	b, err := os.ReadFile(PrivateKeyPath)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ExportPrivateKey 返回本机私钥 PEM 内容（私有证书导出：用于备份/离线迁移；
// 导出即代表持有节点身份，调用方须确认操作者权限，前端需二次确认）
func ExportPrivateKey() (string, error) {
	b, err := os.ReadFile(PrivateKeyPath)
	if err != nil {
		return "", fmt.Errorf("本机私有证书未生成")
	}
	return string(b), nil
}

// RemovePrivateKey 删除本机私钥文件（用于规则自愈：非法状态下的历史残留私钥）
func RemovePrivateKey() error {
	if err := os.Remove(PrivateKeyPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HasNodeInfo 判断证书是否包含节点参数扩展（IP/MAC/系统版本）
func HasNodeInfo(pemCert string) bool {
	block, _ := pem.Decode([]byte(pemCert))
	if block == nil {
		return false
	}
	cert, err := smx509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(nodeInfoOID) {
			return true
		}
	}
	return false
}

// ParseNodeInfo 从证书 PEM 解析节点参数（IP/MAC/系统版本等）；无扩展或解析失败返回零值
func ParseNodeInfo(pemCert string) NodeInfo {
	var info NodeInfo
	block, _ := pem.Decode([]byte(pemCert))
	if block == nil {
		return info
	}
	cert, err := smx509.ParseCertificate(block.Bytes)
	if err != nil {
		return info
	}
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(nodeInfoOID) {
			_ = json.Unmarshal(ext.Value, &info)
			break
		}
	}
	return info
}

func pkixName(name, uuid string) pkix.Name {
	return pkix.Name{CommonName: name, SerialNumber: uuid}
}

// FingerprintPEM 计算 PEM 证书的 SM3 指纹（DER 部分）
func FingerprintPEM(pemCert string) (string, error) {
	block, _ := pem.Decode([]byte(pemCert))
	if block == nil {
		return "", fmt.Errorf("证书 PEM 解析失败")
	}
	return FingerprintDER(block.Bytes), nil
}

// FingerprintDER 计算 DER 证书的 SM3 指纹（特征标签）
func FingerprintDER(der []byte) string {
	sum := sm3.Sum(der)
	return hex.EncodeToString(sum[:])
}

// Validate 解析并校验公证书：格式、自签名（SM2）、有效期
func Validate(pemCert string) (*smx509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemCert))
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("证书格式无效")
	}
	cert, err := smx509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("证书解析失败: %v", err)
	}
	// 自签名校验：签名者公钥必须为证书自身公钥
	pool := smx509.NewCertPool()
	pool.AddCert(cert)
	if _, err := cert.Verify(smx509.VerifyOptions{Roots: pool, KeyUsages: []smx509.ExtKeyUsage{smx509.ExtKeyUsageAny}}); err != nil {
		return nil, fmt.Errorf("证书自签名校验失败: %v", err)
	}
	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return nil, fmt.Errorf("证书不在有效期内（%s ~ %s）", cert.NotBefore.Format("2006-01-02"), cert.NotAfter.Format("2006-01-02"))
	}
	return cert, nil
}

// SavePeer 保存/更新对方节点公证书（node_uuid 唯一），并即时校验状态
func SavePeer(db *gorm.DB, nodeUUID, name, pemCert string) error {
	fp, err := FingerprintPEM(pemCert)
	if err != nil {
		return err
	}
	cert, err := Validate(pemCert)
	if err != nil {
		return err
	}
	now := time.Now()
	status := "valid"
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		status = "expired"
	}
	var rec model.Certificate
	err = db.Where("node_uuid = ?", nodeUUID).First(&rec).Error
	upd := model.Certificate{
		Name:        name, CertType: "public", CertPEM: pemCert,
		Fingerprint: fp, NotBefore: cert.NotBefore, NotAfter: cert.NotAfter,
		Status: status, LastVerify: &now,
	}
	if err != nil {
		rec = model.Certificate{NodeUUID: nodeUUID}
		rec.Name = upd.Name
		rec.CertType = upd.CertType
		rec.CertPEM = upd.CertPEM
		rec.Fingerprint = upd.Fingerprint
		rec.NotBefore = upd.NotBefore
		rec.NotAfter = upd.NotAfter
		rec.Status = upd.Status
		rec.LastVerify = upd.LastVerify
		return db.Create(&rec).Error
	}
	return db.Model(&rec).Updates(upd).Error
}

// VerifyLoop 8 小时周期认证：校验本机与全部已登记对方证书的有效期、自签名与指纹一致性，
// 更新 status/last_verify；返回问题摘要（空 = 全部正常）
func VerifyLoop(db *gorm.DB) string {
	var certs []model.Certificate
	db.Find(&certs)
	now := time.Now()
	problems := []string{}
	for i := range certs {
		c := &certs[i]
		if _, err := Validate(c.CertPEM); err != nil {
			if c.Status != "expired" && c.Status != "invalid" {
				c.Status = "invalid"
				db.Model(c).Updates(map[string]any{"status": "invalid", "last_verify": now})
			}
			problems = append(problems, fmt.Sprintf("%s(%s): %v", c.Name, shortFP(c.Fingerprint), err))
			continue
		}
		// 指纹一致性
		if fp, err := FingerprintPEM(c.CertPEM); err == nil && fp != c.Fingerprint {
			c.Status = "invalid"
			db.Model(c).Updates(map[string]any{"status": "invalid", "last_verify": now})
			problems = append(problems, fmt.Sprintf("%s: 指纹不一致", c.Name))
			continue
		}
		if now.After(c.NotAfter) {
			c.Status = "expired"
		} else {
			c.Status = "valid"
		}
		db.Model(c).Updates(map[string]any{"status": c.Status, "last_verify": now})
	}
	if len(problems) > 0 {
		return "证书认证异常: " + strings.Join(problems, "; ")
	}
	return ""
}

// SignChallenge 用本机私钥对挑战数据签名（SM3 摘要 + SM2 标准 ASN.1 签名，双向认证握手用）
func SignChallenge(challenge []byte) ([]byte, error) {
	key, err := loadPrivateKey()
	if err != nil {
		return nil, err
	}
	return sm2Sign(key, challenge)
}

// VerifyChallenge 用对方公证书验证挑战签名（与 SignChallenge 同一 SM3+SM2 规则）
func VerifyChallenge(pemCert string, challenge, signature []byte) error {
	cert, err := Validate(pemCert)
	if err != nil {
		return err
	}
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("证书公钥类型不支持")
	}
	digest := sm3.Sum(challenge)
	if !sm2.VerifyASN1(pub, digest[:], signature) {
		return fmt.Errorf("签名验证失败")
	}
	return nil
}

func sm2Sign(key *sm2.PrivateKey, data []byte) ([]byte, error) {
	digest := sm3.Sum(data)
	return sm2.SignASN1(rand.Reader, key, digest[:], nil)
}

func writePrivateKey(key *sm2.PrivateKey) error {
	der, err := smx509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("编码私钥失败: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return os.WriteFile(PrivateKeyPath, pemBytes, 0o600)
}

func loadPrivateKey() (*sm2.PrivateKey, error) {
	b, err := os.ReadFile(PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取本机私钥失败: %w", err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("本机私钥解析失败")
	}
	key, err := smx509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析本机私钥失败: %w", err)
	}
	k, ok := key.(*sm2.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("本机私钥不是 SM2 私钥")
	}
	return k, nil
}

func shortFP(fp string) string {
	if len(fp) >= 16 {
		return fp[:8] + "…" + fp[len(fp)-8:]
	}
	return fp
}
