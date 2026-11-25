package gdconf

import "gucooing/lolo/protocol/excel"

// LocalItemHasIcon requires all three icon sizes used by inventory and detail panels.
func LocalItemHasIcon(id uint32) bool {
	item := GetItemConfigure(id)
	if item == nil {
		return false
	}
	catalog := GetLocalCatalog()
	if catalog == nil || catalog.ItemIcons == nil {
		return true
	}
	icons := catalog.ItemIcons[uint32(item.IconID)]
	if len(icons) < 3 {
		return false
	}
	for _, icon := range icons {
		if icon == "" {
			return false
		}
	}
	return true
}

func LocalArmorProperty(id, level uint32) (*excel.ArmorPropertyGroupInfo, uint32) {
	conf := GetArmorAllInfo(id)
	if conf == nil || conf.ArmorInfo == nil {
		return nil, 0
	}
	for _, row := range cc.Excel.Armor.all.GetArmorProperty().GetDatas() {
		if row.ID != conf.ArmorInfo.ArmorPropertyID {
			continue
		}
		for index, group := range row.ArmorPropertyGroupInfo {
			if group != nil && level >= uint32(group.MinLevel) && level <= uint32(group.MaxLevel) {
				return group, uint32(index + 1)
			}
		}
	}
	return nil, 0
}
