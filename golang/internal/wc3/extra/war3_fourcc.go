package extra

import (
	"fmt"
	"strings"
)

const (
	lowerLetters = "abcdefghijklmnopqrstuvwxyz"
	upperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	capacityPerCase = 26 * 26 * 26 * 26
)

func indexToCode(idx int, letters string) string {
	base := 26
	c1 := letters[(idx/(base*base*base))%base]
	c2 := letters[(idx/(base*base))%base]
	c3 := letters[(idx/base)%base]
	c4 := letters[idx%base]
	return string([]byte{c1, c2, c3, c4})
}

// FourCCResult is a generated FourCC string and numeric encoding.
type FourCCResult struct {
	CodeString string
	FourCC     uint32
}

// FourCCGenerator issues unique WC3 object FourCC codes.
type FourCCGenerator struct {
	usedCodes  map[string]struct{}
	lowerIndex int
	upperIndex int
}

// NewFourCCGenerator creates a generator seeded with Blizzard-reserved codes.
func NewFourCCGenerator(initialUsed ...string) *FourCCGenerator {
	g := &FourCCGenerator{usedCodes: make(map[string]struct{}, len(fourCCSet)+len(initialUsed))}
	for code := range fourCCSet {
		g.usedCodes[code] = struct{}{}
	}
	for _, code := range initialUsed {
		g.AddUsed(code)
	}
	return g
}

// AddUsed marks an existing FourCC as consumed.
func (g *FourCCGenerator) AddUsed(code string) {
	if len(code) >= 4 {
		g.usedCodes[code[:4]] = struct{}{}
	}
}

func (g *FourCCGenerator) next(caseType string) (FourCCResult, error) {
	letters := lowerLetters
	idx := g.lowerIndex
	if caseType == "upper" {
		letters = upperLetters
		idx = g.upperIndex
	}

	for idx < capacityPerCase {
		codeString := indexToCode(idx, letters)
		idx++
		if _, used := g.usedCodes[codeString]; used {
			continue
		}
		g.usedCodes[codeString] = struct{}{}
		if caseType == "lower" {
			g.lowerIndex = idx
		} else {
			g.upperIndex = idx
		}
		var fourCC uint32
		for i, ch := range []byte(codeString) {
			fourCC |= uint32(ch) << (8 * (3 - i))
		}
		return FourCCResult{CodeString: codeString, FourCC: fourCC}, nil
	}
	return FourCCResult{}, fmt.Errorf("all FourCC values exhausted for %s case", caseType)
}

// Generate returns a unique FourCC. prefixCase: "lower", "upper", or "any".
func (g *FourCCGenerator) Generate(prefixCase string) (FourCCResult, error) {
	mode := prefixCase
	if prefixCase == "any" {
		if g.lowerIndex <= g.upperIndex {
			mode = "lower"
		} else {
			mode = "upper"
		}
	}
	if mode != "lower" && mode != "upper" {
		mode = "lower"
	}
	return g.next(mode)
}

var defaultGenerator = NewFourCCGenerator()

// GenerateFourCC is a package-level helper using the default generator.
func GenerateFourCC(prefixCase string) (FourCCResult, error) {
	if prefixCase == "" {
		prefixCase = "lower"
	}
	return defaultGenerator.Generate(prefixCase)
}

// BaseDoodadType is the parent doodad type for custom doodads.
const BaseDoodadType = "YOlb"

// BaseDestructibleType is the parent destructible type.
const BaseDestructibleType = "OTds"

// SplitObjectKey splits "code:parent" object table keys.
func SplitObjectKey(key string) (code, parent string) {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, ""
}
