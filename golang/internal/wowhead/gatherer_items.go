package wowhead

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// GatherItemInput is an item id + bonus pair from dressing room equipment.
type GatherItemInput struct {
	ItemID    int
	ItemBonus int
}

// GatheredItem is a resolved item display id.
type GatheredItem struct {
	ItemID    int
	DisplayID int
	Name      string
}

type gathererEntry struct {
	JSON struct {
		ID          int              `json:"id"`
		DisplayID   int              `json:"displayid"`
		Name        string           `json:"name"`
		Appearances map[string][]any `json:"appearances"`
	} `json:"json"`
	JSONEquip *struct {
		Appearances map[string][]any `json:"appearances"`
	} `json:"jsonequip"`
}

// GatherItems resolves item display ids from wowhead gatherer.
func GatherItems(client *HTTPClient, expansion Expansion, items []GatherItemInput) ([]GatheredItem, error) {
	if len(items) == 0 {
		return nil, nil
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = fmt.Sprintf("%d", item.ItemID)
	}
	url := fmt.Sprintf("%s/gatherer?items=%s", GetWowheadPrefix(expansion), joinInts(items))
	body, err := FetchWithCache(client, url)
	if err != nil {
		return nil, err
	}
	payload, err := extractGathererPayload(body)
	if err != nil {
		return nil, err
	}
	var data map[string]gathererEntry
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, err
	}

	out := make([]GatheredItem, 0, len(items))
	for _, item := range items {
		key := fmt.Sprintf("%d", item.ItemID)
		entry, ok := data[key]
		if !ok {
			continue
		}
		displayID := selectDisplayIDForCharacter(entry, item.ItemBonus)
		if displayID > 0 {
			out = append(out, GatheredItem{
				ItemID: item.ItemID, DisplayID: displayID, Name: entry.JSON.Name,
			})
		}
	}
	return out, nil
}

func joinInts(items []GatherItemInput) string {
	if len(items) == 0 {
		return ""
	}
	s := fmt.Sprintf("%d", items[0].ItemID)
	for i := 1; i < len(items); i++ {
		s += "," + fmt.Sprintf("%d", items[i].ItemID)
	}
	return s
}

func extractGathererPayload(scriptText string) (string, error) {
	re := regexp.MustCompile(`WH\.Gatherer\.addData\([^,]+,[^,]+,\s*(\{[\s\S]*\})\);?`)
	match := re.FindStringSubmatch(scriptText)
	if len(match) < 2 {
		return "", fmt.Errorf("failed to extract Gatherer payload")
	}
	return match[1], nil
}

func selectDisplayIDForCharacter(entry gathererEntry, itemBonus int) int {
	idx := displayModIndexForBonus(itemBonus)
	if id := appearanceDisplayID(entry.JSON.Appearances, idx); id > 0 {
		return id
	}
	if entry.JSONEquip != nil {
		if id := appearanceDisplayID(entry.JSONEquip.Appearances, idx); id > 0 {
			return id
		}
	}
	return entry.JSON.DisplayID
}

func displayModIndexForBonus(itemBonus int) int {
	switch itemBonus {
	case 6806, 7980:
		return 1
	case 6807, 7309:
		return 3
	case 12282:
		return 4
	default:
		return 0
	}
}

func appearanceDisplayID(appearances map[string][]any, idx int) int {
	if appearances == nil {
		return 0
	}
	app, ok := appearances[fmt.Sprintf("%d", idx)]
	if !ok || len(app) == 0 {
		return 0
	}
	switch id := app[0].(type) {
	case float64:
		return int(id)
	case int:
		return id
	default:
		return 0
	}
}
