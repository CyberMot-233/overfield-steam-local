package model

import (
	"encoding/json"
	"errors"
	"gucooing/lolo/gdconf"
	"reflect"
	"testing"
)

func TestLocalWeaponEditInstanceAndRoundTrip(t *testing.T) {
	localTestConfig(t)
	p := new(Player)
	p.AddCharacter(101001)
	p.GetItemModel().AddItemBase(101, 987)
	w := p.GetItemModel().AddItemWeapon(1101101)
	if w == nil {
		t.Fatal("missing weapon configuration")
	}
	duplicate := p.GetItemModel().AddItemWeapon(w.WeaponId)
	w.WearerId = 101001
	w.IsLock = true
	w.Inscription1 = 121
	w.Durability = 77
	before, _ := json.Marshal(p)
	old := *w
	r, ok := gdconf.LocalWeaponRangeForLevel(w.WeaponId, 25)
	if !ok {
		t.Fatal("missing level range")
	}
	req := LocalWeaponEdit{w.InstanceId, w.WeaponId, LocalWeaponRevision(w), map[string]float64{"level": 25, "attack": 765432, "criticalRatio": 100, "damageBalance": 125, "strengthLevel": float64(r.StrengthLevels[len(r.StrengthLevels)-1])}}
	if _, err := p.LocalEditWeapon(req); err != nil {
		t.Fatal(err)
	}
	if w.LocalStats["attack"] != 765432 || w.Level != 25 || w.LocalStats["criticalRatio"] != 100 || w.LocalStats["damageBalance"] != 125 || w.PropertyIndex != r.PropertyIndex {
		t.Fatalf("incorrect edit %+v", w)
	}
	if w.InstanceId != old.InstanceId || w.WearerId != old.WearerId || w.Inscription1 != 121 || w.Durability != 77 || !w.IsLock || !reflect.DeepEqual(w.RandomProperty, old.RandomProperty) {
		t.Fatal("unrelated weapon state changed")
	}
	after, _ := json.Marshal(p)
	restored := new(Player)
	if err := json.Unmarshal(after, restored); err != nil {
		t.Fatal(err)
	}
	restoredWeapon, _ := json.Marshal(restored.GetItemModel().GetItemWeaponInfo(w.InstanceId))
	expectedWeapon, _ := json.Marshal(w)
	if string(restoredWeapon) != string(expectedWeapon) {
		t.Fatal("weapon did not survive JSON round trip")
	}
	restoredDuplicate, _ := json.Marshal(restored.GetItemModel().GetItemWeaponInfo(duplicate.InstanceId))
	expectedDuplicate, _ := json.Marshal(duplicate)
	if string(restoredDuplicate) != string(expectedDuplicate) {
		t.Fatal("same-name instance modified")
	}
	*w = old
	again, _ := json.Marshal(p)
	if string(again) != string(before) {
		t.Fatal("another part of the player changed")
	}
}
func TestLocalWeaponRejectsWithoutPartialMutation(t *testing.T) {
	localTestConfig(t)
	p := new(Player)
	w := p.GetItemModel().AddItemWeapon(1101101)
	if w == nil {
		t.Fatal("missing weapon")
	}
	initial, _ := json.Marshal(p)
	for _, changes := range []map[string]float64{{"attack": 99, "level": 0}, {"attack": 99, "level": 1000}, {"strengthLevel": 999}, {"attack": float64(LocalWeaponMaxAttack + 1)}, {"criticalRatio": float64(LocalWeaponMaxPercent + 1)}, {"damageBalance": float64(LocalWeaponMaxPercent + 1)}, {"weaponId": 99}, {}} {
		_, err := p.LocalEditWeapon(LocalWeaponEdit{w.InstanceId, w.WeaponId, LocalWeaponRevision(w), changes})
		if err == nil {
			t.Fatalf("accepted invalid fields %v", changes)
		}
		got, _ := json.Marshal(p)
		if string(got) != string(initial) {
			t.Fatal("rejected edit partially changed the save")
		}
	}
	req := LocalWeaponEdit{w.InstanceId, w.WeaponId, LocalWeaponRevision(w), map[string]float64{"attack": 222}}
	if _, err := p.LocalEditWeapon(req); err != nil {
		t.Fatal(err)
	}
	if _, err := p.LocalEditWeapon(req); !errors.Is(err, ErrLocalWeaponConflict) {
		t.Fatal("stale edit was not rejected", err)
	}
	req.Revision = LocalWeaponRevision(w)
	req.WeaponID++
	if _, err := p.LocalEditWeapon(req); err == nil {
		t.Fatal("wrong item ID accepted")
	}
	req.WeaponID = w.WeaponId
	req.InstanceID = 4294967295
	if _, err := p.LocalEditWeapon(req); err == nil {
		t.Fatal("missing instance accepted")
	}
}
func TestLocalWeaponAllConfiguredLevelBoundaries(t *testing.T) {
	localTestConfig(t)
	count := 0
	for id := range gdconf.GetWeaponAllMap() {
		if !LocalItemAvailable(id) {
			continue
		}
		p := new(Player)
		w := p.GetItemModel().AddItemWeapon(id)
		if w == nil {
			continue
		}
		ranges := gdconf.LocalWeaponRanges(id)
		if len(ranges) == 0 {
			t.Fatalf("no ranges for %d", id)
		}
		for _, r := range ranges {
			for _, lv := range []uint32{r.Min, r.Max} {
				req := LocalWeaponEdit{w.InstanceId, w.WeaponId, LocalWeaponRevision(w), map[string]float64{"level": float64(lv), "strengthLevel": 0}}
				if _, err := p.LocalEditWeapon(req); err != nil {
					t.Fatalf("weapon %d level %d: %v", id, lv, err)
				}
				if w.PropertyIndex != r.PropertyIndex {
					t.Fatal("wrong property group")
				}
				count++
			}
		}
	}
	if count < 1000 {
		t.Fatalf("insufficient configuration coverage: %d", count)
	}
	t.Logf("validated %d weapon level boundaries", count)
}
