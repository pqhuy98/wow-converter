package wowhead

import (
	"encoding/json"
	"fmt"
)

// FileEntry is a file entry from character meta.
type FileEntry struct {
	FileDataID int `json:"FileDataId"`
	Gender     int `json:"Gender"`
	Class      int `json:"Class"`
	Race       int `json:"Race"`
	ExtraData  int `json:"ExtraData"`
}

// CharacterMeta holds character metadata fields.
type CharacterMeta struct {
	Class    int `json:"Class"`
	Race     int `json:"Race"`
	Gender   int `json:"Gender"`
	ChrModel int `json:"ChrModelId"`
}

// CreatureMeta holds creature customization metadata.
type CreatureMeta struct {
	CreatureCustomizations []Customization `json:"CreatureCustomizations"`
	CreatureGeosetData     []GeosetEntry   `json:"CreatureGeosetData"`
}

// Customization is a creature customization option.
type Customization struct {
	OptionID int `json:"optionId"`
	ChoiceID int `json:"choiceId"`
}

// GeosetEntry is creature geoset data.
type GeosetEntry struct {
	GeosetIndex int `json:"GeosetIndex"`
	GeosetValue int `json:"GeosetValue"`
}

// CharacterData is wowhead/zam character or creature meta JSON.
type CharacterData struct {
	Model        *int           `json:"Model"`
	Textures     map[string]int `json:"Textures"`
	Character    *CharacterMeta `json:"Character"`
	Creature     *CreatureMeta  `json:"Creature"`
	Equipment    map[string]int `json:"Equipment"`
	TextureFiles map[string][]FileEntry `json:"TextureFiles"`
	ItemEffects  []ItemEffect   `json:"ItemEffects"`
}

// FetchNpcMeta fetches NPC meta JSON from zam modelviewer.
func FetchNpcMeta(client *HTTPClient, expansion Expansion, displayID int) (CharacterData, error) {
	path := fmt.Sprintf("meta/npc/%d.json", displayID)
	if expansion == ExpansionLatestAvailable {
		var err error
		expansion, err = GetLatestExpansionHavingURL(client, path)
		if err != nil {
			return CharacterData{}, err
		}
	}
	url := GetZamBaseURL(expansion) + "/" + path
	text, err := FetchWithCache(client, url)
	if err != nil {
		expansion, err2 := GetLatestExpansionHavingURL(client, path)
		if err2 != nil {
			return CharacterData{}, err
		}
		url = GetZamBaseURL(expansion) + "/" + path
		text, err = FetchWithCache(client, url)
		if err != nil {
			return CharacterData{}, err
		}
	}
	var data CharacterData
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return CharacterData{}, err
	}
	return data, nil
}

// FetchObjectMeta fetches object meta JSON from zam modelviewer.
func FetchObjectMeta(client *HTTPClient, expansion Expansion, displayID int) (CharacterData, error) {
	path := fmt.Sprintf("meta/object/%d.json", displayID)
	if expansion == ExpansionLatestAvailable {
		var err error
		expansion, err = GetLatestExpansionHavingURL(client, path)
		if err != nil {
			return CharacterData{}, err
		}
	}
	url := GetZamBaseURL(expansion) + "/" + path
	text, err := FetchWithCache(client, url)
	if err != nil {
		expansion, err2 := GetLatestExpansionHavingURL(client, path)
		if err2 != nil {
			return CharacterData{}, err
		}
		url = GetZamBaseURL(expansion) + "/" + path
		text, err = FetchWithCache(client, url)
		if err != nil {
			return CharacterData{}, err
		}
	}
	var data CharacterData
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return CharacterData{}, err
	}
	return data, nil
}

// FetchItemVisualMeta fetches item visual effect meta JSON.
func FetchItemVisualMeta(client *HTTPClient, expansion Expansion, visualID int) (CharacterData, error) {
	path := fmt.Sprintf("meta/itemvisual/%d.json", visualID)
	if expansion == ExpansionLatestAvailable {
		var err error
		expansion, err = GetLatestExpansionHavingURL(client, path)
		if err != nil {
			return CharacterData{}, err
		}
	}
	url := GetZamBaseURL(expansion) + "/" + path
	text, err := FetchWithCache(client, url)
	if err != nil {
		return CharacterData{}, err
	}
	var data CharacterData
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return CharacterData{}, err
	}
	return data, nil
}
