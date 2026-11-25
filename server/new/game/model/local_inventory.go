package model

import (
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
	"math"
	"sort"
)

// Legacy placeholders are kept in the save for recovery, outside the wire inventory.
// The archive never goes through PackNotice or the equipment-list serializers.
type LocalInventoryArchive struct {
	Base    map[uint32]*ItemBaseInfo   `json:"base,omitempty"`
	Armor   map[uint32]*ItemArmorInfo  `json:"armor,omitempty"`
	Weapons map[uint32]*ItemWeaponInfo `json:"weapons,omitempty"`
}

type LocalInventoryRepair struct {
	BaseQuarantined       int `json:"baseQuarantined"`
	ArmorQuarantined      int `json:"armorQuarantined"`
	WeaponsQuarantined    int `json:"weaponsQuarantined"`
	EquippedArmorReplaced int `json:"equippedArmorReplaced"`
	ArmorStatsInitialized int `json:"armorStatsInitialized"`
}

// Steam decodes main_property_val / 100. A value without a supported property
// type (the old constructor emitted MaxHP=0) contributes no equipment stats.
// Give formerly empty local armor a defense roll at its existing level, within
// the client's configured interpolated range. Existing nonzero rolls are retained.
func LocalInitializeArmorStats(a *ItemArmorInfo) bool {
	if a == nil {
		return false
	}
	if a.MainPropertyVal != 0 {
		return true
	}
	level := a.Level
	if level == 0 {
		level = 1
	}
	row, index := gdconf.LocalArmorProperty(a.ArmorId, level)
	if row == nil {
		return false
	}
	ratio := float64(0)
	if row.MaxLevel > row.MinLevel {
		ratio = float64(int64(level)-int64(row.MinLevel)) / float64(row.MaxLevel-row.MinLevel)
	}
	value := float64(row.MinExtDefense) + (float64(row.MaxExtDefense)-float64(row.MinExtDefense))*ratio
	low, high := float64(row.MinFloatValue), float64(row.MaxFloatValue)
	if low == 0 && high == 0 {
		low, high = 1, 1
	}
	value *= (low + high) / 2
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	a.MainPropertyType = proto.EPropertyType_EPropertyType_ExtDefense
	a.MainPropertyVal = uint32(math.Round(value * 100))
	a.Level = level
	a.PropertyIndex = index
	return a.MainPropertyVal > 0
}

func (p *Player) inventoryArchive() *LocalInventoryArchive {
	if p.InventoryArchive == nil {
		p.InventoryArchive = &LocalInventoryArchive{}
	}
	a := p.InventoryArchive
	if a.Base == nil {
		a.Base = map[uint32]*ItemBaseInfo{}
	}
	if a.Armor == nil {
		a.Armor = map[uint32]*ItemArmorInfo{}
	}
	if a.Weapons == nil {
		a.Weapons = map[uint32]*ItemWeaponInfo{}
	}
	return a
}

func localArmorReplacement(id uint32) uint32 {
	old, item := gdconf.GetArmorAllInfo(id), gdconf.GetItemConfigure(id)
	if old == nil || old.ArmorInfo == nil || item == nil {
		return 0
	}
	ids := []int{}
	for candidate, c := range gdconf.GetArmorAllMap() {
		it := gdconf.GetItemConfigure(candidate)
		if c != nil && c.ArmorInfo != nil && it != nil && it.Quality == item.Quality &&
			c.ArmorInfo.NewEquipType == old.ArmorInfo.NewEquipType &&
			c.ArmorInfo.NewWeaponSystemType == old.ArmorInfo.NewWeaponSystemType && LocalItemAvailable(candidate) {
			ids = append(ids, int(candidate))
		}
	}
	sort.Ints(ids)
	if len(ids) == 0 {
		return 0
	}
	return uint32(ids[0])
}

// Repair the existing player in place. Equipment references, instance numbers,
// character progression, outfit presets and other inventory fields are preserved.
func (p *Player) LocalRepairInventory() LocalInventoryRepair {
	report := LocalInventoryRepair{}
	items := p.GetItemModel()
	wornArmor, wornWeapons := map[uint32]bool{}, map[uint32]bool{}
	for _, c := range p.GetCharacterModel().GetCharacterMap() {
		if c == nil {
			continue
		}
		for _, preset := range c.EquipmentPresetList {
			if preset == nil {
				continue
			}
			if preset.WeaponInstanceId != 0 {
				wornWeapons[preset.WeaponInstanceId] = true
			}
			for _, slot := range preset.Armors {
				if slot != nil && slot.InstanceId != 0 {
					wornArmor[slot.InstanceId] = true
				}
			}
		}
	}
	for id, item := range items.ItemBaseInfo {
		if item != nil && item.ItemType == proto.EBagItemTag_EBagItemTag_TreasureMap {
			archive := p.inventoryArchive()
			if archive.Base[id] == nil {
				archive.Base[id] = item
			}
			delete(items.ItemBaseInfo, id)
			report.BaseQuarantined++
		}
	}
	for id, armor := range items.ItemArmorMap {
		if armor == nil {
			continue
		}
		if !LocalItemAvailable(armor.ArmorId) {
			archive := p.inventoryArchive()
			if archive.Armor[id] == nil {
				archive.Armor[id] = armor
			}
			if armor.WearerId != 0 || wornArmor[id] {
				replacement := localArmorReplacement(armor.ArmorId)
				if replacement != 0 {
					copyArmor, copyBase := *armor, *armor.ItemBaseInfo
					copyArmor.ItemBaseInfo = &copyBase
					copyArmor.ItemId, copyArmor.ArmorId = replacement, replacement
					// An unsupported property on an old placeholder must not survive conversion.
					copyArmor.MainPropertyVal = 0
					armor = &copyArmor
					items.ItemArmorMap[id] = armor
					report.EquippedArmorReplaced++
				} else {
					continue
				} // Preserve a worn instance if no compatible replacement exists.
			} else {
				delete(items.ItemArmorMap, id)
				report.ArmorQuarantined++
				continue
			}
		}
		if armor.MainPropertyVal == 0 && LocalInitializeArmorStats(armor) {
			report.ArmorStatsInitialized++
		}
	}
	for id, weapon := range items.ItemWeaponMap {
		if weapon == nil || LocalItemAvailable(weapon.WeaponId) {
			continue
		}
		// Do not break references to an already equipped or character-default weapon.
		if weapon.WearerId != 0 || wornWeapons[id] {
			continue
		}
		archive := p.inventoryArchive()
		if archive.Weapons[id] == nil {
			archive.Weapons[id] = weapon
		}
		delete(items.ItemWeaponMap, id)
		report.WeaponsQuarantined++
	}
	return report
}
