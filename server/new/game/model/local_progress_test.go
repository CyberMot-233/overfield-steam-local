package model

import (
	"encoding/json"
	"gucooing/lolo/config"
	"gucooing/lolo/gdconf"
	"gucooing/lolo/pkg/log"
	"gucooing/lolo/protocol/proto"
	"os"
	"sync"
	"testing"
)

var localConfigOnce sync.Once

func localTestConfig(t *testing.T) {
	t.Helper()
	file := os.Getenv("LOLO_LOCAL_CONFIG")
	if file == "" {
		t.Skip("set LOLO_LOCAL_CONFIG to a local profile config for client-table integration tests")
	}
	localConfigOnce.Do(func() {
		config.CfgPath = file
		if e := config.LoadConfig(); e != nil {
			t.Fatal(e)
		}
		log.NewApp()
		log.NewGame()
		gdconf.LoadGameConfig()
	})
}
func TestLocalProgressUsesConfiguredScenes(t *testing.T) {
	localTestConfig(t)
	p := new(Player)
	p.LocalCompleteStory()
	p.LocalCompleteCollections()
	before, _ := json.Marshal(p)
	if n := p.LocalCompleteStory() + p.LocalCompleteCollections(); n != 0 {
		t.Fatal("not idempotent", n)
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("reapplying changed save")
	}
	for _, v := range p.LocalSceneProcesses() {
		conf := gdconf.GetLocalCatalog().Scenes[v.SceneId]
		if conf == nil || v.Process != conf.Total {
			t.Fatal("incorrect scene total", v)
		}
	}
	for _, id := range gdconf.LocalSceneIDs() {
		data := &proto.SceneData{SceneId: id}
		p.LocalFillSceneData(data)
		conf := gdconf.GetLocalCatalog().Scenes[id]
		for _, group := range data.Collections {
			for key := range group.ItemMap {
				if !conf.Collections[key] {
					t.Fatal("cross-scene collection", id, key)
				}
			}
		}
	}
	if p.GetSceneModel().GetWorldLevel() == 0 {
		t.Fatal("invalid world ID")
	}
	for _, q := range p.GetQuestDetail().Quests {
		if gdconf.GetQuestInfos()[q.QuestId] == nil {
			t.Fatal("unknown quest")
		}
	}
}
func TestLocalDyeSaveCancelAndRoundTrip(t *testing.T) {
	localTestConfig(t)
	f := &ItemFashionInfo{DyeSchemes: map[uint32]*OutfitDyeScheme{0: {SchemeIndex: 0, IsUnLock: true}}}
	color := &proto.PosColor{Pos: 2, Red: 12, Green: 77, Blue: 200}
	if f.LocalStageDye(1, color) == nil {
		t.Fatal("locked scheme accepted")
	}
	if e := f.LocalUnlockDye(1); e != nil {
		t.Fatal(e)
	}
	if e := f.LocalStageDye(1, color); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(f)
	var restored ItemFashionInfo
	if e := json.Unmarshal(b, &restored); e != nil {
		t.Fatal(e)
	}
	saved, e := restored.LocalSaveDye(1, 2, true)
	if e != nil || saved.Red != 12 || saved.Green != 77 || saved.Blue != 200 {
		t.Fatal(saved, e)
	}
	if e = restored.LocalStageDye(1, &proto.PosColor{Pos: 2, Red: 255}); e != nil {
		t.Fatal(e)
	}
	cancel, e := restored.LocalSaveDye(1, 2, false)
	if e != nil || cancel.Red != 12 || cancel.Green != 77 || cancel.Blue != 200 {
		t.Fatal("cancel overwrote saved color", cancel, e)
	}
	if restored.LocalStageDye(1, &proto.PosColor{Red: 256}) == nil {
		t.Fatal("invalid RGB accepted")
	}
	if len(restored.PendingDyes) != 0 {
		t.Fatal("preview not cleared")
	}
	var hide *OutfitHideInfo
	if hide.OutfitHideInfo() == nil {
		t.Fatal("missing default visibility")
	}
}
