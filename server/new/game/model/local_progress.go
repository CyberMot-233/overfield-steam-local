package model

import (
	"gucooing/lolo/gdconf"
	"gucooing/lolo/protocol/proto"
	"sort"
	"time"
)

type LocalProgress struct {
	Version         int                     `json:"version"`
	StoryComplete   bool                    `json:"storyComplete,omitempty"`
	Quests          map[uint32]*proto.Quest `json:"quests,omitempty"`
	Maps            map[uint32]bool         `json:"maps,omitempty"`
	Collections     map[uint32]bool         `json:"collections,omitempty"`
	CompletedScenes map[uint32]bool         `json:"completedScenes,omitempty"`
	DailyDate       string                  `json:"dailyDate,omitempty"`
	DailyConverted  uint32                  `json:"dailyConverted,omitempty"`
}

func (s *Player) LocalProgress() *LocalProgress {
	if s.Progress == nil {
		s.Progress = &LocalProgress{Version: 1}
	}
	p := s.Progress
	if p.Quests == nil {
		p.Quests = map[uint32]*proto.Quest{}
	}
	if p.Maps == nil {
		p.Maps = map[uint32]bool{}
	}
	if p.Collections == nil {
		p.Collections = map[uint32]bool{}
	}
	if p.CompletedScenes == nil {
		p.CompletedScenes = map[uint32]bool{}
	}
	return p
}
func CompletedLocalQuest(id uint32) *proto.Quest {
	conf := gdconf.GetQuestInfos()[id]
	if conf == nil {
		return nil
	}
	q := &proto.Quest{QuestId: id, Status: proto.QuestStatus_QuestStatus_Finish, CompleteCount: 1, BonusTimes: 1, Conditions: []*proto.Condition{}}
	if conf.ConditionSetGroup != nil {
		for _, set := range conf.ConditionSetGroup.QuestConditionSet {
			for _, raw := range set.AchieveConditionID {
				id := uint32(raw)
				count := uint32(1)
				if a := gdconf.GetAchieveConfigure(id); a != nil && a.CountParam > 0 {
					count = uint32(a.CountParam)
				}
				q.Conditions = append(q.Conditions, &proto.Condition{ConditionId: id, Progress: count, Status: proto.QuestStatus_QuestStatus_Finish})
			}
		}
	}
	return q
}

// Complete narrative tasks only; daily/repeating/event tasks retain their lifecycle.
func (s *Player) LocalCompleteStory() int {
	p := s.LocalProgress()
	n := 0
	for id, c := range gdconf.GetQuestInfos() {
		switch proto.EQuestType(c.Config.NewType) {
		case proto.EQuestType_EQuestType_MainQuest, proto.EQuestType_EQuestType_SideQuest, proto.EQuestType_EQuestType_WorldQuest, proto.EQuestType_EQuestType_ExtraQuest:
			if p.Quests[id] == nil {
				n++
			}
			p.Quests[id] = CompletedLocalQuest(id)
		}
	}
	p.StoryComplete = true
	return n
}
func (s *Player) LocalUnlockMaps() int {
	p := s.LocalProgress()
	n := 0
	for _, id := range gdconf.LocalSceneIDs() {
		if !p.Maps[id] {
			n++
		}
		p.Maps[id] = true
		areas := s.GetSceneModel().GetSceneInfo(id).GetAreaDatas()
		for _, a := range gdconf.GetLocalCatalog().Scenes[id].Areas {
			areas[a.ID] = &AreaData{AreaId: a.ID, AreaState: proto.AreaState_AreaState_Unlock, Level: a.Level}
		}
	}
	s.GetSceneModel().GetWorldLevel()
	return n
}
func (s *Player) LocalCompleteCollections() int {
	p := s.LocalProgress()
	n := 0
	s.LocalUnlockMaps()
	for id, c := range gdconf.GetAllCollectionItems() {
		if c.NewCollectionType < 1 || c.NewCollectionType > 9 {
			continue
		}
		if !p.Collections[id] {
			n++
		}
		p.Collections[id] = true
	}
	for _, id := range gdconf.LocalSceneIDs() {
		p.CompletedScenes[id] = true
		boxes := s.GetSceneModel().GetSceneInfo(id).GetTreasurBoxs()
		for _, b := range gdconf.GetSceneInfo(id).Info.CollectionTreasureInfos {
			key := uint32(b.ID)
			boxes[key] = &TreasureBox{Index: key, BoxId: key, Type: proto.ETreasureBoxType_ETreasureBoxType_Normal, State: proto.TreasureBoxState_TreasureBoxState_Open}
		}
	}
	return n
}
func (s *Player) LocalSceneProcesses() []*proto.SceneProcess {
	out := []*proto.SceneProcess{}
	p := s.LocalProgress()
	for _, id := range gdconf.LocalSceneIDs() {
		if !p.Maps[id] && id != 100 && id != 9999 && id != 1 {
			continue
		}
		process := uint32(0)
		if p.CompletedScenes[id] {
			process = gdconf.GetLocalCatalog().Scenes[id].Total
		}
		out = append(out, &proto.SceneProcess{SceneId: id, Process: process})
	}
	return out
}
func (s *Player) LocalCollectionIDs() []uint32 {
	set := map[uint32]bool{}
	for id, v := range s.LocalProgress().Collections {
		if v {
			set[id] = true
		}
	}
	for _, sc := range s.GetSceneModel().GetSceneMap() {
		for _, c := range sc.GetCollections() {
			for id := range c.ItemMap {
				set[id] = true
			}
		}
	}
	out := []uint32{}
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
func (s *Player) LocalFillSceneData(info *proto.SceneData) {
	p := s.LocalProgress()
	if !p.CompletedScenes[info.SceneId] {
		return
	}
	// Return only the collectables mapped to this scene by configured area relations.
	groups := map[uint32]*proto.CollectionData{}
	for _, g := range info.Collections {
		groups[g.Type] = g
	}
	for id := range gdconf.GetLocalCatalog().Scenes[info.SceneId].Collections {
		if !p.Collections[id] {
			continue
		}
		c := gdconf.GetCollectionItem(id)
		if c == nil {
			continue
		}
		t := uint32(c.NewCollectionType)
		g := groups[t]
		if g == nil {
			g = &proto.CollectionData{Type: t, ItemMap: map[uint32]*proto.PBCollectionRewardData{}}
			groups[t] = g
		}
		if g.ItemMap == nil {
			g.ItemMap = map[uint32]*proto.PBCollectionRewardData{}
		}
		g.ItemMap[id] = &proto.PBCollectionRewardData{ItemId: id, Status: proto.RewardStatus_RewardStatus_Reward}
	}
	info.Collections = nil
	for _, g := range groups {
		g.Level = gdconf.GetLocalCatalog().Scenes[info.SceneId].Levels[g.Type]
		info.Collections = append(info.Collections, g)
	}
	sort.Slice(info.Collections, func(i, j int) bool { return info.Collections[i].Type < info.Collections[j].Type })
	for _, v := range gdconf.GetLocalCatalog().Challenge.GetChallenge().GetDatas() {
		if uint32(v.SceneID) == info.SceneId {
			info.Challenges = append(info.Challenges, &proto.ChallengeData{ChallengeId: uint32(v.ID), State: proto.BattleState_BattleState_Finish, Star: 3, UseTime: 1})
		}
	}
	// Use the scene's configured riddle relations.
	for id := range gdconf.GetLocalCatalog().Scenes[info.SceneId].Riddles {
		info.Riddles = append(info.Riddles, &proto.RiddleData{RiddleId: id, IsWin: true, State: proto.BattleState_BattleState_Finish})
	}
	for _, v := range gdconf.GetSceneFlagConfigure(info.SceneId) {
		for _, f := range v.FlagGroupInfo {
			info.Flags = append(info.Flags, &proto.FlagBattleData{BattleId: uint32(f.RelationID), Type: proto.EMissionType(f.NewMissionType), State: proto.BattleState_BattleState_Finish, FinishTimes: 1})
		}
	}
}
func (s *Player) LocalDailyTask() *proto.PlayerDailyTask {
	p := s.LocalProgress()
	date := time.Now().In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02")
	if p.DailyDate != date {
		p.DailyDate = date
		p.DailyConverted = 0
	}
	max := uint32(len(gdconf.GetLocalCatalog().Daily.GetDailyTaskReward().GetDatas()))
	left := uint32(0)
	if p.DailyConverted < max {
		left = max - p.DailyConverted
	}
	return &proto.PlayerDailyTask{Tasks: map[uint32]uint32{}, TodayConverted: p.DailyConverted, ExchangeTimesLeft: left}
}
func (s *Player) LocalProgressSummary() map[string]int {
	p := s.LocalProgress()
	areas, boxes := 0, 0
	for _, sc := range s.GetSceneModel().GetSceneMap() {
		areas += len(sc.AreaDatas)
		for _, b := range sc.TreasureBoxs {
			if b.State == proto.TreasureBoxState_TreasureBoxState_Open {
				boxes++
			}
		}
	}
	return map[string]int{"quests": len(p.Quests), "maps": len(p.Maps), "collections": len(p.Collections), "completedMaps": len(p.CompletedScenes), "areas": areas, "treasureBoxes": boxes}
}
