package model

import (
	"encoding/json"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
	"testing"
)

func TestLocalHiddenCharacterAndNilItem(t *testing.T) {
	localTestConfig(t)
	if gdconf.GetLocalCatalog().ClientAssets == nil {
		t.Skip("client asset catalog not installed")
	}
	p := new(Player)
	for _, id := range []uint32{302003, 201002} {
		if !LocalCharacterAvailable(id) {
			t.Fatal("complete hidden character excluded", id)
		}
		c := p.AddCharacter(id)
		if c == nil || c.CharacterId != id || c.GetOutfitPreset(0).Clothes == 0 || c.GetEquipmentPreset(0).WeaponInstanceId == 0 {
			t.Fatal("character initialization incomplete", id)
		}
		if d := p.AddAllTypeItem(id, 1).AddItemDetail(); d == nil {
			t.Fatal("character item serialization failed", id)
		}
	}
	for _, id := range []uint32{102003, 202003, 403004} {
		if LocalCharacterAvailable(id) || p.AddCharacter(id) != nil {
			t.Fatal("incomplete character accepted", id)
		}
	}
	var missing *CharacterInfo
	if (&AddItemCtx{EBagItemTag: missing}).AddItemDetail() != nil {
		t.Fatal("typed nil accepted")
	}
	if p.AddAllTypeItem(102003, 1) != nil {
		t.Fatal("nil character wrapped in item")
	}
	raw, _ := json.Marshal(p)
	var restored Player
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.GetCharacterModel().GetCharacterInfo(302003) == nil {
		t.Fatal("Lia did not persist")
	}
	if _, err := p.NewGachaCtx(&proto.GachaReq{GachaId: 999999999, IsSingle: true}); err == nil {
		t.Fatal("unknown pool accepted")
	}
}
