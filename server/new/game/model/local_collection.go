package model

import (
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
)

// LocalItemAvailable excludes placeholder entries without usable client data.
func LocalItemAvailable(id uint32) bool {
	item := gdconf.GetItemConfigure(id)
	if item == nil {
		return false
	}
	switch proto.EBagItemTag(item.NewBagItemTag) {
	case proto.EBagItemTag_EBagItemTag_TreasureMap:
		// A map needs a TreasureInstance and valid digging coordinates. Sending
		// a stackable BaseItem produces null TreasureMapModel entries in Steam.
		return false
	case proto.EBagItemTag_EBagItemTag_Armor:
		c := gdconf.GetArmorAllInfo(id)
		property, _ := gdconf.LocalArmorProperty(id, 1)
		return c != nil && c.ArmorInfo != nil && property != nil && gdconf.LocalItemHasIcon(id)
	case proto.EBagItemTag_EBagItemTag_UnlockItem:
		return gdconf.GetPlayerUnlockConfigure(int32(id)) != nil
	case proto.EBagItemTag_EBagItemTag_AbilityItem:
		return gdconf.GetPlayerAbilityConfigure(int32(id)) != nil
	case proto.EBagItemTag_EBagItemTag_Card:
		return LocalCharacterAvailable(id)
	case proto.EBagItemTag_EBagItemTag_Fashion:
		return gdconf.GetFashionAllInfo(id) != nil
	case proto.EBagItemTag_EBagItemTag_Weapon:
		c := gdconf.GetWeaponAllInfo(id)
		return c != nil && c.WeaponInfo != nil && c.PropertyGroup[10] != nil && gdconf.LocalItemHasIcon(id)
	case proto.EBagItemTag_EBagItemTag_Poster:
		c := gdconf.GetPosterAllInfo(id)
		return c != nil && c.PosterInfo != nil && c.PosterIllustration != nil && c.PosterIllustration.IsShow
	}
	return true
}

func (p *Player) LocalOwnsItem(id uint32) bool {
	i := p.GetItemModel()
	if i.GetItemBaseInfo(id) != nil || p.GetCharacterModel().GetCharacterInfo(id) != nil || i.GetItemFashionInfo(id) != nil {
		return true
	}
	if p.GetChatModel().UnLockExpression[id] != nil {
		return true
	}
	for _, v := range i.ItemWeaponMap {
		if v.ItemId == id {
			return true
		}
	}
	for _, v := range i.ItemArmorMap {
		if v.ItemId == id {
			return true
		}
	}
	for _, v := range i.ItemPosterMap {
		if v.ItemId == id {
			return true
		}
	}
	for _, v := range i.ItemInscriptionMap {
		if v.ItemId == id {
			return true
		}
	}
	for _, v := range i.ItemPetInfoMap {
		if v.ItemId == id {
			return true
		}
	}
	for _, v := range i.ItemHeadMap {
		if v.ItemId == id {
			return true
		}
	}
	return false
}

// Ability and unlock entries are permanent flags, not stackable resources.
func LocalUniqueItem(id uint32) bool {
	c := gdconf.GetItemConfigure(id)
	if c == nil {
		return false
	}
	switch proto.EBagItemTag(c.NewBagItemTag) {
	case proto.EBagItemTag_EBagItemTag_AbilityItem, proto.EBagItemTag_EBagItemTag_UnlockAbilityItem, proto.EBagItemTag_EBagItemTag_UnlockItem, proto.EBagItemTag_EBagItemTag_Fashion:
		return true
	}
	return false
}

func (p *Player) LocalNormalizeUnlockItems() int {
	changed := 0
	for id, it := range p.GetItemModel().ItemBaseInfo {
		if LocalUniqueItem(id) && it.Num > 1 {
			it.Num = 1
			changed++
		}
	}
	return changed
}

// AtLeast grants a collection idempotently. A successful constructor is checked
// against the resulting inventory, not merely a non-nil AddItemCtx.
func (p *Player) LocalGrantItem(id uint32, count int64, atLeast bool) bool {
	if count < 1 || count > 999999999 || !LocalItemAvailable(id) {
		return false
	}
	i := p.GetItemModel()
	tag := proto.EBagItemTag(gdconf.GetItemConfigure(id).NewBagItemTag)
	if LocalUniqueItem(id) {
		count = 1
		if old := i.GetItemBaseInfo(id); old != nil {
			old.Num = 1
			return true
		}
	}
	if atLeast {
		if old := i.GetItemBaseInfo(id); old != nil {
			if old.Num < count {
				old.Num = count
			}
			return true
		}
		if p.LocalOwnsItem(id) {
			if i.GetItemFashionInfo(id) != nil {
				i.AddItemBase(id, 1)
			}
			return true
		}
	}
	p.AddAllTypeItem(id, count)
	if !p.LocalOwnsItem(id) && (tag == proto.EBagItemTag_EBagItemTag_UnlockItem || tag == proto.EBagItemTag_EBagItemTag_AbilityItem) {
		i.AddItemBase(id, count)
	}
	if i.GetItemFashionInfo(id) != nil && i.GetItemBaseInfo(id) == nil {
		i.AddItemBase(id, 1)
	}
	return p.LocalOwnsItem(id)
}

// LocalCharacterAvailable checks playable configuration. IsShow controls the
// gallery and also hides complete limited characters such as Lia in Steam.
func LocalCharacterAvailable(id uint32) bool {
	c := gdconf.GetCharacterAll(id)
	if c == nil || c.CharacterInfo == nil || len(c.LevelRules) == 0 {
		return false
	}
	v := c.CharacterInfo
	assets := gdconf.GetLocalCatalog().ClientAssets
	if assets != nil {
		if !assets.CharacterAssets[id] {
			return false
		}
	} else if !v.IsShow {
		return false
	}
	if v.AssetFullName == "" || v.FacePath == "" || len(v.SpellIDs) < 2 || len(v.ExSpellIDs) == 0 {
		return false
	}
	if gdconf.GetItemConfigure(id) == nil || gdconf.GetCharacterLevelMap(v.GrowthLevelID)[1] == nil {
		return false
	}
	w := gdconf.GetWeaponAllInfo(uint32(v.DefaultWeaponID))
	if w == nil || w.WeaponInfo == nil || w.PropertyGroup[10] == nil {
		return false
	}
	return gdconf.GetFashionAllInfo(uint32(v.HairID)) != nil && gdconf.GetFashionAllInfo(uint32(v.ClothID)) != nil
}
