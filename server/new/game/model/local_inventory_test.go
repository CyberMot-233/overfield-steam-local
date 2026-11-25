package model

import (
	"encoding/json"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
	"reflect"
	"testing"
)

func TestLocalInventoryRepairPreservesProgressAndEquipment(t *testing.T) {
	localTestConfig(t)
	if gdconf.GetLocalCatalog().ItemIcons == nil {
		t.Skip("UI icon table unavailable")
	}
	p := new(Player)
	c := p.AddCharacter(101001)
	if c == nil {
		t.Fatal("missing initial character")
	}
	p.GetTeamModel().GetTeamInfo().Char1 = 101001
	i := p.GetItemModel()
	i.AddItemBase(101, 12345)
	i.AddItemBase(5070001, 9999999) // Old serializer used the wrong protobuf oneof.
	i.GetItemArmorMap()[800] = &ItemArmorInfo{ItemBaseInfo: &ItemBaseInfo{ItemId: 2133009, ItemType: proto.EBagItemTag_EBagItemTag_Armor}, ArmorId: 2133009, InstanceId: 800, WeaponSystemType: 1, Level: 1, PropertyIndex: 1, WearerId: 101001, IsLock: true, StrengthLevel: 2}
	c.GetEquipmentPreset(0).GetArmor(proto.EEquipType(2)).InstanceId = 800
	i.GetItemArmorMap()[801] = &ItemArmorInfo{ItemBaseInfo: &ItemBaseInfo{ItemId: 2111008, ItemType: proto.EBagItemTag_EBagItemTag_Armor}, ArmorId: 2111008, InstanceId: 801, WeaponSystemType: 1, Level: 1}
	i.GetItemArmorMap()[802] = &ItemArmorInfo{ItemBaseInfo: &ItemBaseInfo{ItemId: 2111001, ItemType: proto.EBagItemTag_EBagItemTag_Armor}, ArmorId: 2111001, InstanceId: 802, WeaponSystemType: 1, Level: 1}
	i.GetItemWeaponMap()[803] = &ItemWeaponInfo{ItemBaseInfo: &ItemBaseInfo{ItemId: 1301100, ItemType: proto.EBagItemTag_EBagItemTag_Weapon}, WeaponId: 1301100, InstanceId: 803}
	outfit, _ := json.Marshal(c.OutfitPresetList)
	r := p.LocalRepairInventory()
	if r.BaseQuarantined != 1 || r.ArmorQuarantined != 1 || r.WeaponsQuarantined != 1 || r.EquippedArmorReplaced != 1 || r.ArmorStatsInitialized != 2 {
		t.Fatalf("unexpected repair: %+v", r)
	}
	if i.ItemBaseInfo[101].Num != 12345 || p.GetTeamModel().GetTeamInfo().Char1 != 101001 || c.GetEquipmentPreset(0).GetArmor(proto.EEquipType(2)).InstanceId != 800 {
		t.Fatal("progress or equipment reference changed")
	}
	armor := i.ItemArmorMap[800]
	if !LocalItemAvailable(armor.ArmorId) || armor.InstanceId != 800 || armor.WearerId != 101001 || !armor.IsLock || armor.StrengthLevel != 2 || armor.MainPropertyVal == 0 || armor.MainPropertyType != proto.EPropertyType_EPropertyType_ExtDefense {
		t.Fatal("equipped armor repair lost fields")
	}
	old, new := gdconf.GetArmorAllInfo(2133009).ArmorInfo, gdconf.GetArmorAllInfo(armor.ArmorId).ArmorInfo
	if old.NewEquipType != new.NewEquipType || old.NewWeaponSystemType != new.NewWeaponSystemType || gdconf.GetItemConfigure(2133009).Quality != gdconf.GetItemConfigure(armor.ArmorId).Quality {
		t.Fatal("replacement changed slot, class or quality")
	}
	if p.InventoryArchive.Base[5070001].Num != 9999999 || p.InventoryArchive.Armor[800].ArmorId != 2133009 || p.InventoryArchive.Armor[800].MainPropertyVal != 0 || p.InventoryArchive.Weapons[803].WeaponId != 1301100 {
		t.Fatal("original items not archived")
	}
	afterOutfit, _ := json.Marshal(c.OutfitPresetList)
	if string(outfit) != string(afterOutfit) {
		t.Fatal("outfit changed")
	}
	before, _ := json.Marshal(p)
	again := p.LocalRepairInventory()
	after, _ := json.Marshal(p)
	if !reflect.DeepEqual(again, LocalInventoryRepair{}) || string(before) != string(after) {
		t.Fatal("repair is not idempotent")
	}
	var restored Player
	if err := json.Unmarshal(after, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.InventoryArchive.Base[5070001].Num != 9999999 || restored.Item.ItemArmorMap[800].MainPropertyVal == 0 {
		t.Fatal("archive/attributes did not persist")
	}
}

func TestLocalInventoryGrantRejectsBrokenItemsAndRollsArmor(t *testing.T) {
	localTestConfig(t)
	if gdconf.GetLocalCatalog().ItemIcons == nil {
		t.Skip("UI icon table unavailable")
	}
	p := new(Player)
	for _, id := range []uint32{5070001, 5070002, 2133009, 2111008, 1301100} {
		if LocalItemAvailable(id) || p.LocalGrantItem(id, 1, false) || p.AddAllTypeItem(id, 1) != nil {
			t.Fatal("broken item was granted", id)
		}
	}
	for id := range gdconf.GetArmorAllMap() {
		if !LocalItemAvailable(id) {
			continue
		}
		a := p.GetItemModel().AddItemArmor(id)
		if a == nil || a.MainPropertyVal == 0 || a.PropertyIndex == 0 || a.MainPropertyType != proto.EPropertyType_EPropertyType_ExtDefense {
			t.Fatal("armor has empty property", id)
		}
		row, _ := gdconf.LocalArmorProperty(id, a.Level)
		decoded := float64(a.ArmorInstance().MainPropertyVal) / 100
		if decoded < float64(row.MinExtDefense*row.MinFloatValue)-0.01 || decoded > float64(row.MaxExtDefense*row.MaxFloatValue)+0.01 {
			t.Fatal("decoded property outside configured range", id, decoded)
		}
		a.MainPropertyVal = 4321
		if !LocalInitializeArmorStats(a) || a.MainPropertyVal != 4321 {
			t.Fatal("existing valid stat rerolled")
		}
	}
}
