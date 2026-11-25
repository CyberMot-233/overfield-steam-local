package model

import (
	"encoding/json"
	"gucooing/lolo/gdconf"
	"reflect"
	"testing"
)

func TestLocalUnlockAllPersistsAndPreservesAccount(t *testing.T) {
	localTestConfig(t)
	p := new(Player)
	c := p.AddCharacter(101001)
	p.GetTeamModel().GetTeamInfo().Char1 = c.CharacterId
	p.GetItemModel().AddItemBase(101, 777)
	p.GetItemModel().AddItemWeapon(1101101)
	p.LocalUnlockGatherTools()
	alternate := p.GetItemModel().AddItemWeapon(1003002)
	if alternate == nil {
		t.Fatal("missing alternate mining tool")
	}
	c.CharacterAppearance.MiningHammerInstanceId = alternate.InstanceId
	beforeChar, _ := json.Marshal(p.Character)
	c.CharacterAppearance.LoggingAxeInstanceId = 0
	beforeWeapons, _ := json.Marshal(p.Item.ItemWeaponMap)
	result := p.LocalUnlockAll()
	if result["toolSlots"] < 5 {
		t.Fatal("gather slots not filled", result)
	}
	if result["actions"] == 0 || result["actions"] != len(LocalAvailableActions()) || result["abilities"] != len(gdconf.GetAbilityMap()) {
		t.Fatalf("incomplete unlock result %v", result)
	}
	for id, conf := range gdconf.GetAbilityMap() {
		max := uint32(0)
		for _, lv := range conf.LevelInfo {
			if uint32(lv.Level) > max {
				max = uint32(lv.Level)
			}
		}
		if p.Unlocks.Abilities[id] != max {
			t.Fatalf("ability %d not max", id)
		}
	}
	if p.Item.ItemBaseInfo[101].Num != 777 || p.Team.TeamInfo.Char1 != 101001 {
		t.Fatal("currency or party changed")
	}
	afterChar, _ := json.Marshal(p.Character)
	afterWeapons, _ := json.Marshal(p.Item.ItemWeaponMap)
	if string(beforeChar) != string(afterChar) || string(beforeWeapons) != string(afterWeapons) {
		t.Fatal("character or weapon changed")
	}
	data, _ := json.Marshal(p)
	restored := new(Player)
	if e := json.Unmarshal(data, restored); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(restored.LocalUnlockSummary(), result) {
		t.Fatal("unlocks not persisted")
	}
	again := p.LocalUnlockAll()
	if !reflect.DeepEqual(again, result) {
		t.Fatal("unlock is not idempotent")
	}
	final, _ := json.Marshal(p)
	if string(data) != string(final) {
		t.Fatal("repeat changed save")
	}
}
