// Package auth：JWT 登录/注册（用户落 Postgres；密码 bcrypt）
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/demo/exchange/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const ContextUserKey = "user_id"

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// UserStore 用户持久化（生产=Postgres；测试可换内存实现）
type UserStore interface {
	CreateUser(ctx context.Context, userID, passwordHash string) error
	GetUserPassword(ctx context.Context, userID string) (string, error)
	EnsureUser(ctx context.Context, userID, passwordHash string) error
}

type Service struct {
	secret []byte
	ttl    time.Duration
	users  UserStore
}

func New(secret string, ttlHours int, users UserStore) *Service {
	if secret == "" {
		secret = "exchange-demo-secret-change-me"
	}
	if ttlHours <= 0 {
		ttlHours = 24
	}
	return &Service{
		secret: []byte(secret),
		ttl:    time.Duration(ttlHours) * time.Hour,
		users:  users,
	}
}

// SeedDemoUsers 写入演示账号（已存在则跳过）
func (s *Service) SeedDemoUsers(ctx context.Context) error {
	for _, u := range []string{"alice", "bob"} {
		hash, err := bcrypt.GenerateFromPassword([]byte(u), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := s.users.EnsureUser(ctx, u, string(hash)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) issue(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "exchange-demo",
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.secret)
}

func (s *Service) Login(userID, password string) (token string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	hash, err := s.users.GetUserPassword(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return "", fmt.Errorf("用户名或密码错误")
		}
		return "", fmt.Errorf("查询用户失败: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", fmt.Errorf("用户名或密码错误")
	}
	return s.issue(userID)
}

// Register 注册新用户（写 users 表）并签发 JWT
func (s *Service) Register(userID, password string) (token string, err error) {
	userID = strings.TrimSpace(userID)
	if userID == "" || password == "" {
		return "", fmt.Errorf("需要 user_id 和 password")
	}
	if len(userID) < 2 || len(userID) > 32 {
		return "", fmt.Errorf("用户名长度需 2–32")
	}
	if len(password) < 3 {
		return "", fmt.Errorf("密码至少 3 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("哈希密码失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.users.CreateUser(ctx, userID, string(hash)); err != nil {
		if errors.Is(err, store.ErrUserExists) {
			return "", fmt.Errorf("用户名已存在")
		}
		return "", fmt.Errorf("注册失败: %w", err)
	}
	return s.issue(userID)
}

func (s *Service) Parse(tokenStr string) (string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return "", errors.New("invalid token")
	}
	return claims.UserID, nil
}

// Middleware：优先 Bearer JWT；否则兼容旧的 X-User-Id（方便静态页）
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := c.GetHeader("Authorization")
		if strings.HasPrefix(authz, "Bearer ") {
			userID, err := s.Parse(strings.TrimPrefix(authz, "Bearer "))
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "无效 token"})
				return
			}
			c.Set(ContextUserKey, userID)
			c.Next()
			return
		}
		if u := c.GetHeader("X-User-Id"); u != "" {
			c.Set(ContextUserKey, u)
			c.Next()
			return
		}
		if u := c.Query("user_id"); u != "" {
			c.Set(ContextUserKey, u)
			c.Next()
			return
		}
		c.Set(ContextUserKey, "alice")
		c.Next()
	}
}

func UserID(c *gin.Context) string {
	if v, ok := c.Get(ContextUserKey); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "alice"
}
