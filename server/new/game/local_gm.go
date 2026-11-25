package game

import (
	"fmt"
	"gucooing/lolo/config"
	"gucooing/lolo/db"
	"gucooing/lolo/game/model"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/pkg/alg"
	"gucooing/lolo/pkg/log"
	"gucooing/lolo/protocol/proto"
	"strconv"
	"strings"
)

// The upstream GmCode handler is a placeholder. Implement local save commands
// and return explicit failure for unsupported commands.
func (g *Game) LocalGM(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.GmCodeReq)
	rsp := &proto.GmCodeRsp{Status: proto.StatusCode_StatusCode_Fail}
	defer func() {
		if e := recover(); e != nil {
			rsp.Status = proto.StatusCode_StatusCode_Fail
			rsp.Result = fmt.Sprint("GM error: ", e)
		}
		g.send(s, msg.PacketId, rsp)
	}()
	if config.GetGateWay().GetOuterIp() != "127.0.0.1" {
		rsp.Result = "Local GM is available in solo mode only"
		return
	}
	if len(req.Param) > 2048 {
		rsp.Result = "Command too long"
		return
	}
	log.Game.Infof("LocalGM UID:%v command:%s", s.UserId, req.Param)
	args := strings.Fields(strings.NewReplacer(",", " ", "|", " ", "#", " ").Replace(req.Param))
	if len(args) == 0 {
		rsp.Result = "Empty command"
		return
	}
	uintArg := func(i int) (uint32, error) {
		if i >= len(args) {
			return 0, fmt.Errorf("Missing argument %d", i)
		}
		v, e := strconv.ParseUint(args[i], 10, 32)
		return uint32(v), e
	}
	grant := func(id uint32, count int64) bool { return s.LocalGrantItem(id, count, false) }
	changed := false
	switch strings.ToLower(args[0]) {
	case "add_item", "add_item_temp_pack", "weaponadd", "armoradd", "posteradd", "spellcardadd", "change_item":
		id, e := uintArg(1)
		if e != nil {
			rsp.Result = e.Error()
			return
		}
		count := int64(1)
		if len(args) > 2 {
			count, e = strconv.ParseInt(args[2], 10, 64)
			if e != nil || count < 1 || count > 999999999 {
				rsp.Result = "Count must be 1..999999999"
				return
			}
		}
		if model.LocalUniqueItem(id) {
			count = 1
		}
		if strings.EqualFold(args[0], "change_item") {
			if old := s.GetItemModel().GetItemBaseInfo(id); old != nil {
				old.Num = count
				changed = true
			}
		}
		if !changed {
			changed = grant(id, count)
		}
		if !changed {
			rsp.Result = "Item is not supported by this model"
			return
		}
		rsp.RewardInfos = []*proto.GmRewardInfo{{ItemId: id, Num: count}}
		rsp.Result = "Item updated"
	case "gm_get_item_num":
		id, e := uintArg(1)
		if e != nil {
			rsp.Result = e.Error()
			return
		}
		num := int64(0)
		if it := s.GetItemModel().GetItemBaseInfo(id); it != nil {
			num = it.Num
		}
		rsp.Result = fmt.Sprintf("%d = %d", id, num)
	case "player_level":
		level, e := uintArg(1)
		if e != nil || level < 1 || level > 50 {
			rsp.Result = "player_level 1..50"
			return
		}
		e = db.UpGameBasic(s.UserId, func(b *db.OFGameBasic) bool { b.Level = level; b.Exp = 0; return true })
		if e != nil {
			rsp.Result = e.Error()
			return
		}
		g.send(s, 0, &proto.PlayerLevelExpNotice{Status: proto.StatusCode_StatusCode_Ok, Level: level, Exp: 0})
		rsp.Result = "Player level updated"
	case "charlv":
		id, e := uintArg(1)
		if e != nil {
			rsp.Result = e.Error()
			return
		}
		level, e := uintArg(2)
		if e != nil || level < 1 || level > 100 {
			rsp.Result = "charLv characterId level(1..100)"
			return
		}
		c := s.GetCharacterModel().GetCharacterInfo(id)
		if c == nil {
			rsp.Result = "Character not owned"
			return
		}
		conf := gdconf.GetCharacterAll(id)
		c.BreakLevel = uint32(len(conf.LevelRules))
		c.UpMaxLevel()
		if level > c.MaxLevel {
			level = c.MaxLevel
		}
		c.Level = level
		c.Exp = 0
		changed = true
		rsp.Result = "Character updated; reconnect to refresh character UI"
	case "addallgmitem", "add_gm_count_item":
		n := int64(9999999)
		if len(args) > 1 {
			v, e := strconv.ParseInt(args[1], 10, 64)
			if e != nil || v < 1 || v > 999999999 {
				rsp.Result = "Count must be 1..999999999"
				return
			}
			n = v
		}
		for _, it := range gdconf.GetAllItemConfigure() {
			if model.LocalItemAvailable(uint32(it.ID)) {
				s.LocalGrantItem(uint32(it.ID), n, true)
			}
		}
		changed = true
		rsp.Result = "All model-supported items granted; reconnect to refresh characters"
	case "quest_complete", "forcequestcomplete":
		if len(args) > 1 {
			id, e := uintArg(1)
			if e != nil {
				rsp.Result = e.Error()
				return
			}
			q := model.CompletedLocalQuest(id)
			if q == nil {
				rsp.Result = "Unknown quest ID"
				return
			}
			s.LocalProgress().Quests[id] = q
		} else {
			s.LocalCompleteStory()
		}
		changed = true
		rsp.Result = "Quest records saved; reconnect to refresh"
	case "unlockscene", "unlockallareaandaddflagrecord":
		s.LocalUnlockMaps()
		changed = true
		rsp.Result = "Available local maps and areas unlocked; reconnect to refresh"
	default:
		rsp.Result = "Not implemented by Lolo: " + args[0] + ". Use the companion save editor or a supported client-side command."
		return
	}
	rsp.Status = proto.StatusCode_StatusCode_Ok
	if changed {
		s.SetActiveTime()
		if e := s.SavePlayer(); e != nil {
			rsp.Status = proto.StatusCode_StatusCode_Fail
			rsp.Result = e.Error()
			return
		}
		g.AllPackNotice(s)
	}
}
