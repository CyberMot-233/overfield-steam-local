package game

import (
	"gucooing/lolo/game/model"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/pkg/alg"
	"gucooing/lolo/protocol/proto"
	"math/rand/v2"
)

func (g *Game) OutfitDyeUnlockIndex(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.OutfitDyeUnlockIndexReq)
	rsp := &proto.OutfitDyeUnlockIndexRsp{Status: proto.StatusCode_StatusCode_BadReq}
	defer g.send(s, msg.PacketId, rsp)
	f := s.GetItemModel().GetItemFashionInfo(req.OutfitId)
	if f.LocalUnlockDye(req.Index) != nil {
		return
	}
	rsp.Status = proto.StatusCode_StatusCode_Ok
	g.PackNoticeByItems(s, []*proto.ItemDetail{f.ItemDetail()})
	_ = s.SavePlayer()
}
func (g *Game) LocalOutfitColorantSelect(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.OutfitColorantSelectReq)
	rsp := &proto.OutfitColorantSelectRsp{Status: proto.StatusCode_StatusCode_BadReq, Param: &proto.OutfitDyeParam{PictureId: 1, Params: []float64{}}}
	defer g.send(s, msg.PacketId, rsp)
	f := s.GetItemModel().GetItemFashionInfo(req.OutfitId)
	if f == nil {
		return
	}
	for _, c := range gdconf.GetLocalCatalog().Dye.GetDyeData().GetDatas() {
		if uint32(c.ID) == req.ColorantItemId {
			f.ColorantID = req.ColorantItemId
			f.PreviewColor = &model.PosColor{Red: uint32(c.ColorR), Green: uint32(c.ColorG), Blue: uint32(c.ColorB)}
			if c.NewDyeType == 0 {
				f.PreviewColor = &model.PosColor{Red: rand.Uint32N(256), Green: rand.Uint32N(256), Blue: rand.Uint32N(256)}
			}
			rsp.Status = proto.StatusCode_StatusCode_Ok
			rsp.Param.IsDye = true
			return
		}
	}
}
func (g *Game) OutfitRandomColor(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.OutfitRandomColorReq)
	rsp := &proto.OutfitRandomColorRsp{Status: proto.StatusCode_StatusCode_BadReq, OutfitId: req.OutfitId}
	defer g.send(s, msg.PacketId, rsp)
	f := s.GetItemModel().GetItemFashionInfo(req.OutfitId)
	if f == nil {
		return
	}
	f.PreviewColor = &model.PosColor{Red: rand.Uint32N(256), Green: rand.Uint32N(256), Blue: rand.Uint32N(256)}
	rsp.PosColor = &proto.PosColor{Red: f.PreviewColor.Red, Green: f.PreviewColor.Green, Blue: f.PreviewColor.Blue}
	rsp.Status = proto.StatusCode_StatusCode_Ok
}
func (g *Game) LocalOutfitDye(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.OutfitDyeReq)
	rsp := &proto.OutfitDyeRsp{Status: proto.StatusCode_StatusCode_NotDye, OutfitId: req.OutfitId, TotalGuaranteeDyeNum: 5}
	defer g.send(s, msg.PacketId, rsp)
	f := s.GetItemModel().GetItemFashionInfo(req.OutfitId)
	if f == nil {
		return
	}
	color := req.SpecifyColor
	if color == nil && f.PreviewColor != nil {
		c := f.PreviewColor
		color = &proto.PosColor{Red: c.Red, Green: c.Green, Blue: c.Blue}
	}
	if color == nil {
		return
	}
	color = &proto.PosColor{Pos: req.Pos, Red: color.Red, Green: color.Green, Blue: color.Blue}
	if f.LocalStageDye(req.SchemeIndex, color) != nil {
		return
	}
	rsp.PosColor = color
	rsp.Status = proto.StatusCode_StatusCode_Ok
	_ = s.SavePlayer()
}
func (g *Game) LocalOutfitDyeSave(s *model.Player, msg *alg.GameMsg) {
	req := msg.Body.(*proto.OutFitDyeSaveReq)
	rsp := &proto.OutFitDyeSaveRsp{Status: proto.StatusCode_StatusCode_NotDye, SchemeIndex: req.SchemeIndex, IsSaveDyeResult: req.IsSaveDyeResult, Items: []*proto.ItemDetail{}}
	defer g.send(s, msg.PacketId, rsp)
	f := s.GetItemModel().GetItemFashionInfo(req.OutfitId)
	color, e := f.LocalSaveDye(req.SchemeIndex, req.Pos, req.IsSaveDyeResult)
	if e != nil {
		return
	}
	rsp.PosColor = color
	rsp.Items = []*proto.ItemDetail{f.ItemDetail()}
	rsp.Status = proto.StatusCode_StatusCode_Ok
	g.PackNoticeByItems(s, rsp.Items)
	_ = s.SavePlayer()
}
