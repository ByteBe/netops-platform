// Package scriptgen 脚本生成器模块
// 模板全部在代码中制作（templates.go），启动时同步写入数据库，后端与前端不可配置（只读展示）
package scriptgen

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	// 模板列表（支持厂商/设备类型/功能分类过滤）
	g.GET("/templates", func(c *gin.Context) {
		vendor := c.Query("vendor")
		dt := c.Query("device_type")
		cat := c.Query("category")
		q := a.DB.Model(&model.ScriptTemplate{})
		if vendor != "" {
			q = q.Where("vendor = ?", vendor)
		}
		if dt != "" {
			q = q.Where("device_type = ?", dt)
		}
		if cat != "" {
			q = q.Where("category = ?", cat)
		}
		var list []model.ScriptTemplate
		q.Order("device_type asc, category asc, id asc").Find(&list)
		// 附加表单Schema（前端动态渲染）
		out := make([]gin.H, 0, len(list))
		for _, t := range list {
			out = append(out, gin.H{
				"id": t.ID, "code": t.Code, "vendor": t.Vendor, "device_type": t.DeviceType,
				"category": t.Category, "name": t.Name, "description": t.Description,
				"enabled": t.Enabled, "schema": schemaOf(t.SchemaJSON),
			})
		}
		response.OK(c, out)
	})

	// 分类树（按厂商+设备类型）
	g.GET("/categories", func(c *gin.Context) {
		response.OK(c, CategoryTree())
	})

	// 模板详情
	g.GET("/templates/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var t model.ScriptTemplate
		if err := a.DB.First(&t, id).Error; err != nil {
			response.NotFound(c, "模板不存在")
			return
		}
		response.OK(c, gin.H{
			"id": t.ID, "code": t.Code, "vendor": t.Vendor, "device_type": t.DeviceType,
			"category": t.Category, "name": t.Name, "description": t.Description,
			"enabled": t.Enabled, "schema": schemaOf(t.SchemaJSON),
		})
	})

	// 生成脚本
	g.POST("/generate", func(c *gin.Context) {
		var req struct {
			TemplateID uint              `json:"template_id"`
			Params     map[string]string `json:"params"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		var t model.ScriptTemplate
		if err := a.DB.First(&t, req.TemplateID).Error; err != nil {
			response.NotFound(c, "模板不存在")
			return
		}
		script, err := RenderByCode(t.Code, req.Params, t.Vendor)
		if err != nil {
			response.Bad(c, "生成失败: "+err.Error())
			return
		}
		// 校验必填参数
		if err := validateParams(schemaOf(t.SchemaJSON), req.Params); err != nil {
			response.Fail(c, 422, response.CodeValidationFailed, err.Error())
			return
		}
		username, _ := c.Get(middleware.KeyUsername)
		paramsJSON, _ := json.Marshal(req.Params)
		a.DB.Create(&model.ScriptHistory{
			TemplateID: t.ID, TemplateName: t.Name, Vendor: t.Vendor,
			DeviceType: t.DeviceType, ParamsJSON: string(paramsJSON),
			Script: script, Creator: username.(string),
		})
		response.OK(c, gin.H{"script": script})
	})

	// 生成历史
	g.GET("/history", func(c *gin.Context) {
		var list []model.ScriptHistory
		a.DB.Order("id desc").Limit(200).Find(&list)
		response.OK(c, list)
	})

	// 删除历史
	g.DELETE("/history/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.ScriptHistory{}, id)
		response.OK(c, gin.H{"ok": true})
	})
}

func schemaOf(s string) []Field {
	var fields []Field
	_ = json.Unmarshal([]byte(s), &fields)
	return fields
}

func validateParams(fields []Field, params map[string]string) error {
	for _, f := range fields {
		if f.Required {
			if v, ok := params[f.Key]; !ok || v == "" {
				return validationErr("必填参数缺失: " + f.Label)
			}
		}
	}
	return nil
}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

func validationErr(msg string) error { return &validationError{msg: msg} }

// init 自动注册路由
func init() {
	modreg.RegisterProtected("scriptgen", RegisterProtected)
}
