package model

import (
	"fmt"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
	"strconv"
)

func LocalDyeSchemeCount() uint32 {
	v := gdconf.GetGlobalConfigConfigure("Fashion_ColoringPlan_Count")
	if v != nil && len(v.Values) > 0 {
		n, e := strconv.ParseUint(v.Values[0], 10, 32)
		if e == nil && n > 0 && n <= 32 {
			return uint32(n)
		}
	}
	return 5
}
func (f *ItemFashionInfo) LocalUnlockDye(index uint32) error {
	if f == nil || index >= LocalDyeSchemeCount() {
		return fmt.Errorf("invalid dye scheme")
	}
	if f.DyeSchemes == nil {
		f.DyeSchemes = map[uint32]*OutfitDyeScheme{}
	}
	if f.DyeSchemes[index] == nil {
		f.DyeSchemes[index] = &OutfitDyeScheme{SchemeIndex: index, Colors: Colors{}}
	}
	f.DyeSchemes[index].IsUnLock = true
	return nil
}
func (f *ItemFashionInfo) LocalStageDye(index uint32, color *proto.PosColor) error {
	if f == nil || color == nil || color.Pos > 63 || color.Red > 255 || color.Green > 255 || color.Blue > 255 {
		return fmt.Errorf("invalid color")
	}
	scheme := f.GetDyeScheme(index)
	if scheme == nil || !scheme.IsUnLock {
		return fmt.Errorf("scheme locked")
	}
	if f.PendingDyes == nil {
		f.PendingDyes = map[uint32]*PosColor{}
	}
	f.PendingDyes[index*64+color.Pos] = &PosColor{Pos: color.Pos, Red: color.Red, Green: color.Green, Blue: color.Blue}
	return nil
}
func (f *ItemFashionInfo) LocalSaveDye(index, pos uint32, save bool) (*proto.PosColor, error) {
	if f == nil || pos > 63 || index >= LocalDyeSchemeCount() {
		return nil, fmt.Errorf("invalid dye position")
	}
	scheme := f.GetDyeScheme(index)
	if scheme == nil || !scheme.IsUnLock {
		return nil, fmt.Errorf("scheme locked")
	}
	key := index*64 + pos
	pending := f.PendingDyes[key]
	if save && pending == nil {
		return nil, fmt.Errorf("no dye preview to save")
	}
	if save {
		found := false
		for i, c := range scheme.Colors {
			if c.Pos == pos {
				scheme.Colors[i] = pending
				found = true
				break
			}
		}
		if !found {
			scheme.Colors = append(scheme.Colors, pending)
		}
	}
	delete(f.PendingDyes, key)
	for _, c := range scheme.Colors {
		if c.Pos == pos {
			return &proto.PosColor{Pos: c.Pos, Red: c.Red, Green: c.Green, Blue: c.Blue}, nil
		}
	}
	return &proto.PosColor{Pos: pos}, nil
}
