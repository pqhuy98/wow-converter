package mapexporter

import (
	"fmt"
	"math"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

func sourceCreatureGameplay(c azerothcore.Creature) azerothcore.CreatureGameplay {
	if c.Gameplay != nil {
		return *c.Gameplay
	}
	// Compatibility for old bundles: level-based resources, never a curhealth=1 maximum.
	level := math.Max(1, float64(c.Template.MaxLevel))
	return azerothcore.CreatureGameplay{Class: 1, Type: 7, Ground: 1, Swim: 1, WalkSpeed: 1, RunSpeed: 1,
		DetectionRange: 20, RegenHealth: true, Health: 40 + level*20, DamageMin: level + 2, DamageMax: level + 4, AttackSeconds: 2, RangedSeconds: 2}
}

func creatureCanAttack(g azerothcore.CreatureGameplay) bool {
	// Critters, pacified actors and explicit noncombat pets do not inherit attacks.
	// Vendor/gossip flags alone do not imply noncombat (guards can have those flags).
	return g.Type != 8 && g.Type != 12 && g.UnitFlags&0x00020000 == 0 && (g.DamageMax > 0 || g.RangedMax > 0)
}

func creatureRangedWeapon(c azerothcore.Creature) *azerothcore.ItemTemplate {
	if c.Equipment == nil {
		return nil
	}
	// Monster equipment often stores the bow/gun in Item1, not Item3.
	// Prefer the main hand, as the existing model animation export does. A
	// melee main hand plus an unused ranged slot should not turn a guard into
	// an archer with a sword-swing animation.
	for _, it := range []*azerothcore.ItemTemplate{c.Equipment.Item1, c.Equipment.Item3, c.Equipment.Item2} {
		if it == nil || it.Class != 2 {
			continue
		}
		switch it.Subclass {
		case 2, 3, 16, 18, 19:
			return it
		}
		if it == c.Equipment.Item1 {
			return nil
		}
	}
	return nil
}

func creatureDamage(c azerothcore.Creature, g azerothcore.CreatureGameplay) (float64, float64) {
	if creatureRangedWeapon(c) != nil {
		return g.RangedMin, g.RangedMax
	}
	return g.DamageMin, g.DamageMax
}

func creatureMovement(g azerothcore.CreatureGameplay) string {
	if g.Rooted != 0 || g.UnitFlags&4 != 0 {
		return ""
	}
	if g.Flight != 0 && g.Ground == 0 {
		return "fly"
	}
	if g.Ground == 0 && g.Swim != 0 {
		return "float"
	}
	if g.Ground != 0 && g.Swim != 0 {
		return "amph"
	}
	return "foot"
}

// creatureArmor preserves armor-only physical effective HP against an equal-level
// attacker. AzerothCore Unit::CalcArmorReducedDamage uses attacker level, not
// victim level; a fixed WE armor value cannot match every attacker at runtime.
func creatureArmor(armor float64, level int) int {
	if armor <= 0 {
		return 0
	}
	attackerLevel := float64(max(1, level))
	if attackerLevel > 59 {
		attackerLevel += 4.5 * (attackerLevel - 59)
	}
	// AC reduction is A/(A+K), capped at 75%: EHP/HP = 1+min(A/K,3).
	// Stock WC3 reduction is .06*W/(1+.06*W): EHP/HP = 1+.06*W.
	k := 85*attackerLevel + 400
	return int(math.Round(math.Min(3, armor/k) / .06))
}

func creatureUnitFields(c azerothcore.Creature, model, name string, modelScale, yardScale, movementYardScale float64) []data.Modification {
	g := sourceCreatureGameplay(c)
	var mods []data.Modification
	str := func(id, v string) {
		mods = append(mods, data.Modification{ID: id, Type: data.ModificationString, Value: v})
	}
	integer := func(id string, v int) {
		mods = append(mods, data.Modification{ID: id, Type: data.ModificationInt, Value: v})
	}
	real := func(id string, v float64) {
		kind := data.ModificationUnreal
		if id == "umxp" || id == "umxr" {
			kind = data.ModificationReal
		}
		mods = append(mods, data.Modification{ID: id, Type: kind, Value: float32(v)})
	}
	hp := int(math.Round(g.Health))
	mana := max(0, int(math.Round(g.Mana)))
	reach := c.Model.CombatReach
	if reach <= 0 {
		reach = 1.5
	}
	bodyScale := yardScale * math.Max(.01, c.Model.DisplayScale)
	// Geometry-derived collision is filled after exporting/resolving model assets.
	collision := 8.0
	movement := creatureMovement(g)
	// Movement follows terrain distance, independently of visual NPC enlargement.
	speed := g.RunSpeed * 7 * movementYardScale
	if movement == "fly" {
		speed = g.FlightSpeed * 7 * movementYardScale
	}
	if movement == "float" {
		speed = g.SwimSpeed * 4.722222 * movementYardScale
	}
	if movement == "" {
		speed = 0
	}
	speed = math.Min(522, math.Max(0, speed))
	str("unam", name)
	str("umdl", model)
	str("upor", model)
	str("uabi", "")
	str("uhab", "")
	str("upgr", "")
	str("utra", "")
	str("ures", "")
	str("uupt", "")
	str("ubui", "")
	// Presentation units are not part of a melee economy or tech tree.
	integer("ugol", 0)
	integer("ulum", 0)
	integer("ufoo", 0)
	integer("ufma", 0)
	integer("ubdg", 0)
	mods = append(mods, data.Modification{ID: "usca", Type: data.ModificationReal, Value: float32(modelScale)}, data.Modification{ID: "ussc", Type: data.ModificationReal, Value: float32(creatureSelectionScale(c, yardScale))})
	real("ucol", collision)
	real("udtm", 3)
	pitch, roll := 0.0, 0.0
	if g.Type == 1 && movement != "fly" {
		pitch, roll = 10, 5
	}
	real("umxp", pitch)
	real("umxr", roll)
	str("uani", "")
	str("umvt", movement)
	integer("umvs", int(math.Round(speed)))
	integer("umis", 0)
	integer("umas", 522)
	flyHeight := 0.0
	if movement == "fly" {
		flyHeight = math.Min(1000, math.Max(90, g.HoverHeight*yardScale))
	}
	real("umvh", flyHeight)
	real("umvf", 0)
	integer("uhpm", max(1, hp))
	integer("umpm", mana)
	integer("umpi", mana)
	healthRegen := 0.0
	if g.RegenHealth {
		healthRegen = float64(hp) * .001
	}
	real("uhpr", healthRegen)
	str("uhrt", "always")
	real("umpr", float64(mana)*.002)
	armor := creatureArmor(g.Armor, c.Template.MaxLevel)
	integer("udef", armor)
	defenseType := "medium"
	if armor == 0 || g.Class == 8 {
		defenseType = "none"
	} else if armor >= 4 || g.Type == 9 {
		defenseType = "large"
	}
	if movement == "fly" && g.Type != 2 {
		defenseType = "small"
	}
	str("udty", defenseType)
	armorSound := "Flesh"
	if g.Type == 9 {
		armorSound = "Metal"
	}
	str("uarm", armorSound)
	integer("udup", 0)
	integer("ulev", max(1, min(100, c.Template.MaxLevel)))
	integer("ucam", 1)
	integer("uspe", 0)
	race, classification, targetType := "creeps", "", "ground"
	switch g.Type {
	case 6:
		race = "undead"
		classification = "undead"
	case 9:
		classification = "mechanical"
	case 5:
		classification = "giant"
	case 3:
		race = "demon"
	case 8:
		race = "critters"
	}
	if movement == "fly" {
		targetType = "air"
	}
	if g.Type == 9 {
		targetType += ",mechanical"
	} else {
		targetType += ",organic"
	}
	str("urac", race)
	str("utyp", classification)
	str("utar", targetType)
	attacks := 1
	if !creatureCanAttack(g) {
		attacks = 0
	}
	integer("uaen", attacks)
	low, high := creatureDamage(c, g)
	// WC3 stores integer damage as base + dice; preserve both source endpoints.
	minDamage := max(0, int(math.Round(low)))
	maxDamage := max(minDamage, int(math.Round(high)))
	dice := 1
	sides := maxDamage - minDamage + 1
	base := minDamage - 1
	if !creatureCanAttack(g) {
		base = 0
		dice = 0
		sides = 0
	}
	integer("ua1b", base)
	integer("ua1d", dice)
	integer("ua1s", sides)
	integer("ua2b", 0)
	integer("ua2d", 0)
	integer("ua2s", 0)
	str("ua2g", "")
	str("ucs2", "")
	cooldown := g.AttackSeconds
	weapon, attackType, targets, missile := "normal", "normal", "ground,structure,debris,item,ward", ""
	attackRange := math.Max(90, (reach*c.Model.DisplayScale+1.5)*yardScale)
	sound := "WoodMediumBash"
	if g.Type == 1 {
		switch g.Family {
		case 2, 3, 11:
			sound = "MetalLightSlice"
		}
	}
	if c.Equipment != nil && c.Equipment.Item1 != nil {
		it := c.Equipment.Item1
		subclass := it.Subclass
		if it.SoundSubclass >= 0 {
			subclass = it.SoundSubclass
		}
		switch subclass {
		case 0, 1:
			sound = "MetalMediumChop"
		case 7, 8, 15:
			sound = "MetalMediumSlice"
		case 4, 5, 10:
			sound = "MetalMediumBash"
		case 6:
			sound = "MetalMediumSlice"
		}
	}
	if it := creatureRangedWeapon(c); it != nil {
		cooldown = g.RangedSeconds
		weapon = "missile"
		attackType = "pierce"
		targets = "air,ground,structure,debris,item,ward"
		sound = ""
		attackRange = math.Min(1200, math.Max(500, 30*yardScale))
		switch it.Subclass {
		case 2, 18:
			missile = "Abilities\\Weapons\\Arrow\\ArrowMissile.mdl"
		case 3:
			weapon = "instant"
			missile = "Abilities\\Weapons\\Rifle\\RifleImpact.mdl"
		case 16:
			missile = "Abilities\\Weapons\\Axe\\AxeMissile.mdl"
		case 19:
			attackType = "magic"
			missile = "Abilities\\Weapons\\AvengerMissile\\AvengerMissile.mdl"
		}
	}
	if g.DamageSchool != 0 {
		attackType = "magic"
	}
	if g.Type == 9 && weapon == "normal" {
		sound = "MetalHeavyBash"
	}
	real("ua1c", math.Max(.1, cooldown))
	integer("ua1r", int(math.Round(attackRange)))
	str("ua1w", weapon)
	str("ua1t", attackType)
	str("ua1g", targets)
	str("ua1m", missile)
	integer("ua1z", 1800)
	real("uma1", 0)
	integer("umh1", 1)
	str("ucs1", sound)
	real("udp1", math.Min(.4, cooldown*.3))
	real("ubs1", math.Min(.5, cooldown*.3))
	real("ulpx", 0)
	real("ulpy", math.Min(100, collision*.5))
	real("ulpz", math.Min(500, reach*bodyScale*.8))
	acquisition := 0.0
	if attacks != 0 && g.DetectionRange > 0 && c.Template.FlagsExtra&2 == 0 {
		acquisition = math.Max(attackRange, math.Min(1200, g.DetectionRange*yardScale))
	}
	real("uacq", acquisition)
	return mods
}

func creatureUnitKey(c azerothcore.Creature) string {
	// Include per-spawn movement overrides; otherwise identical templates share a type.
	g := sourceCreatureGameplay(c)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d/%d/%g", c.Template.Entry, c.Model.CreatureDisplayID, c.Model.DisplayScale))
	b.WriteString("/")
	b.WriteString(creatureMovement(g))
	return b.String()
}
