package animation

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

// AttackTag identifies weapon attack animation filtering.
type AttackTag string

const (
	AttackTagAuto     AttackTag = "Auto"
	AttackTag1H       AttackTag = "1H"
	AttackTag2H       AttackTag = "2H"
	AttackTag2HL      AttackTag = "2HL"
	AttackTagUnarmed  AttackTag = "Unarmed"
	AttackTagBow      AttackTag = "Bow"
	AttackTagRifle    AttackTag = "Rifle"
	AttackTagThrown   AttackTag = "Thrown"
)

// GetWowAnimName returns the WoW animation name for an ID.
func GetWowAnimName(animID int) string {
	if animID >= 0 && animID < len(animNames) {
		return animNames[animID]
	}
	return "unknown"
}

// AnimNameCount returns the number of known WoW animation names.
func AnimNameCount() int { return len(animNames) }

// Wc3AnimInfo is WC3 sequence mapping for a WoW animation name.
type Wc3AnimInfo struct {
	WC3Name   string
	AttackTag string
}

// GetWc3AnimInfo maps a WoW animation name to WC3 sequence info.
func GetWc3AnimInfo(wowAnimName string) Wc3AnimInfo {
	info := getWc3AnimName(wowAnimName)
	return Wc3AnimInfo{WC3Name: info.wc3Name, AttackTag: info.attackTag}
}

type wc3AnimInfo struct {
	wc3Name   string
	attackTag string
	loop      *bool
}

// getWc3AnimName is generated in wc3_anim_mapper_gen.go from animation-mapper.ts.

// AnimationMeta mirrors bundle animation metadata.
type AnimationMeta struct {
	ID             uint16
	VariationIndex uint16
	Duration       uint32
	MoveSpeed      float32
	Flags          uint32
	Frequency      uint32
	ReplayMin      uint32
	ReplayMax      uint32
	BlendTimeIn    uint16
	BlendTimeOut   uint16
	BoxPosMin      [3]float32
	BoxPosMax      [3]float32
	BoxRadius      float32
	VariationNext  int16
	AliasNext      int16
}

// GetWarcraftSequenceData maps WoW animation metadata to MDL sequence data.
func GetWarcraftSequenceData(anim AnimationMeta) components.SequenceData {
	wowName := GetWowAnimName(int(anim.ID))
	info := getWc3AnimName(wowName)
	return components.SequenceData{
		WC3Name:      info.wc3Name,
		WowName:      wowName,
		WowVariant:   int(anim.VariationIndex),
		WowFrequency: float64(anim.Frequency) / 32767,
		AttackTag:    info.attackTag,
		Loop:         info.loop,
	}
}

// IsLoopAnimation reports whether a WoW animation should loop in WC3.
func IsLoopAnimation(wowAnimation string) bool {
	info := getWc3AnimName(wowAnimation)
	if !strings.Contains(info.wc3Name, "Cinematic") {
		if info.loop != nil {
			return *info.loop
		}
		return false
	}
	_, ok := cinematicLoopWowAnimNames[wowAnimation]
	return ok
}
