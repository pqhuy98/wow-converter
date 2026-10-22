package creature

// FactionSummary uses the same merged players and overflow policy as export.
type FactionSummary struct {
	Allies         []FactionCount `json:"allies"`
	Enemies        []FactionCount `json:"enemies"`
	Neutral        []FactionCount `json:"neutral"`
	NeutralHostile int            `json:"neutralHostile"`
	NeutralPassive int            `json:"neutralPassive"`
}

type FactionCount struct {
	Name  string `json:"name"`
	Count int    `json:"units"`
}

func PreviewFactionSummary(side string, counts map[int]int) (FactionSummary, error) {
	p, err := planFactionCounts(side, counts)
	if err != nil {
		return FactionSummary{}, err
	}
	return summarizeFactions(p), nil
}

func summarizeFactions(p *FactionPlan) FactionSummary {
	s := FactionSummary{Allies: []FactionCount{}, Enemies: []FactionCount{}, Neutral: []FactionCount{}}
	user := 0
	for i, player := range p.Players {
		if player.Slot == p.UserSlot {
			user = i
		}
	}
	for i, player := range p.Players {
		if i == user || player.Count == 0 {
			continue
		}
		switch p.Reactions[i][user] {
		case FactionFriendly:
			s.Allies = appendFactionCount(s.Allies, player)
		case factionHostile:
			s.Enemies = appendFactionCount(s.Enemies, player)
		default:
			s.Neutral = appendFactionCount(s.Neutral, player)
		}
	}
	for _, assignment := range p.Assignments {
		switch assignment.Slot {
		case 24:
			s.NeutralHostile += assignment.Count
		case 27:
			s.NeutralPassive += assignment.Count
		}
	}
	return s
}

func appendFactionCount(players []FactionCount, player FactionPlayer) []FactionCount {
	for i := range players {
		if players[i].Name == player.Name {
			players[i].Count += player.Count
			return players
		}
	}
	return append(players, FactionCount{Name: player.Name, Count: player.Count})
}
