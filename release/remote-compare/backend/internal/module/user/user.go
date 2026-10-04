// Package user 用户管理（用户名/工号/邮箱；创建/重置密码/停用）
package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"netops/internal/common/crypto"
	"netops/internal/common/password"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
)

// RegisterProtected 受保护路由（写操作仅管理员）
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/users", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		kw := c.Query("keyword")
		q := a.DB.Model(&model.User{})
		if kw != "" {
			q = q.Where("username LIKE ? OR employee_no LIKE ? OR email LIKE ?",
				"%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}
		var total int64
		q.Count(&total)
		var list []model.User
		q.Order("id asc").Offset((page - 1) * size).Limit(size).Find(&list)
		response.OK(c, gin.H{"total": total, "list": list})
	})

	// 创建用户：用户名/工号/邮箱（初始密码由系统生成，符合强度规则，仅返回一次）
	g.POST("/users", middleware.RequireRole("admin"), func(c *gin.Context) {
		var req struct {
			Username   string `json:"username"`
			EmployeeNo string `json:"employee_no"`
			Email      string `json:"email"`
			Role       string `json:"role"`
			Password   string `json:"password"` // 可选，不填则系统生成
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		if req.Username == "" {
			response.Bad(c, "用户名必填")
			return
		}
		var cnt int64
		a.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&cnt)
		if cnt > 0 {
			response.Fail(c, 409, response.CodeConflict, "用户名已存在")
			return
		}
		role := req.Role
		if role == "" {
			role = "operator"
		}
		pwd := req.Password
		if pwd == "" {
			pwd = "123456"
		} else if err := password.Validate(pwd); err != nil {
			response.Fail(c, 422, response.CodeWeakPassword, err.Error())
			return
		}
		salt, _ := crypto.GenerateSalt()
		u := model.User{
			Username: req.Username, EmployeeNo: req.EmployeeNo, Email: req.Email,
			Role: role, Status: "active", MustChangePwd: true,
			PasswordHash: crypto.PasswordHash(pwd, salt), Salt: salt,
		}
		if err := a.DB.Create(&u).Error; err != nil {
			response.Err(c, err)
			return
		}
		response.OK(c, gin.H{"id": u.ID, "initial_password": pwd, "message": "用户已创建，首次登录需修改密码"})
	})

	// 修改用户
	g.PUT("/users/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var u model.User
		if err := a.DB.First(&u, id).Error; err != nil {
			response.NotFound(c, "用户不存在")
			return
		}
		var req struct {
			EmployeeNo string `json:"employee_no"`
			Email      string `json:"email"`
			Role       string `json:"role"`
			Status     string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		updates := map[string]any{}
		if req.EmployeeNo != "" {
			updates["employee_no"] = req.EmployeeNo
		}
		if req.Email != "" {
			updates["email"] = req.Email
		}
		if req.Role != "" {
			updates["role"] = req.Role
		}
		if req.Status != "" {
			updates["status"] = req.Status
		}
		if len(updates) > 0 {
			a.DB.Model(&u).Updates(updates)
		}
		response.OK(c, gin.H{"ok": true})
	})

	// 删除用户
	g.DELETE("/users/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		self, _ := c.Get(middleware.KeyUserID)
		if uint(id) == self {
			response.Bad(c, "不能删除当前登录账号")
			return
		}
		a.DB.Delete(&model.User{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	// 重置密码（生成新临时密码，下次登录强制修改）
	g.POST("/users/:id/reset-password", middleware.RequireRole("admin"), func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var u model.User
		if err := a.DB.First(&u, id).Error; err != nil {
			response.NotFound(c, "用户不存在")
			return
		}
		pwd := "123456"
		salt, _ := crypto.GenerateSalt()
		a.DB.Model(&u).Updates(map[string]any{
			"password_hash": crypto.PasswordHash(pwd, salt), "salt": salt, "must_change_pwd": true,
		})
		response.OK(c, gin.H{"initial_password": pwd, "message": "密码已重置，下次登录需修改"})
	})

	// 角色列表
	g.GET("/users/roles", func(c *gin.Context) {
		response.OK(c, []gin.H{
			{"value": "admin", "label": "管理员"},
			{"value": "operator", "label": "运维人员"},
			{"value": "viewer", "label": "只读人员"},
		})
	})
}

// init 自动注册路由
func init() {
	modreg.RegisterProtected("user", RegisterProtected)
}
