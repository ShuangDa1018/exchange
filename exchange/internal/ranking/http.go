package ranking

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func NewHTTP(board *Board) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "ranking"})
	})
	r.GET("/api/leaderboard", func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		sortBy := c.DefaultQuery("sort", "volume")
		if sortBy != "equity" {
			sortBy = "volume"
		}
		ctx := c.Request.Context()
		rows, lastPrice, err := board.Top(ctx, sortBy, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []Row{}
		}
		c.JSON(http.StatusOK, gin.H{
			"sort":       sortBy,
			"last_price": lastPrice,
			"updated_at": time.Now().UTC(),
			"rows":       rows,
		})
	})
	return r
}
