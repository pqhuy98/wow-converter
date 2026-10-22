package creature

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

type factionDefinition struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}
type factionTemplate struct {
	ID        int   `json:"id"`
	Faction   int   `json:"faction"`
	Flags     int   `json:"flags"`
	Group     int   `json:"group"`
	Friends   int   `json:"friends"`
	Enemies   int   `json:"enemies"`
	FriendIDs []int `json:"friendIDs"`
	EnemyIDs  []int `json:"enemyIDs"`
}
type factionCatalog struct {
	factions  map[int]factionDefinition
	templates map[int]factionTemplate
}

var loadCreatureFactions = sync.OnceValues(func() (*factionCatalog, error) {
	raw, err := os.ReadFile(filepath.Join(workspace.AppRoot(), "resources", "npc-factions.json"))
	if err != nil {
		return nil, fmt.Errorf("NPC factions: %w", err)
	}
	var resource struct {
		Version   int                 `json:"version"`
		Factions  []factionDefinition `json:"factions"`
		Templates []factionTemplate   `json:"templates"`
	}
	if err = json.Unmarshal(raw, &resource); err != nil {
		return nil, fmt.Errorf("NPC factions: %w", err)
	}
	if resource.Version != 1 || len(resource.Factions) == 0 || len(resource.Templates) == 0 {
		return nil, fmt.Errorf("invalid NPC factions resource")
	}
	c := &factionCatalog{factions: map[int]factionDefinition{}, templates: map[int]factionTemplate{}}
	for _, f := range resource.Factions {
		if _, exists := c.factions[f.ID]; exists || f.ID < 0 || f.Name == "" || (f.Kind != "map_player" && f.Kind != "neutral_by_reaction") {
			return nil, fmt.Errorf("invalid/duplicate NPC faction %d", f.ID)
		}
		c.factions[f.ID] = f
	}
	for _, t := range resource.Templates {
		if _, exists := c.templates[t.ID]; exists || t.ID <= 0 {
			return nil, fmt.Errorf("invalid/duplicate faction template %d", t.ID)
		}
		c.templates[t.ID] = t
	}
	if c.templates[1].ID != 1 || c.templates[2].ID != 2 {
		return nil, fmt.Errorf("missing Human/Orc player faction templates")
	}
	return c, nil
})

type FactionReaction string

const (
	FactionFriendly FactionReaction = "friendly"
	factionNeutral  FactionReaction = "neutral"
	factionHostile  FactionReaction = "hostile"
)

// AzerothCore Unit::GetFactionReactionTo / FactionTemplateEntry. Resolve each
// direction separately; explicit enemies precede friends and masks. Reputation,
// quest scripts and temporary forced reactions are outside this static export.
func reactionTo(a, b factionTemplate) FactionReaction {
	if templateHostile(a, b) {
		return factionHostile
	}
	if templateFriendly(a, b) || templateFriendly(b, a) {
		return FactionFriendly
	}
	if a.Flags&0x2000 != 0 {
		return factionHostile
	}
	return factionNeutral
}
func templateHostile(a, b factionTemplate) bool {
	if b.Faction != 0 {
		for _, id := range a.EnemyIDs {
			if id == b.Faction {
				return true
			}
		}
		for _, id := range a.FriendIDs {
			if id == b.Faction {
				return false
			}
		}
	}
	return a.Enemies&b.Group != 0
}
func templateFriendly(a, b factionTemplate) bool {
	if a.Faction == b.Faction {
		return true
	}
	if b.Faction != 0 {
		for _, id := range a.EnemyIDs {
			if id == b.Faction {
				return false
			}
		}
		for _, id := range a.FriendIDs {
			if id == b.Faction {
				return true
			}
		}
	}
	return a.Friends&b.Group != 0 || a.Group&b.Friends != 0
}

type FactionPlayer struct {
	Slot        int    `json:"slot"`
	Name        string `json:"name"`
	FactionName string `json:"factionName"`
	TemplateIDs []int  `json:"templateIDs"`
	Count       int    `json:"units"`
}
type factionAssignment struct {
	TemplateID int `json:"templateID"`
	Slot       int `json:"slot"`
	Count      int `json:"units"`
}
type FactionPlan struct {
	PlayerFaction string              `json:"playerFaction"`
	UserSlot      int                 `json:"userSlot"`
	Players       []FactionPlayer     `json:"players"`
	Reactions     [][]FactionReaction `json:"reactions"`
	Assignments   []factionAssignment `json:"assignments"`
	Collapsed     []FactionPlayer     `json:"collapsedToNeutral,omitempty"`
	owners        map[int]int
}

func MakeFactionPlan(side string, counts map[int]int, c *factionCatalog) (*FactionPlan, error) {
	if side == "" {
		side = "Alliance"
	}
	if side != "Horde" && side != "Alliance" {
		return nil, fmt.Errorf("player faction must be Horde or Alliance")
	}
	user, reference := 0, 2
	if side == "Alliance" {
		user, reference = 1, 1
	}
	p := &FactionPlan{PlayerFaction: side, UserSlot: user, owners: map[int]int{}}
	p.Players = []FactionPlayer{{Slot: user, Name: "Player (" + side + ")", FactionName: side, TemplateIDs: []int{reference}}}
	if len(counts) == 0 {
		p.Reactions = [][]FactionReaction{{FactionFriendly}}
		return p, nil
	}
	ids := make([]int, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	named := []int{reference}
	for _, id := range ids {
		t, ok := c.templates[id]
		if !ok {
			return nil, fmt.Errorf("unmapped faction template %d; update npc-factions.json", id)
		}
		f, ok := c.factions[t.Faction]
		if !ok {
			return nil, fmt.Errorf("unclassified faction %d (template %d); update npc-factions.json", t.Faction, id)
		}
		if f.Kind == "map_player" {
			named = append(named, id)
		} else {
			owner := 27
			if reactionTo(t, c.templates[reference]) == factionHostile {
				owner = 24
			}
			p.owners[id] = owner
		}
	}
	// Rebuild equivalence classes after each overflow eviction: removing a
	// participant can make previously distinct same-name templates equivalent.
	for {
		groups := namedFactionGroups(named, counts, c)
		if len(groups) <= 22 {
			p.Players = append(p.Players, groups...)
			break
		}
		least := 0
		for i := range groups {
			if groups[i].Count < groups[least].Count {
				least = i
			}
		}
		dropped := groups[least]
		dropped.Slot = 27
		if reactionTo(c.templates[dropped.TemplateIDs[0]], c.templates[reference]) == factionHostile {
			dropped.Slot = 24
		}
		p.Collapsed = append(p.Collapsed, dropped)
		removed := map[int]bool{}
		for _, id := range dropped.TemplateIDs {
			p.owners[id] = dropped.Slot
			removed[id] = true
		}
		kept := []int{reference}
		for _, id := range named[1:] {
			if !removed[id] {
				kept = append(kept, id)
			}
		}
		named = kept
	}
	nameCounts := map[string]int{}
	for _, g := range p.Players[1:] {
		nameCounts[g.Name]++
	}
	for i := 1; i < len(p.Players); i++ {
		g := &p.Players[i]
		g.Slot = i + 1
		if nameCounts[g.Name] > 1 {
			g.Name = fmt.Sprintf("%s [%d]", g.Name, g.TemplateIDs[0])
		}
		for _, id := range g.TemplateIDs {
			p.owners[id] = g.Slot
		}
	}
	p.Reactions = make([][]FactionReaction, len(p.Players))
	for i, a := range p.Players {
		p.Reactions[i] = make([]FactionReaction, len(p.Players))
		for j, b := range p.Players {
			p.Reactions[i][j] = reactionTo(c.templates[a.TemplateIDs[0]], c.templates[b.TemplateIDs[0]])
		}
	}
	for _, id := range ids {
		p.Assignments = append(p.Assignments, factionAssignment{TemplateID: id, Slot: p.owners[id], Count: counts[id]})
	}
	return p, nil
}

func planFactionCounts(side string, counts map[int]int) (*FactionPlan, error) {
	var c *factionCatalog
	if len(counts) > 0 {
		var err error
		c, err = loadCreatureFactions()
		if err != nil {
			return nil, err
		}
	}
	return MakeFactionPlan(side, counts, c)
}

func ConvertFactionPlayers(side string, mm *extra.MapManager, units []PlacedUnit) (*FactionPlan, error) {
	counts := map[int]int{}
	for _, u := range units {
		if u.unitIndex >= 0 && u.Creature.Gameplay != nil && u.Creature.Gameplay.Faction > 0 {
			counts[u.Creature.Gameplay.Faction]++
		}
	}
	p, err := planFactionCounts(side, counts)
	if err != nil {
		return nil, err
	}
	for i := range units {
		u := &units[i]
		if u.unitIndex < 0 {
			continue
		}
		owner := p.UserSlot
		if u.Creature.Gameplay != nil && u.Creature.Gameplay.Faction > 0 {
			owner = p.owners[u.Creature.Gameplay.Faction]
		}
		mm.Units[u.unitIndex].Player = owner
		mm.Units[u.unitIndex].Color = -1
		u.Player = new(int)
		*u.Player = owner
	}
	return p, nil
}

func namedFactionGroups(named []int, counts map[int]int, c *factionCatalog) []FactionPlayer {
	matrix := make([][]FactionReaction, len(named))
	for i, a := range named {
		matrix[i] = make([]FactionReaction, len(named))
		for j, b := range named {
			matrix[i][j] = reactionTo(c.templates[a], c.templates[b])
		}
	}
	var players []FactionPlayer
	groups := map[string]int{}
	for offset, id := range named[1:] {
		i := offset + 1
		name := c.factions[c.templates[id].Faction].Name
		var key strings.Builder
		key.WriteString(strconv.Quote(name))
		for j := range named {
			fmt.Fprintf(&key, "/%s:%s", matrix[i][j], matrix[j][i])
		}
		g, ok := groups[key.String()]
		if !ok {
			g = len(players)
			groups[key.String()] = g
			players = append(players, FactionPlayer{Name: name, FactionName: name})
		}
		players[g].TemplateIDs = append(players[g].TemplateIDs, id)
		players[g].Count += counts[id]
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].Name != players[j].Name {
			return players[i].Name < players[j].Name
		}
		return players[i].TemplateIDs[0] < players[j].TemplateIDs[0]
	})
	return players
}
