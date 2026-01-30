package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims 定义 Token 的 Payload 结构
type JWTClaims struct {
	Address              string `json:"address"` // 用户以太坊地址
	Role                 string `json:"role"`    // 用户角色，如 "admin"、"user"
	jwt.RegisteredClaims        // 包含标准的注册声明
}

// JWTMiddleware 定义 JWT 验证中间件
func JWTMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 Authorization Header 提取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// 2. 检查 Bearer 格式
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be Bearer token"})
			return
		}

		tokenString := parts[1]

		// 3. 解析并验证 Token
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// 4. 将地址和角色注入 Context
		// 后续 Controller 可以通过 c.GetString("user_address") 获取，确保逻辑安全
		c.Set("user_address", claims.Address)
		c.Set("user_role", claims.Role)

		c.Next()
	}
}

// GenerateToken 用于在 Login 成功后生成 Token
func GenerateToken(address, role, secret string, duration time.Duration) (string, error) {
	claims := JWTClaims{
		Address: address,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)), // 设置过期时间 通常为 1 小时
			IssuedAt:  jwt.NewNumericDate(time.Now()),               // 设置签发时间
			NotBefore: jwt.NewNumericDate(time.Now()),               // 设置生效时间
			Issuer:    "polyagent-api",                              // 签发者
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// RoleGuard 用于特定角色的权限控制中间件
func RoleGuard(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role.(string) != requiredRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Permission denied: " + requiredRole + " role required"})
			return
		}
		c.Next()
	}
}
