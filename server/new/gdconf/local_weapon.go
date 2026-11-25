package gdconf

import (
	"gucooing/lolo/protocol/excel"
	"sort"
)

// LocalWeaponRange follows the client's one-based property-group index.
type LocalWeaponRange struct {
	Min            uint32   `json:"min"`
	Max            uint32   `json:"max"`
	PropertyIndex  uint32   `json:"propertyIndex"`
	StrengthLevels []uint32 `json:"strengthLevels"`
}

func localWeaponStrengthLevels(group *excel.WeaponPropertyGroupInfo) []uint32 {
	levels := []uint32{0}
	if group == nil || group.StrengthenID == 0 {
		return levels
	}
	for _, row := range cc.Excel.Weapon.all.GetWeaponStrengthen().GetDatas() {
		if row.ID != group.StrengthenID {
			continue
		}
		seen := map[uint32]bool{0: true}
		for _, v := range row.GetWeaponStrengthenGroupInfo() {
			if v != nil && v.Level > 0 && !seen[uint32(v.Level)] {
				levels = append(levels, uint32(v.Level))
				seen[uint32(v.Level)] = true
			}
		}
		break
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	return levels
}

func LocalWeaponRanges(id uint32) []LocalWeaponRange {
	conf := GetWeaponAllInfo(id)
	ranges := []LocalWeaponRange{}
	if conf == nil || conf.WeaponInfo == nil {
		return ranges
	}
	indices := []int{}
	for index := range conf.PropertyGroup {
		indices = append(indices, int(index))
	}
	sort.Ints(indices)
	for _, i := range indices {
		group := conf.PropertyGroup[uint32(i)]
		if group == nil || group.MinLevel < 1 || group.MaxLevel < group.MinLevel {
			continue
		}
		ranges = append(ranges, LocalWeaponRange{uint32(group.MinLevel), uint32(group.MaxLevel), uint32(i), localWeaponStrengthLevels(group)})
	}
	return ranges
}

func LocalWeaponRangeForLevel(id, level uint32) (LocalWeaponRange, bool) {
	for _, r := range LocalWeaponRanges(id) {
		if level >= r.Min && level <= r.Max {
			return r, true
		}
	}
	return LocalWeaponRange{}, false
}
