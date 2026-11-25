package gdconf

import (
	"gucooing/lolo/protocol/excel"
	"testing"
)

func TestLocalGachaSelectionIsAtomic(t *testing.T) {
	a := &GachaData{Conf: &excel.GachaInfoConfigure{ID: 3004, ClosingTime: "old"}}
	b := &GachaData{Conf: &excel.GachaInfoConfigure{ID: 103006}}
	g := &GameConfig{Excel: &Excel{Gacha: &Gacha{GachaDatas: map[uint32]*GachaData{3004: a, 103006: b}, OpenList: []*GachaData{a}}}}
	c := LocalGachaConfig{Mode: "all-available", PoolIDs: []uint32{3004, 99}, OpeningTime: "2020-01-01 00:00:00", ClosingTime: "2099-12-31 23:59:59"}
	if g.applyLocalGacha(c) == nil || a.Conf.ClosingTime != "old" || len(g.Excel.Gacha.OpenList) != 1 {
		t.Fatal("invalid catalog changed active data")
	}
	c.PoolIDs = []uint32{103006, 3004}
	if err := g.applyLocalGacha(c); err != nil {
		t.Fatal(err)
	}
	if len(g.Excel.Gacha.OpenList) != 2 || g.Excel.Gacha.OpenList[0] != b || b.Conf.Index != 1 || a.Conf.ClosingTime != c.ClosingTime {
		t.Fatal("schedule or order not applied")
	}
	c.PoolIDs = []uint32{3004, 3004}
	if g.applyLocalGacha(c) == nil {
		t.Fatal("duplicate pool accepted")
	}
}
