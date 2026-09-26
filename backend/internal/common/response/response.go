// Package response 统一 API 响应结构
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body 统一响应体
type Body struct {
	Code    int    `json:"code"`    // 0=成功，非0=业务错误
	Message string `json:"message"` // 提示信息
	Data    any    `json:"data,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

// 业务错误码
const (
	CodeOK                = 0
	CodeBadRequest        = 40000
	CodeUnauthorized      = 40100 // 未登录或凭证失效
	CodeForbidden         = 40300 // 无权限
	CodeMustChangePwd     = 40310 // 必须修改初始密码
	CodeNotFound          = 40400
	CodeConflict          = 40900
	CodeServerError       = 50000
	CodeCryptoError       = 40001 // 国密加解密失败
	CodeNotInitialized    = 41000 // 系统未初始化
	CodeWeakPassword      = 42201 // 密码强度不足
	CodeValidationFailed  = 42200
)

// OK 成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Message: "ok", Data: data})
}

// OKMsg 成功响应（自定义提示）
func OKMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Message: msg, Data: data})
}

// Fail 失败响应
func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.JSON(httpStatus, Body{Code: code, Message: msg})
}

// Err 服务端错误
func Err(c *gin.Context, err error) {
	Fail(c, http.StatusInternalServerError, CodeServerError, err.Error())
}

// Bad 参数错误
func Bad(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, CodeBadRequest, msg)
}

// Unauthorized 未授权
func Unauthorized(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, CodeUnauthorized, msg)
}

// Forbidden 禁止访问
func Forbidden(c *gin.Context, msg string) {
	Fail(c, http.StatusForbidden, CodeForbidden, msg)
}

// MustChangePwd 强制改密
func MustChangePwd(c *gin.Context) {
	Fail(c, http.StatusForbidden, CodeMustChangePwd, "首次登录或密码已过期，请修改密码")
}

// NotFound 资源不存在
func NotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, CodeNotFound, msg)
}
