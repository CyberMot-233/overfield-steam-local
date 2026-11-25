package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"
	"gucooing/lolo/db"
	"gucooing/lolo/game/model"
	"io"
	"net/http"
	"strconv"
	"time"
)

func localWeaponUID(c *gin.Context) (uint32, bool) {
	uid, err := strconv.ParseUint(c.Param("uid"), 10, 32)
	if err != nil || uid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效账号"})
		return 0, false
	}
	return uint32(uid), true
}
func localWeaponReply(c *gin.Context, value any, err error) {
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, model.ErrLocalWeaponConflict) {
			status = http.StatusConflict
		}
		if errors.Is(err, ErrPlayerOffline) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", value.([]byte))
}
func (g *Game) registerLocalWeapons() {
	g.router.GET("/local-admin/client-state/:uid", localAdminAuth, func(c *gin.Context) {
		uid, ok := localWeaponUID(c)
		if !ok {
			return
		}
		value, err := g.InvokePlayerFunc(uid, func(p *model.Player) (any, error) {
			rows := []any{}
			for _, w := range p.GetItemModel().GetItemWeaponMap() {
				if !w.LocalEdited {
					continue
				}
				b, e := protojson.Marshal(w.WeaponInstance())
				if e != nil {
					return nil, e
				}
				rows = append(rows, gin.H{"instanceId": w.InstanceId, "weaponId": w.WeaponId, "revision": model.LocalWeaponRevision(w), "data": json.RawMessage(b), "stats": w.LocalStats})
			}
			return json.Marshal(gin.H{"uid": uid, "weapons": rows, "unlocks": gin.H{"actions": p.LocalUnlockedActions(), "abilities": p.LocalAbilityList()}})
		}, 5*time.Second)
		localWeaponReply(c, value, err)
	})
	g.router.GET("/local-admin/weapons/:uid", localAdminAuth, func(c *gin.Context) {
		uid, ok := localWeaponUID(c)
		if !ok {
			return
		}
		value, err := g.InvokePlayerFunc(uid, func(p *model.Player) (any, error) {
			return json.Marshal(gin.H{"weapons": p.LocalWeaponList(), "online": true, "uid": uid})
		}, 5*time.Second)
		localWeaponReply(c, value, err)
	})
	g.router.POST("/local-admin/weapon/:uid", localAdminAuth, func(c *gin.Context) {
		uid, ok := localWeaponUID(c)
		if !ok {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		var req model.LocalWeaponEdit
		if err := decoder.Decode(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "武器请求格式错误：" + err.Error()})
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求包含多余数据"})
			return
		}
		// No application timeout: once accepted, a queued write must not be reported
		// as failed merely because it executes later. The UI can always reload it.
		value, err := g.InvokePlayerFunc(uid, func(p *model.Player) (any, error) {
			old := p.GetItemModel().GetItemWeaponInfo(req.InstanceID)
			if old == nil {
				return nil, fmt.Errorf("武器实例不存在")
			}
			before := *old
			w, err := p.LocalEditWeapon(req)
			if err != nil {
				return nil, err
			}
			bin, err := json.Marshal(p)
			if err != nil {
				*w = before
				return nil, err
			}
			// Use the same persistence shard as autosaves so an older queued snapshot
			// cannot overwrite this edit. Wait for the actual SQLite result.
			done := make(chan error, 1)
			db.Persist(p.UserId, func() error { e := db.SaveOFGameBin(uid, bin); done <- e; return e })
			if err = <-done; err != nil {
				*w = before
				return nil, fmt.Errorf("武器未保存：%w", err)
			}
			return json.Marshal(gin.H{"weapon": model.LocalViewWeapon(w), "online": true, "saved": true, "uid": uid})
		}, 0)
		localWeaponReply(c, value, err)
	})
}
