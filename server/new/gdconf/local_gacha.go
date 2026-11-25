package gdconf

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// LocalGachaConfig is an explicit offline schedule, separate from official Excel data.
type LocalGachaConfig struct {
	Mode        string   `json:"mode"`
	PoolIDs     []uint32 `json:"poolIds"`
	OpeningTime string   `json:"openingTime"`
	ClosingTime string   `json:"closingTime"`
}

func (g *GameConfig) loadLocalGacha() {
	raw, err := os.ReadFile(g.configPath + "LocalGacha.json")
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		panic(err)
	}
	var conf LocalGachaConfig
	if err = json.Unmarshal(raw, &conf); err != nil {
		panic(err)
	}
	if err = g.applyLocalGacha(conf); err != nil {
		panic(err)
	}
}

func (g *GameConfig) applyLocalGacha(c LocalGachaConfig) error {
	if c.Mode != "all-available" || len(c.PoolIDs) == 0 || len(c.PoolIDs) > 200 {
		return fmt.Errorf("invalid local gacha mode or pool count")
	}
	begin, e1 := time.Parse("2006-01-02 15:04:05", c.OpeningTime)
	end, e2 := time.Parse("2006-01-02 15:04:05", c.ClosingTime)
	if e1 != nil || e2 != nil || !begin.Before(end) {
		return fmt.Errorf("invalid local gacha period")
	}
	selected := make([]*GachaData, 0, len(c.PoolIDs))
	seen := map[uint32]bool{}
	for _, id := range c.PoolIDs {
		data := g.Excel.Gacha.GachaDatas[id]
		if seen[id] || data == nil || data.Conf == nil {
			return fmt.Errorf("invalid local pool %d", id)
		}
		seen[id] = true
		selected = append(selected, data)
	}
	// Validate everything before changing the active list.
	for index, data := range selected {
		data.Conf.OpeningTime = c.OpeningTime
		data.Conf.ClosingTime = c.ClosingTime
		data.Conf.Index = int32(index + 1)
	}
	g.Excel.Gacha.OpenList = selected
	return nil
}

func IsOpenGachaID(id uint32) bool {
	for _, pool := range GetOpenGachas() {
		if uint32(pool.Conf.ID) == id {
			return true
		}
	}
	return false
}
