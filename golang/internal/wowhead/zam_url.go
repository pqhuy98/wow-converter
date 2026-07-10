package wowhead

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Expansion identifies a zam modelviewer expansion.
type Expansion string

const (
	ExpansionClassic          Expansion = "classic"
	ExpansionTBC              Expansion = "tbc"
	ExpansionWrath            Expansion = "wrath"
	ExpansionCata             Expansion = "cata"
	ExpansionMists            Expansion = "mists"
	ExpansionLive             Expansion = "live"
	ExpansionPTR              Expansion = "ptr"
	ExpansionPTR2             Expansion = "ptr2"
	ExpansionLatestAvailable  Expansion = "latest-available"
)

// ZamType identifies zam URL entity types.
type ZamType string

const (
	ZamTypeNPC          ZamType = "npc"
	ZamTypeObject       ZamType = "object"
	ZamTypeItem         ZamType = "item"
	ZamTypeDressingRoom ZamType = "dressing-room"
)

// NpcURL is a zam NPC meta URL descriptor.
type NpcURL struct {
	Expansion Expansion
	Type      ZamType
	DisplayID int
}

// ObjectURL is a zam object meta URL descriptor.
type ObjectURL struct {
	Expansion Expansion
	Type      ZamType
	DisplayID int
}

// ItemURL is a zam item URL descriptor.
type ItemURL struct {
	Expansion Expansion
	Type      ZamType
	DisplayID int
	SlotID    *int
}

// ZamURL is a parsed zam/wowhead URL.
type ZamURL struct {
	Expansion Expansion
	Type      ZamType
	DisplayID int
	SlotID    *int
	Hash      string
}

var expansionMap = []struct {
	wowhead string
	zam     Expansion
}{
	{"classic", ExpansionClassic},
	{"tbc", ExpansionTBC},
	{"wotlk", ExpansionWrath},
	{"cata", ExpansionCata},
	{"mop-classic", ExpansionMists},
	{"retail", ExpansionLive},
	{"ptr", ExpansionPTR},
	{"ptr-2", ExpansionPTR2},
	{"", ExpansionLive},
}

// GetZamBaseURL returns the zam modelviewer base URL.
func GetZamBaseURL(expansion Expansion) string {
	return "https://wow.zamimg.com/modelviewer/" + string(expansion)
}

// GetWowheadPrefix returns wowhead prefix for an expansion.
func GetWowheadPrefix(expansion Expansion) string {
	for i := len(expansionMap) - 1; i >= 0; i-- {
		if expansionMap[i].zam == expansion {
			if expansionMap[i].wowhead == "" {
				return "https://www.wowhead.com"
			}
			return "https://www.wowhead.com/" + expansionMap[i].wowhead
		}
	}
	return "https://www.wowhead.com"
}

// GetExpansionFromURL parses expansion from a wowhead/zam URL.
func GetExpansionFromURL(url string) Expansion {
	for _, e := range expansionMap {
		prefix := "https://www.wowhead.com/"
		if e.wowhead != "" {
			prefix += e.wowhead + "/"
		}
		zamPrefix := GetZamBaseURL(e.zam)
		if strings.HasPrefix(url, prefix) || strings.HasPrefix(url, zamPrefix) {
			return e.zam
		}
	}
	return ""
}

// GetLatestExpansionHavingURL finds the newest expansion with a meta URL.
func GetLatestExpansionHavingURL(client *HTTPClient, path string) (Expansion, error) {
	for i := len(expansionMap) - 1; i >= 0; i-- {
		zamEx := expansionMap[i].zam
		url := GetZamBaseURL(zamEx) + "/" + path
		res, err := client.Get(url)
		if err != nil {
			continue
		}
		res.Body.Close()
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return zamEx, nil
		}
	}
	return "", fmt.Errorf("no expansion has url %s", path)
}

// GetZamURLFromWowheadURL parses a wowhead URL into a ZamURL.
func GetZamURLFromWowheadURL(client *HTTPClient, url string) (ZamURL, error) {
	typ := getTypeFromWowheadURL(url)
	if typ == "" {
		return ZamURL{}, fmt.Errorf("cannot infer type from wowhead url: %s", url)
	}
	expansion := GetExpansionFromURL(url)
	if expansion == "" {
		expansion = ExpansionLive
	}
	if typ == ZamTypeDressingRoom {
		hash := strings.Split(strings.Split(url, "#")[1], "?")[0]
		return ZamURL{Expansion: expansion, Type: typ, Hash: hash}, nil
	}
	displayID, slotID, err := getDisplayIDFromURL(client, url)
	if err != nil {
		return ZamURL{}, err
	}
	z := ZamURL{Expansion: expansion, Type: typ, DisplayID: displayID, SlotID: slotID}
	return z, nil
}

func getTypeFromWowheadURL(url string) ZamType {
	switch {
	case regexp.MustCompile(`(?i)/npc[=/]`).MatchString(url), regexp.MustCompile(`(?i)/spell[=/]`).MatchString(url):
		return ZamTypeNPC
	case regexp.MustCompile(`(?i)/object[=/]`).MatchString(url):
		return ZamTypeObject
	case regexp.MustCompile(`(?i)/item[=/]`).MatchString(url):
		return ZamTypeItem
	case regexp.MustCompile(`(?i)/dressing-room(\?.+)?#`).MatchString(url):
		return ZamTypeDressingRoom
	default:
		return ""
	}
}

func getDisplayIDFromURL(client *HTTPClient, url string) (int, *int, error) {
	if m := regexp.MustCompile(`(?i)/(?:npc|item)/(\d+)\.json`).FindStringSubmatch(url); len(m) == 2 {
		id, _ := strconv.Atoi(m[1])
		return id, nil, nil
	}
	html, err := FetchWithCache(client, url)
	if err != nil {
		return 0, nil, err
	}
	if m := regexp.MustCompile(`data-mv-display-id="(\d+)"`).FindStringSubmatch(html); len(m) == 2 {
		id, _ := strconv.Atoi(m[1])
		var slot *int
		if sm := regexp.MustCompile(`data-mv-slot="(\d+)"`).FindStringSubmatch(html); len(sm) == 2 {
			s, _ := strconv.Atoi(sm[1])
			slot = &s
		}
		return id, slot, nil
	}
	if m := regexp.MustCompile(`(?i)/(?:item|npc|object)[=/](\d+)`).FindStringSubmatch(url); len(m) == 2 {
		entityID, _ := strconv.Atoi(m[1])
		if getTypeFromWowheadURL(url) == ZamTypeItem {
			if id, err := ParseDisplayIDFromGathererHTML(html, entityID); err == nil {
				return id, nil, nil
			}
			exp := GetExpansionFromURL(url)
			if exp == "" {
				exp = ExpansionLive
			}
			gathererURL := GetWowheadPrefix(exp) + "/gatherer?items=" + m[1]
			ghtml, err := FetchWithCache(client, gathererURL)
			if err == nil {
				if id, err := ParseDisplayIDFromGathererHTML(ghtml, entityID); err == nil {
					return id, nil, nil
				}
			}
		}
	}
	return 0, nil, fmt.Errorf("cannot extract displayId from wowhead url: %s", url)
}
