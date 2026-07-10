package wowhead

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
)

//go:embed dressing_room_templates.json
var dressingRoomTemplatesJSON []byte

//go:embed race_gender_map.json
var raceGenderMapJSON []byte

//go:embed item_enchants.json
var itemEnchantsJSON []byte

var (
	paperdollSlots = map[int]int{
		1: 1, 2: 3, 3: 16, 4: 5, 5: 4, 6: 19, 7: 9, 8: 10,
		9: 6, 10: 7, 11: 8, 12: 12, 13: 13, 14: 20,
	}
	hashTemplates     map[int]hashTemplate
	raceGenderMap     map[string]map[string]int
	itemEnchantVisual map[string]struct {
		Visual int `json:"visual"`
	}
)

type hashTemplate struct {
	Version                  int               `json:"version"`
	IncreaseDelimiters       int               `json:"increaseDelimiters"`
	DecreaseDelimiters       int               `json:"decreaseDelimiters"`
	ModifyEncodingLength     int               `json:"modifyEncodingLength"`
	ZeroDelimiterCompression json.RawMessage   `json:"zeroDelimiterCompression"`
	Data                     []templateSegment `json:"data"`
}

type templateSegment struct {
	Key                 templatePath    `json:"key"`
	KeyLong             templatePath    `json:"keyLong"`
	Delimiter           json.RawMessage `json:"delimiter"`
	BuildKey            templatePath    `json:"buildKey"`
	CalculatorValue     string          `json:"calculatorValue"`
	CalculatorLongValue string          `json:"calculatorLongValue"`
}

type templatePath []json.RawMessage

func (p *templatePath) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*p = nil
		return nil
	}
	if len(data) > 0 && data[0] == '[' {
		var values []json.RawMessage
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		*p = values
		return nil
	}
	*p = []json.RawMessage{append(json.RawMessage(nil), data...)}
	return nil
}

type decodeConfig struct {
	encoding                 string
	encodingLength           int
	delimiters               []string
	zeroDelimiterCompression int // -1 means false/disabled
}

type decodedDressingData struct {
	Settings    map[string]int            `json:"settings"`
	CustChoices map[string]custChoice     `json:"custChoices"`
	Equipment   map[string]equipmentEntry `json:"equipment"`
}

type custChoice struct {
	OptionID int `json:"optionId"`
	ChoiceID int `json:"choiceId"`
}

type equipmentEntry struct {
	ItemID    int `json:"itemId"`
	ItemBonus int `json:"itemBonus"`
	Enchant   int `json:"enchant"`
}

func initDressingRoomData() error {
	if hashTemplates != nil {
		return nil
	}
	if err := json.Unmarshal(dressingRoomTemplatesJSON, &hashTemplates); err != nil {
		return err
	}
	if err := json.Unmarshal(raceGenderMapJSON, &raceGenderMap); err != nil {
		return err
	}
	if err := json.Unmarshal(itemEnchantsJSON, &itemEnchantVisual); err != nil {
		return err
	}
	return nil
}

func getLatestTemplateVersion() int {
	latest := 0
	for v := range hashTemplates {
		if v > latest {
			latest = v
		}
	}
	return latest
}

func prepareDecodeConfig(version int) decodeConfig {
	const (
		defaultEncoding       = "0zMcmVokRsaqbdrfwihuGINALpTjnyxtgevElBCDFHJKOPQSUWXYZ123456789"
		defaultEncodingLength = 60
	)
	tpl := hashTemplates[version]
	cfg := decodeConfig{
		encoding:       defaultEncoding,
		encodingLength: defaultEncodingLength,
		delimiters:     []string{"9", "8"},
	}
	for i := 0; i < tpl.IncreaseDelimiters; i++ {
		ch := cfg.encoding[cfg.encodingLength-1]
		cfg.delimiters = append(cfg.delimiters, string(ch))
		cfg.encodingLength--
	}
	for i := 0; i < tpl.DecreaseDelimiters; i++ {
		cfg.delimiters = cfg.delimiters[:len(cfg.delimiters)-1]
		cfg.encodingLength++
	}
	cfg.encodingLength += tpl.ModifyEncodingLength
	cfg.zeroDelimiterCompression = parseZeroDelimiterCompression(tpl.ZeroDelimiterCompression)
	return cfg
}

func parseZeroDelimiterCompression(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "false" {
		return -1
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	return -1
}

func getDelimiter(cfg decodeConfig, idx int) string {
	if idx < 0 {
		idx = 1
	}
	if idx >= len(cfg.delimiters) {
		panic(fmt.Sprintf("Requested undefined delimiter: %d", idx))
	}
	return cfg.delimiters[idx]
}

func maxEncodingIndex(cfg decodeConfig) int { return cfg.encodingLength - 1 }

func charValue(cfg decodeConfig, ch string) int { return stringsIndex(cfg.encoding, ch) }

func stringsIndex(s, ch string) int {
	for i := 0; i < len(s); i++ {
		if string(s[i]) == ch {
			return i
		}
	}
	return -1
}

func longValue(cfg decodeConfig, s string) int {
	if len(s) < 2 {
		return charValue(cfg, s)
	}
	digits := []rune(s)
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	acc := 0
	for pos, digit := range digits {
		v := charValue(cfg, string(digit))
		for a := 0; a < pos; a++ {
			v *= maxEncodingIndex(cfg)
		}
		acc += v
	}
	return acc
}

func decompress(cfg decodeConfig, s string) string {
	return decodeZeroDelimiters(cfg, decodeZeroes(cfg, s))
}

func decodeZeroes(cfg decodeConfig, s string) string {
	chars := []rune(s)
	var result []rune
	run := -1
	for _, c := range chars {
		ch := string(c)
		if run >= 0 && ch == getDelimiter(cfg, 0) {
			run++
		} else if run >= 0 {
			count := charValue(cfg, ch) + (run-1)*maxEncodingIndex(cfg)
			for n := 0; n < count; n++ {
				result = append(result, '0')
			}
			run = -1
		} else {
			if ch == getDelimiter(cfg, 0) {
				run = 1
			} else {
				result = append(result, c)
			}
		}
	}
	return string(result)
}

func decodeZeroDelimiters(cfg decodeConfig, s string) string {
	if cfg.zeroDelimiterCompression < 0 {
		return s
	}
	indicator := getDelimiter(cfg, cfg.zeroDelimiterCompression)
	chars := []rune(s)
	var result []rune
	run := -1
	for _, c := range chars {
		ch := string(c)
		if run >= 0 && ch == indicator {
			run++
		} else if run >= 0 {
			count := charValue(cfg, ch) + (run-1)*maxEncodingIndex(cfg)
			for n := 0; n < count; n++ {
				result = append(result, '0')
				result = append(result, []rune(getDelimiter(cfg, 1))...)
			}
			run = -1
		} else {
			if ch == indicator {
				run = 1
			} else {
				result = append(result, c)
			}
		}
	}
	return string(result)
}

func pathKey(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.Itoa(n), nil
	}
	return "", fmt.Errorf("invalid path key")
}

func setValueOnObject(target map[string]any, path []json.RawMessage, value int) {
	var obj any = target
	for i, rawKey := range path {
		key, err := pathKey(rawKey)
		if err != nil {
			return
		}
		isLast := i == len(path)-1
		if isLast {
			if m, ok := obj.(map[string]any); ok {
				m[key] = value
			}
			return
		}
		switch cur := obj.(type) {
		case map[string]any:
			next, ok := cur[key]
			if !ok || next == nil {
				next = map[string]any{}
				cur[key] = next
			}
			obj = next
		default:
			return
		}
	}
}

func getHashPieces(cfg decodeConfig, hash string, current, next templateSegment) (string, string) {
	parts := []string{hash}
	if len(next.Key) == 0 && len(next.KeyLong) == 0 &&
		(len(current.Delimiter) != 0 || isDelimiterTrue(next.Delimiter)) {
		delim := getDelimiter(cfg, 1)
		if len(next.Delimiter) != 0 && !isDelimiterTrue(next.Delimiter) {
			var idx int
			_ = json.Unmarshal(next.Delimiter, &idx)
			delim = getDelimiter(cfg, idx)
		}
		parts = stringsSplitN(hash, delim, 2)
		if len(parts) == 1 {
			return parts[0], ""
		}
		return parts[0], delim + parts[1]
	}
	return hash, ""
}

func stringsSplitN(s, sep string, n int) []string {
	if n == 0 {
		return nil
	}
	if sep == "" {
		return []string{s}
	}
	var out []string
	for len(out) < n-1 {
		i := stringsIndexStr(s, sep)
		if i < 0 {
			break
		}
		out = append(out, s[:i])
		s = s[i+len(sep):]
	}
	out = append(out, s)
	return out
}

func stringsIndexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func isDelimiterTrue(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	return false
}

func decodeWithTemplate(cfg decodeConfig, tpl hashTemplate, rawHash string) map[string]any {
	hash := rawHash
	build := map[string]any{}
	index := 0
	for len(hash) > 0 && index < len(tpl.Data) {
		seg := tpl.Data[index]
		var next templateSegment
		if index+1 < len(tpl.Data) {
			next = tpl.Data[index+1]
		}

		if len(seg.Key) > 0 {
			if len(hash) == 0 {
				break
			}
			setValueOnObject(build, []json.RawMessage(seg.Key), charValue(cfg, hash[:1]))
			hash = hash[1:]
			index++
			continue
		}
		if len(seg.KeyLong) > 0 {
			lhs, rhs := getHashPieces(cfg, hash, seg, next)
			setValueOnObject(build, []json.RawMessage(seg.KeyLong), longValue(cfg, lhs))
			hash = rhs
			index++
			continue
		}
		if seg.Delimiter != nil {
			ch := getDelimiter(cfg, 1)
			if !isDelimiterTrue(seg.Delimiter) {
				var idx int
				_ = json.Unmarshal(seg.Delimiter, &idx)
				ch = getDelimiter(cfg, idx)
			}
			if stringsHasPrefix(hash, ch) {
				hash = hash[len(ch):]
			}
			index++
			continue
		}
		if len(seg.BuildKey) > 0 && (seg.CalculatorValue != "" || seg.CalculatorLongValue != "") {
			if seg.CalculatorLongValue != "" {
				lhs, rhs := getHashPieces(cfg, hash, seg, next)
				setValueOnObject(build, []json.RawMessage(seg.BuildKey), longValue(cfg, lhs))
				hash = rhs
			} else if len(hash) > 0 {
				setValueOnObject(build, []json.RawMessage(seg.BuildKey), charValue(cfg, hash[:1]))
				hash = hash[1:]
			}
			index++
			continue
		}
		index++
	}
	return build
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func mapToDecoded(data map[string]any) decodedDressingData {
	out := decodedDressingData{
		Settings:    map[string]int{},
		CustChoices: map[string]custChoice{},
		Equipment:   map[string]equipmentEntry{},
	}
	if settings, ok := data["settings"].(map[string]any); ok {
		for k, v := range settings {
			out.Settings[k] = toInt(v)
		}
	}
	if cust, ok := data["custChoices"].(map[string]any); ok {
		for k, v := range cust {
			if m, ok := v.(map[string]any); ok {
				out.CustChoices[k] = custChoice{
					OptionID: toInt(m["optionId"]),
					ChoiceID: toInt(m["choiceId"]),
				}
			}
		}
	}
	if equip, ok := data["equipment"].(map[string]any); ok {
		for k, v := range equip {
			if m, ok := v.(map[string]any); ok {
				out.Equipment[k] = equipmentEntry{
					ItemID:    toInt(m["itemId"]),
					ItemBonus: toInt(m["itemBonus"]),
					Enchant:   toInt(m["enchant"]),
				}
			}
		}
	}
	return out
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}

func chrModelIDFor(race, gender int) int {
	raceKey := strconv.Itoa(race)
	genderKey := strconv.Itoa(gender)
	if genders, ok := raceGenderMap[raceKey]; ok {
		if id, ok := genders[genderKey]; ok {
			return id
		}
	}
	return 0
}
