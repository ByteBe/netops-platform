package middleware

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义声明
type Claims struct {
	UserID        uint   `json:"uid"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	NodeScope     string `json:"ns"`
	MustChangePwd bool   `json:"mcp"`
	jwt.RegisteredClaims
}

// GenerateToken 签发 JWT
func GenerateToken(userID uint, username, role, nodeScope string, mustChange bool, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:        userID,
		Username:      username,
		Role:          role,
		NodeScope:     nodeScope,
		MustChangePwd: mustChange,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "netops",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken 解析并校验 JWT
func ParseToken(token, secret string) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非法签名算法")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("token 无效或已过期")
	}
	return claims, nil
}
