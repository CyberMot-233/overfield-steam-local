package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gucooing/lolo/gdconf"
	"math"
	"sort"
)

var ErrLocalWeaponConflict = errors.New("武器已在游戏中发生变化，请重新读取后再保存")

const LocalWeaponMaxAttack uint32 = 1000000000
const LocalWeaponMaxPercent uint32 = 10000

type LocalWeaponEdit struct {
	InstanceID uint32             `json:"instanceId"`
	WeaponID   uint32             `json:"weaponId"`
	Revision   string             `json:"revision"`
	Changes    map[string]float64 `json:"changes"`
}
type LocalWeaponView struct {
	InstanceID    uint32                    `json:"instanceId"`
	WeaponID      uint32                    `json:"weaponId"`
	Revision      string                    `json:"revision"`
	Level         uint32                    `json:"level"`
	Attack        float64                   `json:"attack"`
	DamageBalance float64                   `json:"damageBalance"`
	CriticalRatio float64                   `json:"criticalRatio"`
	StrengthLevel uint32                    `json:"strengthLevel"`
	WearerID      uint32                    `json:"wearerId"`
	WearerIndex   uint32                    `json:"wearerIndex"`
	IsLock        bool                      `json:"isLock"`
	Ranges        []gdconf.LocalWeaponRange `json:"ranges"`
	MaxAttack     uint32                    `json:"maxAttack"`
	MaxPercent    uint32                    `json:"maxPercent"`
}

func LocalWeaponRevision(w *ItemWeaponInfo) string {
	b, _ := json.Marshal(w)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func LocalWeaponDisplay(w *ItemWeaponInfo) map[string]float64 {
	values := map[string]float64{"attack": float64(w.Attack), "damageBalance": float64(w.DamageBalance), "criticalRatio": float64(w.CriticalRatio)}
	conf := gdconf.GetWeaponAllInfo(w.WeaponId)
	if conf != nil {
		g := conf.PropertyGroup[w.PropertyIndex]
		if g != nil {
			rate := 0.0
			if g.MaxLevel > g.MinLevel {
				rate = (float64(w.Level) - float64(g.MinLevel)) / float64(g.MaxLevel-g.MinLevel)
			}
			lerp := func(a, b float32) float64 { return float64(a) + (float64(b)-float64(a))*rate }
			roll := func(value uint32) float64 {
				return float64(g.MinFloatValue) + (float64(g.MaxFloatValue)-float64(g.MinFloatValue))*float64(value)/100
			}
			values["attack"] = math.Round(lerp(g.MinAttack, g.MaxAttack) * roll(w.Attack))
			values["criticalRatio"] = math.Round(lerp(g.MinCriticalRatio, g.MaxCriticalRatio)*roll(w.CriticalRatio)*10000) / 100
			values["damageBalance"] = math.Round(lerp(g.MinDamageBalance, g.MaxDamageBalance)*roll(w.DamageBalance)*10000) / 100
		}
	}
	for name, value := range w.LocalStats {
		values[name] = value
	}
	return values
}
func LocalViewWeapon(w *ItemWeaponInfo) LocalWeaponView {
	v := LocalWeaponDisplay(w)
	return LocalWeaponView{w.InstanceId, w.WeaponId, LocalWeaponRevision(w), w.Level, v["attack"], v["damageBalance"], v["criticalRatio"], w.StrengthLevel, w.WearerId, w.WearerIndex, w.IsLock, gdconf.LocalWeaponRanges(w.WeaponId), LocalWeaponMaxAttack, LocalWeaponMaxPercent}
}
func (p *Player) LocalWeaponList() []LocalWeaponView {
	list := []LocalWeaponView{}
	for _, w := range p.GetItemModel().GetItemWeaponMap() {
		if w != nil {
			list = append(list, LocalViewWeapon(w))
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].InstanceID < list[j].InstanceID })
	return list
}

// Validate every field before replacing the instance. Unspecified fields, affixes,
// inscription, durability, lock state and all equipment references are preserved.
func (p *Player) LocalEditWeapon(req LocalWeaponEdit) (*ItemWeaponInfo, error) {
	w := p.GetItemModel().GetItemWeaponInfo(req.InstanceID)
	if w == nil || w.InstanceId != req.InstanceID || w.WeaponId != req.WeaponID {
		return nil, fmt.Errorf("武器实例不存在或物品 ID 不匹配")
	}
	if req.Revision == "" || req.Revision != LocalWeaponRevision(w) {
		return nil, ErrLocalWeaponConflict
	}
	if len(req.Changes) == 0 {
		return nil, fmt.Errorf("没有需要修改的武器字段")
	}
	next := *w
	next.LocalEdited = true
	next.LocalStats = map[string]float64{}
	for key, value := range w.LocalStats {
		next.LocalStats[key] = value
	}
	for name, value := range req.Changes {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return nil, fmt.Errorf("武器数值必须为有限的非负数")
		}
		switch name {
		case "resetStats":
			if value != 1 || len(req.Changes) != 1 {
				return nil, fmt.Errorf("恢复配置属性必须单独提交")
			}
			next.LocalStats = map[string]float64{}
		case "level", "strengthLevel":
			if value != math.Trunc(value) || value > 4294967295 {
				return nil, fmt.Errorf("等级必须为非负整数")
			}
			if name == "level" {
				next.Level = uint32(value)
			} else {
				next.StrengthLevel = uint32(value)
			}
		case "attack":
			if value > float64(LocalWeaponMaxAttack) || value != math.Trunc(value) {
				return nil, fmt.Errorf("攻击力必须为 0..%d 的整数", LocalWeaponMaxAttack)
			}
			next.LocalStats[name] = value
		case "damageBalance", "criticalRatio":
			if value > float64(LocalWeaponMaxPercent) || math.Abs(value*100-math.Round(value*100)) > 0.000001 {
				return nil, fmt.Errorf("百分比范围为 0..%d，最多两位小数", LocalWeaponMaxPercent)
			}
			next.LocalStats[name] = value
		default:
			return nil, fmt.Errorf("不支持的武器字段 %q", name)
		}
	}
	group, ok := gdconf.LocalWeaponRangeForLevel(next.WeaponId, next.Level)
	if !ok {
		return nil, fmt.Errorf("等级 %d 在这把武器的客户端配置中不存在", next.Level)
	}
	validStrength := false
	for _, level := range group.StrengthLevels {
		if level == next.StrengthLevel {
			validStrength = true
			break
		}
	}
	if !validStrength {
		return nil, fmt.Errorf("当前等级只支持强化等级 %v", group.StrengthLevels)
	}
	next.PropertyIndex = group.PropertyIndex
	if next.StrengthLevel != w.StrengthLevel {
		next.StrengthExp = 0
	}
	*w = next
	return w, nil
}
