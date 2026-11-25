package game

import (
	"gucooing/lolo/game/model"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/pkg/alg"
	"gucooing/lolo/protocol/proto"
	"strconv"
)

func (g *Game) DailyTaskExchange(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.DailyTaskExchangeReq)
	state := s.LocalDailyTask()
	rsp := &proto.DailyTaskExchangeRsp{Status: proto.StatusCode_StatusCode_BadReq, Rewards: []*proto.ItemDetail{}, TodayConverted: state.TodayConverted, ExchangeTimesLeft: state.ExchangeTimesLeft}
	defer g.send(s, msg.PacketId, rsp)
	if state.ExchangeTimesLeft == 0 {
		return
	}
	cfg := gdconf.GetGlobalConfigConfigure("MoonStone_ChangeReward")
	if cfg == nil || len(cfg.Values) == 0 {
		return
	}
	rewardID, e := strconv.ParseUint(cfg.Values[0], 10, 32)
	if e != nil {
		return
	}
	rewards := gdconf.GetRewardItemPoolByRewardId(uint32(rewardID))
	if len(rewards) == 0 {
		return
	}
	for _, r := range rewards {
		if r.ItemMinCount <= 0 || r.ItemMinCount != r.ItemMaxCount || !model.LocalItemAvailable(uint32(r.ItemID)) {
			return
		}
	}
	rows := gdconf.GetLocalCatalog().Daily.GetDailyTaskReward().GetDatas()
	costs := map[uint32]int64{}
	count := uint32(0)
	for i := state.TodayConverted; i < uint32(len(rows)); i++ {
		row := rows[i]
		id := uint32(row.CostItemID)
		cost := int64(row.CostItemNum)
		item := s.GetItemModel().GetItemBaseInfo(id)
		if cost < 1 || item == nil || item.Num < costs[id]+cost {
			break
		}
		costs[id] += cost
		count++
		if !req.ExchangeAll {
			break
		}
	}
	if count == 0 {
		return
	}
	tx, e := s.GetItemModel().Begin()
	if e != nil {
		return
	}
	defer tx.Rollback()
	for id, cost := range costs {
		tx.DelBaseItem(id, cost)
	}
	if tx.Error != nil {
		return
	}
	tx.Commit()
	for _, r := range rewards {
		item := s.AddAllTypeItem(uint32(r.ItemID), int64(r.ItemMinCount)*int64(count))
		rsp.Rewards = append(rsp.Rewards, item.AddItemDetail())
	}
	s.LocalProgress().DailyConverted += count
	state = s.LocalDailyTask()
	rsp.TodayConverted = state.TodayConverted
	rsp.ExchangeTimesLeft = state.ExchangeTimesLeft
	rsp.Status = proto.StatusCode_StatusCode_Ok
	g.send(s, 0, tx.PackNotice)
	g.PackNoticeByItems(s, rsp.Rewards)
	_ = s.SavePlayer()
}
func (g *Game) ChangeWorldLevel(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.ChangeWorldLevelReq)
	rsp := &proto.ChangeWorldLevelRsp{Status: proto.StatusCode_StatusCode_BadReq, WorldLevel: s.GetSceneModel().GetWorldLevel()}
	defer g.send(s, msg.PacketId, rsp)
	if !gdconf.ValidWorldLevel(req.WorldLevel) {
		return
	}
	s.GetSceneModel().WorldLevel = req.WorldLevel
	rsp.WorldLevel = req.WorldLevel
	rsp.Status = proto.StatusCode_StatusCode_Ok
	_ = s.SavePlayer()
}
func (g *Game) UnlockWorldLevel(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.UnlockWorldLevelReq)
	rsp := &proto.UnlockWorldLevelRsp{Status: proto.StatusCode_StatusCode_BadReq, WorldLevel: s.GetSceneModel().GetWorldLevel()}
	defer g.send(s, msg.PacketId, rsp)
	if !gdconf.ValidWorldLevel(req.WorldLevel) {
		return
	}
	rsp.WorldLevel = req.WorldLevel
	rsp.Status = proto.StatusCode_StatusCode_Ok
}
func (g *Game) OutfitHideSwitch(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.OutfitHideSwitchReq)
	rsp := &proto.OutfitHideSwitchRsp{Status: proto.StatusCode_StatusCode_BadReq, CharId: req.CharId, HideInfo: req.GetHideInfo()}
	defer g.send(s, msg.PacketId, rsp)
	c := s.GetCharacterModel().GetCharacterInfo(req.CharId)
	if c == nil {
		return
	}
	p := c.GetOutfitPreset(c.InUseOutfitPresetIndex)
	p.OutfitHideInfo = &model.OutfitHideInfo{HideOrn: req.GetHideInfo().GetHideOrn(), HideBraid: req.GetHideInfo().GetHideBraid()}
	rsp.HideInfo = p.OutfitHideInfo.OutfitHideInfo()
	rsp.Status = proto.StatusCode_StatusCode_Ok
	_ = s.SavePlayer()
	g.SceneActionCharacterUpdate(s, proto.SceneActionType_SceneActionType_UpdateFashion, req.CharId)
}
