package azerothcore

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// CreatureGameplay is source gameplay data; WC3 adaptation belongs to mapexporter.
// Maximum resources are derived from class/level stats, never spawn curhealth.
type CreatureGameplay struct {
	Class, Type, Family, Rank, Faction          int
	NPCFlags, UnitFlags, TypeFlags              uint32
	WalkSpeed, RunSpeed, SwimSpeed, FlightSpeed float64
	DetectionRange, HoverHeight                 float64
	Ground, Swim, Flight, Rooted                int
	RegenHealth                                 bool
	DamageSchool                                int
	Health, Mana, Armor                         float64
	DamageMin, DamageMax, RangedMin, RangedMax  float64
	AttackSeconds, RangedSeconds                float64
	FactionGroup, FriendGroup, EnemyGroup       int
}

func enrichCreatures(db *sql.DB, creatures []Creature) error {
	if len(creatures) == 0 {
		return nil
	}
	// Older user bundles and small legacy fixtures retain safe heuristic defaults.
	// Newly generated bundles always include the joined gameplay tables.
	var schema string
	if err := db.QueryRow("SELECT sql FROM sqlite_master WHERE name='creature_template'").Scan(&schema); err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(schema), "healthmodifier") {
		return nil
	}
	guids := make([]string, len(creatures))
	byGUID := map[int64]int{}
	for i, c := range creatures {
		guids[i] = strconv.FormatInt(c.Creature.GUID, 10)
		byGUID[c.Creature.GUID] = i
	}
	rows, err := db.Query(`SELECT c.guid, t.unit_class,t.type,t.family,t.rank,t.faction,t.npcflag,t.unit_flags,t.type_flags,
 t.speed_walk,t.speed_run,t.speed_swim,t.speed_flight,t.detection_range,t.HoverHeight,
 COALESCE(o.Ground,m.Ground,1),COALESCE(o.Swim,m.Swim,1),COALESCE(o.Flight,m.Flight,0),COALESCE(o.Rooted,m.Rooted,0),
 t.RegenHealth,t.dmgschool,t.exp,t.HealthModifier,t.ManaModifier,t.ArmorModifier,t.DamageModifier,
 t.BaseAttackTime,t.RangeAttackTime,t.BaseVariance,t.RangeVariance,
 s.basehp0,s.basehp1,s.basehp2,s.basemana,s.basearmor,s.attackpower,s.rangedattackpower,s.damage_base,s.damage_exp1,s.damage_exp2,
 COALESCE(f.FactionGroup,0),COALESCE(f.FriendGroup,0),COALESCE(f.EnemyGroup,0)
 FROM creature c JOIN creature_template t ON t.entry=c.id1
 LEFT JOIN creature_classlevelstats s ON s.level=t.maxlevel AND s.class=CASE WHEN t.unit_class IN (1,2,4,8) THEN t.unit_class ELSE 1 END
 LEFT JOIN creature_template_movement m ON m.CreatureId=t.entry
 LEFT JOIN creature_movement_override o ON o.SpawnId=c.guid
 LEFT JOIN factiontemplate_dbc f ON f.ID=t.faction
 WHERE c.guid IN (` + strings.Join(guids, ",") + ") ORDER BY c.guid")
	if err != nil {
		return fmt.Errorf("load creature gameplay (regenerate bundled AzerothCore tables): %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var guid int64
		var g CreatureGameplay
		var expansion, regen int
		var healthMod, manaMod, armorMod, damageMod, attackMS, rangedMS, variance, rangedVariance float64
		var hp [3]sql.NullFloat64
		var dmg [3]sql.NullFloat64
		var mana, armor, ap, rap sql.NullFloat64
		if err := rows.Scan(&guid, &g.Class, &g.Type, &g.Family, &g.Rank, &g.Faction, &g.NPCFlags, &g.UnitFlags, &g.TypeFlags,
			&g.WalkSpeed, &g.RunSpeed, &g.SwimSpeed, &g.FlightSpeed, &g.DetectionRange, &g.HoverHeight,
			&g.Ground, &g.Swim, &g.Flight, &g.Rooted, &regen, &g.DamageSchool, &expansion, &healthMod, &manaMod, &armorMod, &damageMod,
			&attackMS, &rangedMS, &variance, &rangedVariance, &hp[0], &hp[1], &hp[2], &mana, &armor, &ap, &rap, &dmg[0], &dmg[1], &dmg[2],
			&g.FactionGroup, &g.FriendGroup, &g.EnemyGroup); err != nil {
			return err
		}
		expansion = max(0, min(2, expansion))
		g.RegenHealth = regen != 0
		g.AttackSeconds = attackMS / 1000
		if g.AttackSeconds <= 0 {
			g.AttackSeconds = 2
		}
		g.RangedSeconds = rangedMS / 1000
		if g.RangedSeconds <= 0 {
			g.RangedSeconds = g.AttackSeconds
		}
		i := byGUID[guid]
		if !hp[expansion].Valid {
			return fmt.Errorf("missing creature_classlevelstats for template %d level %d class %d", creatures[i].Template.Entry, creatures[i].Template.MaxLevel, g.Class)
		}
		// Default world rates are 1. Server scripts/rates/auras are intentionally absent.
		g.Health = math.Max(1, math.Ceil(hp[expansion].Float64*healthMod))
		g.Mana = math.Max(0, math.Ceil(mana.Float64*manaMod))
		g.Armor = math.Max(0, math.Ceil(armor.Float64*armorMod))
		base := dmg[expansion].Float64
		g.DamageMin = math.Max(0, (base+ap.Float64/14*variance)*damageMod*g.AttackSeconds)
		g.DamageMax = math.Max(g.DamageMin, (base*1.5+ap.Float64/14*variance)*damageMod*g.AttackSeconds)
		g.RangedMin = math.Max(0, (base+rap.Float64/14*rangedVariance)*damageMod*g.RangedSeconds)
		g.RangedMax = math.Max(g.RangedMin, (base*1.5+rap.Float64/14*rangedVariance)*damageMod*g.RangedSeconds)
		creatures[i].Gameplay = &g
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	displays := map[int]struct{}{}
	items := map[int]struct{}{}
	for _, c := range creatures {
		displays[c.Model.CreatureDisplayID] = struct{}{}
		if c.Equipment != nil {
			for _, it := range []*ItemTemplate{c.Equipment.Item1, c.Equipment.Item2, c.Equipment.Item3} {
				if it != nil {
					items[it.Entry] = struct{}{}
				}
			}
		}
	}
	modelRows, err := db.Query("SELECT DisplayID,BoundingRadius,CombatReach FROM creature_model_info WHERE DisplayID IN (" + intSetSQL(displays) + ")")
	if err != nil {
		return err
	}
	models := map[int][2]float64{}
	for modelRows.Next() {
		var id int
		var radius, reach float64
		if err := modelRows.Scan(&id, &radius, &reach); err != nil {
			modelRows.Close()
			return err
		}
		models[id] = [2]float64{radius, reach}
	}
	err = modelRows.Err()
	modelRows.Close()
	if err != nil {
		return err
	}
	for i := range creatures {
		v := models[creatures[i].Model.CreatureDisplayID]
		creatures[i].Model.BoundingRadius = v[0]
		creatures[i].Model.CombatReach = v[1]
	}
	if len(items) == 0 {
		return nil
	}
	itemRows, err := db.Query("SELECT entry,class,subclass,SoundOverrideSubclass,delay FROM item_template WHERE entry IN (" + intSetSQL(items) + ")")
	if err != nil {
		return err
	}
	defer itemRows.Close()
	enriched := map[int]ItemTemplate{}
	for itemRows.Next() {
		var it ItemTemplate
		if err := itemRows.Scan(&it.Entry, &it.Class, &it.Subclass, &it.SoundSubclass, &it.Delay); err != nil {
			return err
		}
		enriched[it.Entry] = it
	}
	if err := itemRows.Err(); err != nil {
		return err
	}
	for i := range creatures {
		if eq := creatures[i].Equipment; eq != nil {
			for _, it := range []*ItemTemplate{eq.Item1, eq.Item2, eq.Item3} {
				if it != nil {
					v := enriched[it.Entry]
					it.Class = v.Class
					it.Subclass = v.Subclass
					it.SoundSubclass = v.SoundSubclass
					it.Delay = v.Delay
				}
			}
		}
	}
	return nil
}
