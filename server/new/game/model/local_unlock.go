package model

import (
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
	"sort"
)

type LocalUnlockState struct {
	Actions    map[uint32]bool     `json:"actions,omitempty"`
	Abilities  map[uint32]uint32   `json:"abilities,omitempty"`
	BlessTrees map[uint32][]uint32 `json:"blessTrees,omitempty"`
}

func (p *Player) LocalUnlockState() *LocalUnlockState {
	if p.Unlocks == nil {
		p.Unlocks = &LocalUnlockState{}
	}
	u := p.Unlocks
	if u.Actions == nil {
		u.Actions = map[uint32]bool{}
	}
	if u.Abilities == nil {
		u.Abilities = map[uint32]uint32{}
	}
	if u.BlessTrees == nil {
		u.BlessTrees = map[uint32][]uint32{}
	}
	return u
}
func LocalAvailableActions() []uint32 {
	c := gdconf.GetLocalCatalog()
	assets := map[int32]bool{}
	ids := map[uint32]bool{}
	for _, v := range c.Action.GetActionInfo().GetDatas() {
		if v.AnimPrefab != "" {
			assets[v.ID] = true
		}
	}
	for _, v := range c.Expression.GetExpression().GetDatas() {
		for _, g := range v.ExpressionGroupInfo {
			if g.IsShow && g.ActionID > 0 && assets[g.ActionID] {
				ids[uint32(g.ActionID)] = true
			}
		}
	}
	result := []uint32{}
	for id := range ids {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
func (p *Player) LocalUnlockedActions() []uint32 {
	result := []uint32{}
	if p.Unlocks != nil {
		for id, yes := range p.Unlocks.Actions {
			if yes {
				result = append(result, id)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
func (p *Player) LocalAbilityList() []*proto.PlayerAbility {
	result := []*proto.PlayerAbility{}
	if p.Unlocks != nil {
		for id, level := range p.Unlocks.Abilities {
			result = append(result, &proto.PlayerAbility{AbilityId: id, Level: level})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AbilityId < result[j].AbilityId })
	return result
}
func (p *Player) LocalUnlockSummary() map[string]int {
	result := map[string]int{"actions": 0, "abilities": 0, "blessTrees": 0, "blessNodes": 0, "toolSlots": 0}
	if p.Unlocks != nil {
		result["actions"] = len(p.Unlocks.Actions)
		result["abilities"] = len(p.Unlocks.Abilities)
		result["blessTrees"] = len(p.Unlocks.BlessTrees)
		for _, nodes := range p.Unlocks.BlessTrees {
			result["blessNodes"] += len(nodes)
		}
	}
	if p.Character != nil {
		for _, c := range p.Character.CharacterMap {
			for _, slot := range localGatherSlots(c.CharacterAppearance) {
				if *slot.value != 0 {
					result["toolSlots"]++
				}
			}
		}
	}
	return result
}

type localGatherSlot struct {
	kind  int32
	value *uint32
}

func localGatherSlots(a *CharacterAppearance) []localGatherSlot {
	if a == nil {
		return nil
	}
	return []localGatherSlot{{4, &a.CollectionGlovesInstanceId}, {5, &a.LoggingAxeInstanceId}, {6, &a.MiningHammerInstanceId}, {7, &a.InsectNetInstanceId}, {8, &a.WaterBottleInstanceId}, {18, &a.FishingRodInstanceId}, {21, &a.VehicleInstanceId}, {22, &a.ShovelInstanceId}}
}

// The client checks the appearance tool slots when deciding whether gathering
// is unlocked. Fill only missing/invalid slots; retain every valid chosen tool.
func (p *Player) LocalUnlockGatherTools() {
	items := p.GetItemModel()
	candidates := map[int32]*ItemWeaponInfo{}
	for _, w := range items.GetItemWeaponMap() {
		conf := gdconf.GetWeaponAllInfo(w.WeaponId)
		if conf == nil || conf.WeaponInfo == nil || !LocalItemAvailable(w.WeaponId) {
			continue
		}
		kind := int32(conf.WeaponInfo.NewWeaponType)
		old := candidates[kind]
		if old == nil || w.WeaponId < old.WeaponId || (w.WeaponId == old.WeaponId && w.InstanceId < old.InstanceId) {
			candidates[kind] = w
		}
	}
	ids := []uint32{}
	for id := range gdconf.GetWeaponAllMap() {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, c := range p.GetCharacterModel().GetCharacterMap() {
		if c.CharacterAppearance == nil {
			c.CharacterAppearance = newCharacterAppearance(c.CharacterId)
		}
		for _, slot := range localGatherSlots(c.CharacterAppearance) {
			if current := items.GetItemWeaponInfo(*slot.value); current != nil {
				conf := gdconf.GetWeaponAllInfo(current.WeaponId)
				if conf != nil && conf.WeaponInfo != nil && int32(conf.WeaponInfo.NewWeaponType) == slot.kind && LocalItemAvailable(current.WeaponId) {
					continue
				}
			}
			chosen := candidates[slot.kind]
			if chosen == nil {
				for _, id := range ids {
					conf := gdconf.GetWeaponAllInfo(id)
					if conf.WeaponInfo != nil && int32(conf.WeaponInfo.NewWeaponType) == slot.kind && LocalItemAvailable(id) {
						chosen = items.AddItemWeapon(id)
						if chosen != nil {
							candidates[slot.kind] = chosen
							break
						}
					}
				}
			}
			if chosen != nil {
				*slot.value = chosen.InstanceId
			}
		}
	}
}

// Write the records that client menus actually use. Owning the unlock item alone
// is not equivalent to learning an ability or an emote.
func (p *Player) LocalUnlockAll() map[string]int {
	u := p.LocalUnlockState()
	for _, id := range LocalAvailableActions() {
		u.Actions[id] = true
	}
	for id, c := range gdconf.GetAbilityMap() {
		max := uint32(0)
		if c.NeedItemID > 0 {
			p.LocalGrantItem(uint32(c.NeedItemID), 1, true)
		}
		for _, lv := range c.LevelInfo {
			if uint32(lv.Level) > max {
				max = uint32(lv.Level)
			}
			if lv.RewardItemID > 0 {
				p.LocalGrantItem(uint32(lv.RewardItemID), 1, true)
			}
		}
		if max > u.Abilities[id] {
			u.Abilities[id] = max
		}
	}
	c := gdconf.GetLocalCatalog()
	for _, tree := range c.BlessingTree.GetBlessingTree().GetDatas() {
		found := map[uint32]bool{}
		for _, id := range u.BlessTrees[uint32(tree.ID)] {
			found[id] = true
		}
		for _, info := range c.BlessingTree.GetBlessingTreeInfo().GetDatas() {
			if info.ID == tree.BlessingTreeInfoID {
				for _, node := range info.BlessingTreeInfoGroupInfo {
					if node.BoxID > 0 {
						found[uint32(node.BoxID)] = true
					}
				}
			}
		}
		ids := []uint32{}
		for id := range found {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		u.BlessTrees[uint32(tree.ID)] = ids
	}
	for _, f := range p.GetItemModel().GetItemFashionMap() {
		for index := uint32(0); index < LocalDyeSchemeCount(); index++ {
			_ = f.LocalUnlockDye(index)
		}
	}
	p.LocalCompleteStory()
	p.LocalUnlockMaps()
	p.LocalCompleteCollections()
	p.LocalNormalizeUnlockItems()
	p.LocalUnlockGatherTools()
	return p.LocalUnlockSummary()
}
