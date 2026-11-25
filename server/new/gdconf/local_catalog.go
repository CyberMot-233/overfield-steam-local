package gdconf

// Local catalogs come from the installed client's tables. They contain identifiers,
// never invented scenes or task IDs. Missing optional tables keep old profiles usable.
import (
	"encoding/json"
	"fmt"
	"gucooing/lolo/protocol/excel"
	"os"
	"sort"
)

type LocalArea struct {
	ID    uint32
	Level uint32
}
type LocalScene struct {
	ID          uint32
	Areas       []LocalArea
	Total       uint32
	Collections map[uint32]bool
	Riddles     map[uint32]bool
	Levels      map[uint32]uint32
}
type LocalClientAssets struct {
	CharacterAssets map[uint32]bool `json:"characterAssets"`
}
type LocalCatalog struct {
	Expression   *excel.AllExpressionDatas
	Action       *excel.AllActionDatas
	BlessingTree *excel.AllBlessingTreeDatas
	ClientAssets *LocalClientAssets
	ItemIcons    map[uint32][]string
	Dye          *excel.AllDyeStuffDatas
	Level        *excel.AllLevelDatas
	Area         *excel.AllAreaDatas
	Scene        *excel.AllSceneDatas
	Challenge    *excel.AllChallengeDatas
	Riddle       *excel.AllRiddleDatas
	Daily        *excel.AllDailyTaskDatas
	Scenes       map[uint32]*LocalScene
}

func (g *GameConfig) loadLocalCatalog() {
	c := &LocalCatalog{Dye: new(excel.AllDyeStuffDatas), Level: new(excel.AllLevelDatas), Area: new(excel.AllAreaDatas), Scene: new(excel.AllSceneDatas), Challenge: new(excel.AllChallengeDatas), Riddle: new(excel.AllRiddleDatas), Daily: new(excel.AllDailyTaskDatas), Scenes: map[uint32]*LocalScene{}}
	read := func(name string, dst any) {
		b, e := os.ReadFile(g.excelPath + name + ".json")
		if os.IsNotExist(e) {
			return
		}
		if e != nil {
			panic(e)
		}
		if e = json.Unmarshal(b, dst); e != nil {
			panic(fmt.Errorf("%s: %w", name, e))
		}
	}
	if b, e := os.ReadFile(g.configPath + "LocalClientAssets.json"); e == nil {
		if e = json.Unmarshal(b, &c.ClientAssets); e != nil {
			panic(e)
		}
	} else if !os.IsNotExist(e) {
		panic(e)
	}
	c.Expression = new(excel.AllExpressionDatas)
	c.Action = new(excel.AllActionDatas)
	c.BlessingTree = new(excel.AllBlessingTreeDatas)
	read("Expression", c.Expression)
	read("Action", c.Action)
	read("BlessingTree", c.BlessingTree)
	read("DyeStuff", c.Dye)
	read("Level", c.Level)
	read("Area", c.Area)
	read("Scene", c.Scene)
	read("Challenge", c.Challenge)
	read("Riddle", c.Riddle)
	read("DailyTask", c.Daily)
	for id := range g.Config.SceneConfig.SceneMap {
		c.Scenes[id] = &LocalScene{ID: id, Collections: map[uint32]bool{}, Riddles: map[uint32]bool{}, Levels: map[uint32]uint32{}}
	}
	for _, sc := range c.Scene.GetScene().GetDatas() {
		entry := c.Scenes[uint32(sc.ID)]
		if entry == nil {
			continue
		}
		for _, a := range c.Area.GetAreaData().GetDatas() {
			if a.ID != sc.AreaID {
				continue
			}
			for _, v := range a.AreaDataGroup {
				max := uint32(1)
				for _, lv := range c.Area.GetAreaLevelData().GetDatas() {
					if lv.ID == v.AreaLevelID {
						for _, row := range lv.AreaLevelDataGroup {
							if uint32(row.Level) > max {
								max = uint32(row.Level)
							}
						}
					}
				}
				entry.Areas = append(entry.Areas, LocalArea{ID: uint32(v.AreaID), Level: max})
			}
		}
	}

	// Match collectable IDs to scenes through the explicit area relation table.
	prefixes := map[uint32]map[uint32]bool{}
	for _, sc := range c.Scene.GetScene().GetDatas() {
		entry := c.Scenes[uint32(sc.ID)]
		if entry == nil {
			continue
		}
		for _, pr := range c.Scene.GetSceneProgress().GetDatas() {
			if pr.ID == sc.ProgressID {
				for _, v := range pr.SceneProgressInfo {
					entry.Total += uint32(v.Total)
				}
			}
		}
		for _, ar := range c.Area.GetAreaData().GetDatas() {
			if ar.ID != sc.AreaID {
				continue
			}
			for _, a := range ar.AreaDataGroup {
				for _, rel := range c.Area.GetAreaRelationData().GetDatas() {
					if rel.ID != a.RelevancePoolID {
						continue
					}
					for _, r := range rel.AreaRelationDataGroup {
						for _, v := range []int32{r.CollectFileID, r.CollectItemID, r.CollectBookID, r.CollectTapeID, r.CollectPortalID, r.CollectDataID} {
							if v <= 0 {
								continue
							}
							id := uint32(v)
							entry.Collections[id] = true
							prefix := id / 1000 % 100
							if prefixes[prefix] == nil {
								prefixes[prefix] = map[uint32]bool{}
							}
							prefixes[prefix][entry.ID] = true
						}
						if r.RiddleID > 0 {
							entry.Riddles[uint32(r.RiddleID)] = true
						}
					}
				}
			}
		}
	}
	// All linked collection categories use the same scene segment in their IDs.
	// A prefix is accepted only when the configured area relations identify one scene.
	for id, v := range g.Excel.CollectionItem.ItemMap {
		owners := prefixes[id/1000%100]
		if len(owners) != 1 || v.NewCollectionType < 1 || v.NewCollectionType > 9 {
			continue
		}
		for scene := range owners {
			c.Scenes[scene].Collections[id] = true
		}
	}
	var levels map[string]struct{ Datas []map[string]json.RawMessage }
	read("Level", &levels)
	names := map[string]uint32{"FileLevel": 1, "CollectItemLevel": 2, "BookLevel": 3, "TapeLevel": 4, "PortalLevel": 5, "DataLevel": 6, "PhotoLevel": 7, "CrystalLevel": 8, "PhotoItemLevel": 9}
	for name, t := range names {
		for _, row := range levels[name].Datas {
			var id uint32
			_ = json.Unmarshal(row["ID"], &id)
			sc := c.Scenes[id]
			if sc == nil {
				continue
			}
			var infos []struct{ Level uint32 }
			_ = json.Unmarshal(row[name+"Info"], &infos)
			for _, v := range infos {
				if v.Level > sc.Levels[t] {
					sc.Levels[t] = v.Level
				}
			}
		}
	}
	var ui struct {
		Icon struct {
			Datas []struct {
				ID            uint32
				AssetFullName []string
			}
		}
	}
	read("UI", &ui)
	if ui.Icon.Datas != nil {
		c.ItemIcons = make(map[uint32][]string)
		for _, icon := range ui.Icon.Datas {
			c.ItemIcons[icon.ID] = icon.AssetFullName
		}
	}
	g.Local = c
}
func GetLocalCatalog() *LocalCatalog { return cc.Local }
func LocalSceneIDs() []uint32 {
	out := []uint32{}
	for id := range GetLocalCatalog().Scenes {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
func ValidWorldLevel(id uint32) bool {
	for _, v := range GetLocalCatalog().Level.GetWorldLevel().GetDatas() {
		if uint32(v.ID) == id {
			return true
		}
	}
	return false
}
func DefaultWorldLevel() uint32 {
	ids := GetLocalCatalog().Level.GetWorldLevel().GetDatas()
	if len(ids) > 0 {
		return uint32(ids[0].ID)
	}
	return 1
}
func GetAllCollectionItems() map[uint32]*excel.CollectionItemConfigure {
	return cc.Excel.CollectionItem.ItemMap
}
