package distributed

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/logger"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/service/certsvc"
)

var (
	authInterval    = 8 * time.Hour // 双向认证周期
	certWaitTimeout = 3 * time.Minute
	certWaitCheckTk = 30 * time.Second
	// expelledKeyPrefix 证书未导入被断开删除的节点黑名单前缀（需重新上线注册）
	expelledKeyPrefix = "cert_expelled_"
)

// StartCert 启动证书服务：启动每 8 小时周期双向认证与"已注册但证书未在 3 分钟内导入"
// 下级节点的断开清理。本机证书由用户在"系统设置-节点证书"点击"生成本机私有证书"手动生成
// （可独立生成，不依赖是否已导入上级公有证书；证书是必需的，无证书无上报通道）。
func StartCert(a *core.App) {
	go func() {
		// 启动即规则自愈：清除"未导入上级证书却存在本机证书/私钥"的历史残留
		cleanupInvalidSelfCert(a)
		tk := time.NewTicker(authInterval)
		defer tk.Stop()
		verifyAll(a)
		for range tk.C {
			verifyAll(a)
		}
	}()
	go func() {
		tk := time.NewTicker(certWaitCheckTk)
		defer tk.Stop()
		for range tk.C {
			enforceCertWait(a)
		}
	}()
}

// verifyAll 周期认证：校验本机与全部已登记对方证书（有效期/自签名/指纹一致性）
func verifyAll(a *core.App) {
	if selfFP(a) == "" {
		logger.Infof("[cert] 本机私有证书未生成，跳过周期认证（请在系统设置-节点证书点击“生成本机私有证书”）")
		return
	}
	if problem := certsvc.VerifyLoop(a.DB); problem != "" {
		logger.Warnf("[cert] %s", problem)
	} else {
		logger.Infof("[cert] 周期认证完成，全部证书有效")
	}
}

// touchCertWait 维护下级节点证书待导入时限：
// 已登记证书 → 清除等待标记；未登记证书 → 首次起计时（重复上报不重置，保证 3 分钟硬时限）
func touchCertWait(a *core.App, nodeUUID string) {
	var c model.Certificate
	if err := a.DB.Where("node_uuid = ?", nodeUUID).First(&c).Error; err == nil {
		a.DB.Model(&model.Node{}).Where("node_uuid = ?", nodeUUID).Update("cert_wait_at", nil)
		return
	}
	a.DB.Model(&model.Node{}).
		Where("node_uuid = ? AND cert_wait_at IS NULL", nodeUUID).
		Update("cert_wait_at", time.Now())
}

// isExpelled 判断节点是否因证书未导入被断开删除（黑名单）
func isExpelled(a *core.App, nodeUUID string) bool {
	var ss model.SystemSetting
	return a.DB.Where("`key` = ?", expelledKeyPrefix+nodeUUID).First(&ss).Error == nil
}

// clearExpelled 清除节点黑名单（重新注册成功时调用，允许重新上线）
func clearExpelled(a *core.App, nodeUUID string) {
	a.DB.Where("`key` = ?", expelledKeyPrefix+nodeUUID).Delete(&model.SystemSetting{})
}

// enforceCertWait 清理证书未在 3 分钟内导入的下级节点：
// 已注册（nodes 表）但无证书记录、且超过导入时限的节点，断开并删除其全部数据，
// 记入黑名单，下级需重新上线注册
func enforceCertWait(a *core.App) {
	var nodes []model.Node
	a.DB.Where("cert_wait_at IS NOT NULL").Find(&nodes)
	now := time.Now()
	for i := range nodes {
		n := &nodes[i]
		if n.CertWaitAt == nil {
			continue
		}
		var c model.Certificate
		if err := a.DB.Where("node_uuid = ?", n.NodeUUID).First(&c).Error; err == nil {
			// 已导入证书：清除等待标记
			a.DB.Model(n).Update("cert_wait_at", nil)
			continue
		}
		if now.Sub(*n.CertWaitAt) < certWaitTimeout {
			continue
		}
		// 超时未导入：断开删除并记入黑名单（上报将拒绝，直到重新注册）
		logger.Warnf("[cert] 下级节点 %s(%s) 注册后 %.0f 分钟内未导入证书，断开并删除，需重新上线",
			n.Name, n.NodeUUID, now.Sub(*n.CertWaitAt).Minutes())
		setSetting(a, expelledKeyPrefix+n.NodeUUID, now.Format("2006-01-02 15:04:05"))
		a.DB.Where("node_uuid = ?", n.NodeUUID).Delete(&model.MonitorDevice{})
		a.DB.Where("node_uuid = ?", n.NodeUUID).Delete(&model.NodeContainer{})
		a.DB.Where("node_uuid = ?", n.NodeUUID).Delete(&model.NodeLink{})
		a.DB.Where("node_uuid = ?", n.NodeUUID).Delete(&model.Certificate{})
		a.DB.Delete(&model.Node{}, n.ID)
	}
}

// selfFP 返回本机证书指纹（特征标签）
func selfFP(a *core.App) string {
	if c := certsvc.SelfCert(a.DB); c != nil {
		return c.Fingerprint
	}
	return ""
}

// savePeerCert 保存对方公证书（幂等 upsert）
func savePeerCert(a *core.App, nodeUUID, name, pem string) {
	if pem == "" {
		return
	}
	if err := certsvc.SavePeer(a.DB, nodeUUID, name, pem); err != nil {
		logger.Warnf("[cert] 对方证书登记失败 %s: %v", nodeUUID, err)
	}
}

// verifyCertFingerprint 校验上报证书指纹（特征标签）：证书是必须的，没有明文传输的通道——
// 上报必须携带指纹；已登记证书的节点指纹必须匹配（防证书被替换/伪造）
func verifyCertFingerprint(a *core.App, nodeUUID, fp string) bool {
	if fp == "" {
		logger.Warnf("[cert] 节点 %s 上报未携带证书指纹，拒绝（证书必须，无明文通道）", shortFP(nodeUUID))
		return false
	}
	var c model.Certificate
	if err := a.DB.Where("node_uuid = ?", nodeUUID).First(&c).Error; err != nil {
		logger.Warnf("[cert] 节点 %s 携带证书指纹但未登记证书（请上级在 3 分钟内导入其证书）", shortFP(nodeUUID))
		return true
	}
	if c.Fingerprint != fp {
		logger.Warnf("[cert] 节点 %s 证书指纹不匹配（登记 %s，上报 %s）", nodeUUID, shortFP(c.Fingerprint), shortFP(fp))
		return false
	}
	return true
}

func shortFP(fp string) string {
	if len(fp) >= 16 {
		return fp[:8] + "…" + fp[len(fp)-8:]
	}
	return fp
}

// certSummary 返回已保存证书的摘要（是否已配置 + "主题 · 有效期"），不返回明文
func certSummary(a *core.App, key string) (bool, string) {
	pem := getSetting(a, key)
	if pem == "" {
		return false, ""
	}
	if subj, nb, na, err := certsvc.ParsePEM(pem); err == nil {
		return true, fmt.Sprintf("%s · %s ~ %s", subj, nb.Format("2006-01-02"), na.Format("2006-01-02"))
	}
	return true, "已配置（证书信息解析失败）"
}

// cleanupInvalidSelfCert 规则自愈：私有证书是"导入上级公有证书后形成的上报凭证"。
// 下级节点（已配置上级地址）若未导入上级公有证书却存在本机证书/私钥
// （旧版自动生成的历史残留），按规则清除，引导重新走"上传上级证书→生成"流程。
// cleanupInvalidSelfCert 历史规则兼容入口：分支节点可独立生成自己的证书，
// 不依赖是否已导入上级公有证书，因此不再清理任何"未导入上级证书但持有本机证书"的状态。
func cleanupInvalidSelfCert(a *core.App) {}

// registerCertHandlers 证书管理 API（受保护）
func registerCertHandlers(a *core.App, g *gin.RouterGroup) {
	// 本机证书信息（只读，不触发自动生成；未生成时前端引导手动生成）
	g.GET("/cert/self", func(c *gin.Context) {
		cleanupInvalidSelfCert(a)
		cert := certsvc.SelfCert(a.DB)
		keyFP := certsvc.PrivateKeyFingerprint()
		parentSet, parentInfo := certSummary(a, "parent_cert")
		if cert == nil {
			response.OK(c, gin.H{
				"ok": true, "node_uuid": SelfUUID(a), "name": "",
				"fingerprint": "", "not_before": "", "not_after": "",
				"status": "", "cert_pem": "", "algorithm": "SM2(国密)",
				"node_info": nil,
				"is_root": getSetting(a, "parent_url") == "",
				// 私有证书（私钥）：未生成
				"private_key_exists": keyFP != "",
				"private_key_fp":     keyFP,
				// 上级公有证书（已导入）：摘要展示，供生成本机私有证书
				"parent_cert_set":  parentSet,
				"parent_cert_info": parentInfo,
			})
			return
		}
		info := certsvc.ParseNodeInfo(cert.CertPEM)
		response.OK(c, gin.H{
			"ok": true, "node_uuid": SelfUUID(a), "name": cert.Name,
			"fingerprint": cert.Fingerprint, "not_before": cert.NotBefore,
			"not_after": cert.NotAfter, "status": cert.Status,
			"cert_pem": cert.CertPEM, "algorithm": "SM2(国密)",
			"node_info": info,
			"is_root": getSetting(a, "parent_url") == "",
			// 私有证书（私钥）：仅本机存在状态与指纹，绝不返回私钥内容
			"private_key_exists": keyFP != "",
			"private_key_fp":     keyFP,
			// 上级公有证书（已导入）：摘要展示，供生成本机私有证书
			"parent_cert_set":  parentSet,
			"parent_cert_info": parentInfo,
		})
	})
	// 已交换的对方证书列表（含节点参数与关联节点状态）
	g.GET("/cert/peers", func(c *gin.Context) {
		var certs []model.Certificate
		a.DB.Where("node_uuid <> ''").Order("id asc").Find(&certs)
		// 关联节点状态：一次查全部，避免 N+1
		var nodes []model.Node
		a.DB.Find(&nodes)
		nodeMap := map[string]*model.Node{}
		for i := range nodes {
			nodeMap[nodes[i].NodeUUID] = &nodes[i]
		}
		out := []gin.H{}
		for _, ct := range certs {
			info := certsvc.ParseNodeInfo(ct.CertPEM)
			nodeSt, certWait := "", ""
			if n, ok := nodeMap[ct.NodeUUID]; ok {
				nodeSt = n.Status
				if n.CertWaitAt != nil {
					certWait = n.CertWaitAt.Format("2006-01-02 15:04:05")
				}
			}
			out = append(out, gin.H{
				"node_uuid": ct.NodeUUID, "name": ct.Name,
				"fingerprint": ct.Fingerprint, "status": ct.Status,
				"not_after": ct.NotAfter, "last_verify": ct.LastVerify,
				"node_info": info, "node_status": nodeSt, "cert_wait_at": certWait,
			})
		}
		response.OK(c, out)
	})
	// 导入下级证书（PEM）：按节点导入下级单位证书后完成互信；
	// 证书 SerialNumber=节点UUID、CN=节点名；expect_node_uuid 非空时强校验证书必须属于该节点
	g.POST("/cert/import", func(c *gin.Context) {
		var req struct {
			CertPEM        string `json:"cert_pem"`
			ExpectNodeUUID string `json:"expect_node_uuid"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数缺失")
			return
		}
		pemCert := strings.TrimSpace(req.CertPEM)
		if pemCert == "" {
			response.Bad(c, "证书内容为空")
			return
		}
		if len(pemCert) > 512*1024 {
			response.Bad(c, "证书内容过大（限 512KB）")
			return
		}
		cert, err := certsvc.Validate(pemCert)
		if err != nil {
			response.OK(c, gin.H{"ok": false, "error": err.Error()})
			return
		}
		nodeUUID := cert.Subject.SerialNumber
		if nodeUUID == "" {
			response.OK(c, gin.H{"ok": false, "error": "证书未包含节点标识（SerialNumber），无法关联节点"})
			return
		}
		// 每个下级节点各自生成自己的证书：导入目标节点明确时，强校验证书归属，防止导错
		if req.ExpectNodeUUID != "" && nodeUUID != req.ExpectNodeUUID {
			response.OK(c, gin.H{"ok": false, "error": "证书不属于该节点（证书节点标识 " + shortFP(nodeUUID) + "），请导入该节点自己生成的证书"})
			return
		}
		name := cert.Subject.CommonName
		// 与已注册节点匹配：若 nodes 表已有该节点则采用节点名
		var n model.Node
		if a.DB.Where("node_uuid = ?", nodeUUID).First(&n).Error == nil {
			name = n.Name
		}
		if err := certsvc.SavePeer(a.DB, nodeUUID, name, pemCert); err != nil {
			response.OK(c, gin.H{"ok": false, "error": "证书登记失败: " + err.Error()})
			return
		}
		// 导入成功：清除该节点的证书待导入标记
		a.DB.Model(&model.Node{}).Where("node_uuid = ?", nodeUUID).Update("cert_wait_at", nil)
		fp, _ := certsvc.FingerprintPEM(pemCert)
		info := certsvc.ParseNodeInfo(pemCert)
		logger.Infof("[cert] 已导入下级证书 %s(%s) fingerprint=%s", name, nodeUUID, shortFP(fp))
		response.OK(c, gin.H{
			"ok": true, "node_uuid": nodeUUID, "name": name,
			"fingerprint": fp, "not_before": cert.NotBefore, "not_after": cert.NotAfter,
			"status": "valid", "node_info": info,
		})
	})
	// 上传上级公有证书（PEM）：保存后即可生成本机私有证书；仅存摘要，不返回明文
	g.POST("/cert/parent-cert", func(c *gin.Context) {
		var req struct {
			CertPEM string `json:"cert_pem"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数缺失")
			return
		}
		pemCert := strings.TrimSpace(req.CertPEM)
		if pemCert == "" {
			response.Bad(c, "证书内容为空")
			return
		}
		if len(pemCert) > 512*1024 {
			response.Bad(c, "证书内容过大（限 512KB）")
			return
		}
		cert, err := certsvc.Validate(pemCert)
		if err != nil {
			response.OK(c, gin.H{"ok": false, "error": err.Error()})
			return
		}
		setSetting(a, "parent_cert", pemCert)
		fp, _ := certsvc.FingerprintPEM(pemCert)
		logger.Infof("[cert] 已导入上级公有证书 %s fingerprint=%s", cert.Subject.CommonName, shortFP(fp))
		response.OK(c, gin.H{
			"ok": true, "subject": cert.Subject.CommonName,
			"not_before": cert.NotBefore, "not_after": cert.NotAfter,
			"fingerprint": fp,
		})
	})
	// 生成本机私有证书（凭证）：本机证书可独立生成，不依赖是否已导入上级公有证书；
	// 公有证书是上级给下级下发的通信凭证，私有证书是下级给上级传输数据的凭证
	g.POST("/cert/gen-private", func(c *gin.Context) {
		// 已生成则不覆盖（重新生成请用 /cert/rotate）
		if self := certsvc.SelfCert(a.DB); self != nil && certsvc.PrivateKeyFingerprint() != "" {
			response.OK(c, gin.H{"ok": false, "error": "本机私有证书已生成（指纹 " + shortFP(self.Fingerprint) + "），如需重新生成请使用“重新生成证书”"})
			return
		}
		if err := certsvc.Ensure(a.DB, SelfUUID(a), orStr(getSetting(a, "self_node_name"), SelfUUID(a))); err != nil {
			response.Bad(c, "证书生成失败: "+err.Error())
			return
		}
		logger.Infof("[cert] 本机私有证书已生成 fingerprint=%s", shortFP(selfFP(a)))
		response.OK(c, gin.H{"ok": true, "fingerprint": selfFP(a)})
	})
	// 重新生成本机证书（旧证书立即失效，需重新与对方交换）
	g.POST("/cert/rotate", func(c *gin.Context) {
		a.DB.Where("node_uuid = ''").Delete(&model.Certificate{})
		if err := certsvc.Ensure(a.DB, SelfUUID(a), orStr(getSetting(a, "self_node_name"), SelfUUID(a))); err != nil {
			response.Bad(c, "证书生成失败: "+err.Error())
			return
		}
		response.OK(c, gin.H{"ok": true, "fingerprint": selfFP(a)})
	})
	// 导出本机私有证书（私钥 PEM）：用于备份或离线迁移；导出即持有节点身份，前端需二次确认
	g.GET("/cert/self/private-key", func(c *gin.Context) {
		pemKey, err := certsvc.ExportPrivateKey()
		if err != nil {
			response.OK(c, gin.H{"ok": false, "error": err.Error()})
			return
		}
		response.OK(c, gin.H{"ok": true, "private_key_pem": pemKey})
	})
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
