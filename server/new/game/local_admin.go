package game

import (
	"crypto/subtle"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gucooing/lolo/db"
	"gucooing/lolo/game/model"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Local administration is restricted to loopback clients with the profile token.
func (g *Game) registerLocalSnapshot() {
	g.registerLocalWeapons()
	g.router.GET("/local-admin/snapshot/:uid", localAdminAuth, func(c *gin.Context) {
		uid, err := strconv.ParseUint(c.Param("uid"), 10, 32)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		value, err := g.InvokePlayerFunc(uint32(uid), func(p *model.Player) (any, error) {
			basic, ok := db.GetGameBasic(p.UserId)
			if !ok {
				return nil, ErrPlayerOffline
			}
			return json.Marshal(map[string]any{"uid": p.UserId, "player": p, "basic": basic})
		}, 5*time.Second)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", value.([]byte))
	})
}

func localAdminAuth(c *gin.Context) {
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil || !net.ParseIP(ip).IsLoopback() {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	key, err := os.ReadFile("admin-key.txt")
	if err != nil || len(strings.TrimSpace(string(key))) < 32 || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Local-Token")), []byte(strings.TrimSpace(string(key)))) != 1 {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Next()
}
