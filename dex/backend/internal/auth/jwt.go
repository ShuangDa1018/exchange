package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/demo/dex/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Service struct {
	secret []byte
	store  *store.Store
}

func New(secret string, st *store.Store) *Service {
	return &Service{secret: []byte(secret), store: st}
}

type claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

func (s *Service) Issue(userID string) (string, error) {
	c := claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "tf-dex",
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(s.secret)
}

func (s *Service) Parse(tok string) (string, error) {
	parsed, err := jwt.ParseWithClaims(tok, &claims{}, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil {
		return "", err
	}
	c, ok := parsed.Claims.(*claims)
	if !ok || !parsed.Valid {
		return "", jwt.ErrTokenInvalidClaims
	}
	return c.UserID, nil
}

func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if strings.HasPrefix(h, "Bearer ") {
			uid, err := s.Parse(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}
			c.Set("uid", uid)
			c.Next()
			return
		}
		if q := c.Query("token"); q != "" {
			uid, err := s.Parse(q)
			if err == nil {
				c.Set("uid", uid)
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login required"})
	}
}

func UID(c *gin.Context) string {
	v, _ := c.Get("uid")
	s, _ := v.(string)
	return s
}
