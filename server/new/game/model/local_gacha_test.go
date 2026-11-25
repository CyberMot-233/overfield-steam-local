package model

import (
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
	"testing"
)

func TestLocalGachaGuaranteeAndClosedPool(t *testing.T) {
	localTestConfig(t)
	g := DefaultGachaModel()
	kind := proto.EUIGachaType_EUIGachaType_Limit
	g.GetGachaTypeInfo(kind).GuaranteeSSR = 49
	if got := g.GetPbGachaInfo(103005); got == nil || got.Guarantee != 21 {
		t.Fatal("shared remaining guarantee is incorrect", got)
	}
	if g.GetPbGachaInfo(103006).Guarantee != 21 {
		t.Fatal("limited pools did not share guarantee")
	}
	if g.GetPbGachaInfo(99999999) != nil {
		t.Fatal("unknown pool accepted")
	}
	p := new(Player)
	if !gdconf.IsOpenGachaID(3013) {
		if _, err := p.NewGachaCtx(&proto.GachaReq{GachaId: 3013, IsSingle: true}); err == nil {
			t.Fatal("closed pool accepted")
		}
		if len(p.GetItemModel().ItemBaseInfo) != 0 {
			t.Fatal("invalid draw changed inventory")
		}
	}
	ctx := &GachaCtx{conf: gdconf.GetGachaData(103005), probability: gdconf.GetGachaProbability(3), typeInfo: &GachaTypeInfo{GuaranteeSSR: 70}}
	if _, kind := ctx.getPool(); kind != characterSSR || ctx.typeInfo.GuaranteeSSR != 0 {
		t.Fatal("legacy counter skipped guaranteed draw")
	}
}
