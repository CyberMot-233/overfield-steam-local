// Offline save editor. Uses the exact Lolo v26.5 model constructors.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gucooing/lolo/config"
	"gucooing/lolo/game/model"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/pkg/log"
	"gucooing/lolo/protocol/proto"
	"os"
	"sort"
)

type Request struct {
	Weapon     *model.LocalWeaponEdit
	Player     json.RawMessage
	Operation  string
	ID         uint32
	Count      int64
	Level      uint32
	Star       uint32
	SkillLevel uint32
	WorldLevel uint32
	Team       []uint32
}
type Result struct {
	Unlocks         map[string]int
	Weapons         []model.LocalWeaponView
	Player          *model.Player
	Progress        map[string]int
	InventoryRepair model.LocalInventoryRepair
	Summary         map[string]int
	Applied         int
	Skipped         []uint32
	Unavailable     []uint32
}

func summary(p *model.Player) map[string]int {
	i := p.GetItemModel()
	return map[string]int{"characters": len(p.GetCharacterModel().GetCharacterMap()), "resources": len(i.ItemBaseInfo), "skins": len(i.ItemFashionMap), "weapons": len(i.ItemWeaponMap), "armor": len(i.ItemArmorMap), "posters": len(i.ItemPosterMap), "inscriptions": len(i.ItemInscriptionMap), "pets": len(i.ItemPetInfoMap), "heads": len(i.ItemHeadMap)}
}
func power(p *model.Player, id uint32, level, star, skill uint32) error {
	c := p.GetCharacterModel().GetCharacterInfo(id)
	conf := gdconf.GetCharacterAll(id)
	if c == nil || conf == nil {
		return fmt.Errorf("character %d not owned", id)
	}
	if level < 1 || level > 200 || star > 20 || skill < 1 || skill > 20 {
		return fmt.Errorf("invalid character settings")
	}
	maxLevel := uint32(20)
	for _, r := range conf.LevelRules {
		if uint32(r.TopMaxLevel) > maxLevel {
			maxLevel = uint32(r.TopMaxLevel)
		}
	}
	if level > maxLevel {
		level = maxLevel
	}
	c.BreakLevel = uint32(len(conf.LevelRules))
	c.UpMaxLevel()
	c.Level = level
	c.Exp = 0
	maxStar := uint32(0)
	for s := uint32(0); s <= 20; s++ {
		if gdconf.GetCharacterStar(id, s) != nil {
			maxStar = s
		}
	}
	if star > maxStar {
		star = maxStar
	}
	c.Star = star
	for _, s := range c.CharacterSkillList {
		s.SkillLevel = skill
	}
	c.IsUnlockPayment = true
	return nil
}
func run() error {
	conf := flag.String("config", "", "profile config")
	in := flag.String("input", "", "request JSON")
	out := flag.String("output", "", "result JSON")
	flag.Parse()
	if *conf == "" || *in == "" || *out == "" {
		return fmt.Errorf("config,input,output required")
	}
	config.CfgPath = *conf
	if err := config.LoadConfig(); err != nil {
		return err
	}
	log.NewApp()
	log.NewGame()
	defer log.Close()
	gdconf.LoadGameConfig()
	b, e := os.ReadFile(*in)
	if e != nil {
		return e
	}
	var req Request
	if e = json.Unmarshal(b, &req); e != nil {
		return e
	}
	p := new(model.Player)
	if len(req.Player) > 0 && string(req.Player) != "null" {
		if e = json.Unmarshal(req.Player, p); e != nil {
			return e
		}
	}
	if len(p.GetCharacterModel().GetCharacterMap()) == 0 {
		p.AddCharacter(101001)
		p.GetItemModel().InitItem()
	}
	result := Result{Player: p, Skipped: []uint32{}, Unavailable: []uint32{}}
	if req.Count == 0 {
		req.Count = 9999999
	}
	if req.Count < 1 || req.Count > 999999999 {
		return fmt.Errorf("count must be 1..999999999")
	}
	if req.Operation != "weapons" && req.Operation != "weapon-edit" {
		result.InventoryRepair = p.LocalRepairInventory()
		p.LocalNormalizeUnlockItems()
		p.GetSceneModel().GetWorldLevel()
	}
	switch req.Operation {
	case "unlock-all":
		result.Unlocks = p.LocalUnlockAll()
		for _, n := range result.Unlocks {
			result.Applied += n
		}
	case "weapons":
		result.Weapons = p.LocalWeaponList()
	case "weapon-edit":
		if req.Weapon == nil {
			return fmt.Errorf("weapon edit request required")
		}
		if _, err := p.LocalEditWeapon(*req.Weapon); err != nil {
			return err
		}
		result.Applied = 1
		result.Weapons = p.LocalWeaponList()
	case "preset", "resources", "characters", "skins":
		ids := []int{}
		for _, conf := range gdconf.GetAllItemConfigure() {
			ids = append(ids, int(conf.ID))
		}
		sort.Ints(ids)
		for _, raw := range ids {
			id := uint32(raw)
			c := gdconf.GetItemConfigure(id)
			if c == nil {
				result.Skipped = append(result.Skipped, id)
				continue
			}
			tag := proto.EBagItemTag(c.NewBagItemTag)
			if req.Operation == "characters" && tag != proto.EBagItemTag_EBagItemTag_Card {
				continue
			}
			if req.Operation == "skins" && tag != proto.EBagItemTag_EBagItemTag_Fashion {
				continue
			}
			if req.Operation == "resources" && (tag == proto.EBagItemTag_EBagItemTag_Card || tag == proto.EBagItemTag_EBagItemTag_Fashion) {
				continue
			}
			if !model.LocalItemAvailable(id) {
				result.Unavailable = append(result.Unavailable, id)
				continue
			}
			count := req.Count
			if tag == proto.EBagItemTag_EBagItemTag_Fashion {
				count = 1
			}
			if p.LocalGrantItem(id, count, true) {
				result.Applied++
			} else {
				result.Skipped = append(result.Skipped, id)
			}
		}
		if req.Operation == "preset" {
			for id := range p.GetCharacterModel().GetCharacterMap() {
				if e := power(p, id, 200, 20, 1); e != nil {
					return e
				}
			}
		}
	case "item":
		if !p.LocalGrantItem(req.ID, req.Count, false) {
			return fmt.Errorf("item %d unsupported by this server model", req.ID)
		}
		result.Applied = 1
	case "character":
		if p.GetCharacterModel().GetCharacterInfo(req.ID) == nil {
			if p.AddCharacter(req.ID) == nil {
				return fmt.Errorf("invalid character ID")
			}
		}
		if e := power(p, req.ID, req.Level, req.Star, req.SkillLevel); e != nil {
			return e
		}
		result.Applied = 1
	case "progress-story":
		result.Applied = p.LocalCompleteStory()
	case "progress-maps":
		result.Applied = p.LocalUnlockMaps()
	case "progress-collections":
		result.Applied = p.LocalCompleteCollections()
	case "progress-all":
		result.Applied = p.LocalCompleteStory() + p.LocalCompleteCollections()
	case "world":
		if !gdconf.ValidWorldLevel(req.WorldLevel) {
			return fmt.Errorf("invalid world-level configuration ID")
		}
		p.GetSceneModel().WorldLevel = req.WorldLevel
	case "team":
		if len(req.Team) != 3 {
			return fmt.Errorf("three character IDs required")
		}
		seen := map[uint32]bool{}
		for idx, id := range req.Team {
			if idx == 0 && id == 0 {
				return fmt.Errorf("leader required")
			}
			if id != 0 {
				if p.GetCharacterModel().GetCharacterInfo(id) == nil || seen[id] {
					return fmt.Errorf("team has duplicate or unowned character")
				}
				seen[id] = true
			}
		}
		t := p.GetTeamModel().GetTeamInfo()
		t.Char1 = req.Team[0]
		t.Char2 = req.Team[1]
		t.Char3 = req.Team[2]
	case "init", "validate", "repair-abilities", "repair-inventory":
	default:
		return fmt.Errorf("unknown operation %q", req.Operation)
	}
	if req.Operation == "preset" {
		ids := []int{}
		for id := range p.GetCharacterModel().GetCharacterMap() {
			ids = append(ids, int(id))
		}
		sort.Ints(ids)
		t := p.GetTeamModel().GetTeamInfo()
		if len(ids) >= 3 {
			t.Char1 = uint32(ids[0])
			t.Char2 = uint32(ids[1])
			t.Char3 = uint32(ids[2])
		}
	}
	// Exercise the same protobuf conversion paths used at login before writing a save.
	p.GetCharacterModel().GetAllPbCharacter()
	p.GetTeamModel().GetTeamInfo().Team()
	p.GetQuestDetail()
	p.GetUnlockFunctions()
	result.Summary = summary(p)
	result.Progress = p.LocalProgressSummary()
	b, e = json.Marshal(result)
	if e != nil {
		return e
	}
	return os.WriteFile(*out, b, 0600)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
