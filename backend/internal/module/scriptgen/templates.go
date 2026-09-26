// Package scriptgen 模板库（全部在代码中制作，启动时同步到数据库，前后端不可配置）
package scriptgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"netops/internal/model"
)

// Field 表单字段定义（前端动态渲染）
type Field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // text/number/ip/network/select/list/password
	Required    bool     `json:"required"`
	Default     string   `json:"default"`
	Options     []string `json:"options"`
	Placeholder string   `json:"placeholder"`
}

// Template 代码内模板定义
type Template struct {
	Code        string
	Vendor      string
	DeviceType  string
	Category    string
	Name        string
	Description string
	Fields      []Field
	Render      func(params map[string]string, vendor string) []string
}

// registry 模板注册表（代码即事实来源）
var registry []Template

// Register 注册模板
func Register(t Template) { registry = append(registry, t) }

// Templates 获取全部模板
func Templates() []Template { return registry }

// SeedTemplates 将代码模板同步写入数据库（新增或更新已有模板）
func SeedTemplates(db *gorm.DB) {
	for _, t := range registry {
		schema, _ := json.Marshal(t.Fields)
		var cnt int64
		db.Model(&model.ScriptTemplate{}).Where("code = ?", t.Code).Count(&cnt)
		if cnt == 0 {
			db.Create(&model.ScriptTemplate{
				Code: t.Code, Vendor: t.Vendor, DeviceType: t.DeviceType,
				Category: t.Category, Name: t.Name, Description: t.Description,
				SchemaJSON: string(schema), Enabled: true,
			})
		} else {
			db.Model(&model.ScriptTemplate{}).Where("code = ?", t.Code).Updates(map[string]any{
				"schema_json": string(schema),
				"name":        t.Name,
				"description": t.Description,
			})
		}
	}
}

// FindByCode 按代码查找模板
func FindByCode(code string) (*Template, bool) {
	for i := range registry {
		if registry[i].Code == code {
			return &registry[i], true
		}
	}
	return nil, false
}

// RenderByCode 渲染脚本
func RenderByCode(code string, params map[string]string, vendor string) (string, error) {
	t, ok := FindByCode(code)
	if !ok {
		return "", fmt.Errorf("模板不存在: %s", code)
	}
	lines := t.Render(params, vendor)
	return strings.Join(lines, "\n"), nil
}

// CategoryTree 分类树（厂商 → 设备类型 → 分类）
func CategoryTree() []map[string]any {
	type node struct {
		vendor string
		types  map[string][]string
	}
	order := []string{}
	nodes := map[string]*node{}
	for _, t := range registry {
		n, ok := nodes[t.Vendor]
		if !ok {
			n = &node{vendor: t.Vendor, types: map[string][]string{}}
			nodes[t.Vendor] = n
			order = append(order, t.Vendor)
		}
		if !contains(n.types[t.DeviceType], t.Category) {
			n.types[t.DeviceType] = append(n.types[t.DeviceType], t.Category)
		}
	}
	out := make([]map[string]any, 0, len(order))
	for _, v := range order {
		n := nodes[v]
		tree := map[string]any{"vendor": n.vendor, "types": []map[string]any{}}
		typeOrder := []string{"router", "switch", "ac"}
		var types []map[string]any
		for _, dt := range typeOrder {
			if cats, ok := n.types[dt]; ok {
				types = append(types, map[string]any{"device_type": dt, "categories": cats})
			}
		}
		tree["types"] = types
		out = append(out, tree)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ---- 通用渲染工具 ----

func p(params map[string]string, key, def string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return def
}

func sysView(vendor string) string {
	if strings.EqualFold(vendor, "h3c") {
		return "system-view"
	}
	return "sys"
}

func vlanFormat(v string) string {
	// 支持 10、1-10、1,3,5-10
	return v
}

// ---- 初始化全部模板 ----

func init() {
	// ========== 路由器 ==========
	regRouterACL()
	regRouterNAT()
	regRouterOSPF()
	regRouterBGP()
	regRouterISIS()
	regRouterMPLS()
	regRouterMPLSVPN()
	regRouterStatic()
	regRouterPolicyRoute()
	regRouterRoutingPolicy()
	regRouterAuth()
	regRouterVRRP()
	// ========== 交换机 ==========
	regSwitchVLAN()
	regSwitchVLANIF()
	regSwitchACL()
	regSwitchNAT()
	regSwitchOSPF()
	regSwitchBGP()
	regSwitchISIS()
	regSwitchMPLS()
	regSwitchMPLSVPN()
	regSwitchStatic()
	regSwitchPolicyRoute()
	regSwitchRoutingPolicy()
	regSwitchInterface()
	regSwitchAuth()
	regSwitchVRRP()
	regSwitchDHCP()
	// ========== AC 控制器 ==========
	regACCommon()
	regACWireless()
	// ========== 复制为 H3C 版本 ==========
	duplicateForH3C()
}

// duplicateForH3C 将所有华为模板复制为 H3C 版本（H3C CLI 与华为高度兼容）
func duplicateForH3C() {
	huawei := make([]Template, len(registry))
	copy(huawei, registry)
	for _, t := range huawei {
		if t.Vendor != "huawei" {
			continue
		}
		ht := t
		ht.Code = "h3c_" + t.Code
		ht.Vendor = "h3c"
		Register(ht)
	}
}

// field 构造字段
func field(key, label, typ string, required bool) Field {
	return Field{Key: key, Label: label, Type: typ, Required: required}
}

func fieldOpt(key, label, typ string, required bool, opts ...string) Field {
	return Field{Key: key, Label: label, Type: typ, Required: required, Options: opts}
}

func fieldWithPlaceholder(key, label, typ string, required bool, placeholder string) Field {
	return Field{Key: key, Label: label, Type: typ, Required: required, Placeholder: placeholder}
}

