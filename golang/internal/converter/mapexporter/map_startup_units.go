package mapexporter

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

const unitCreationStart = "-- WOW_CONVERTER_UNITS_BEGIN"
const unitCreationEnd = "-- WOW_CONVERTER_UNITS_END"

// DOO placements are editor data; units must also be created by the map script.
// Keep this out of WCT: World Editor generates its own creation functions from
// the DOO on save, so adding a custom trigger would create every unit twice.
func injectUnitCreation(script []byte, units []data.Unit) ([]byte, error) {
	if start := bytes.Index(script, []byte(unitCreationStart)); start >= 0 {
		end := bytes.Index(script[start:], []byte(unitCreationEnd))
		if end < 0 {
			return nil, fmt.Errorf("unterminated unit creation block")
		}
		suffix := script[start+end+len(unitCreationEnd):]
		suffix = bytes.TrimPrefix(bytes.TrimPrefix(suffix, []byte("\r")), []byte("\n"))
		script = append(append([]byte(nil), script[:start]...), suffix...)
	}
	main := regexp.MustCompile(`(?m)^function main\(\)\r?$`).FindIndex(script)
	if main == nil {
		return nil, fmt.Errorf("map script has no main function")
	}
	// Replace any previous exported or template unit-creation invocation.
	tail := regexp.MustCompile(`(?m)^[\t ]*(?:WowConverterCreateUnits|CreateAllUnits)\(\)\r?\n`).ReplaceAll(script[main[0]:], nil)
	init := regexp.MustCompile(`(?m)^[\t ]*InitGlobals\(\)\r?\n`).FindIndex(tail)
	if init == nil {
		return nil, fmt.Errorf("map main has no InitGlobals call for unit creation")
	}
	tail = append(append(append([]byte(nil), tail[:init[1]]...), "WowConverterCreateUnits()\n"...), tail[init[1]:]...)
	var out strings.Builder
	out.WriteString(unitCreationStart + "\n")
	// Bound function size even for large maps. Each batch creates at most 100
	// units; none of the batches is stored as persistent custom trigger source.
	batch, count := 0, 0
	for _, u := range units {
		if u.Type == "sloc" {
			continue
		}
		if count%100 == 0 {
			if count > 0 {
				out.WriteString("end\n")
			}
			fmt.Fprintf(&out, "function WowConverterCreateUnits%d()\n    local u\n", batch)
			batch++
		}
		skin := u.Skin
		if skin == "" {
			skin = u.Type
		}
		fmt.Fprintf(&out, "    u = BlzCreateUnitWithSkin(Player(%d), FourCC(%q), %.9g, %.9g, %.9g, FourCC(%q))\n",
			u.Player, u.Type, u.Position[0], u.Position[1], wc3.Rad2Deg(u.Rotation), skin)
		if u.Scale != [3]float32{} && u.Scale != [3]float32{1, 1, 1} {
			fmt.Fprintf(&out, "    SetUnitScale(u, %.9g, %.9g, %.9g)\n", u.Scale[0], u.Scale[1], u.Scale[2])
		}
		if u.Color >= 0 {
			fmt.Fprintf(&out, "    SetUnitColor(u, ConvertPlayerColor(%d))\n", u.Color)
		}
		if u.Hero.Level > 1 {
			fmt.Fprintf(&out, "    SetHeroLevel(u, %d, false)\n", u.Hero.Level)
		}
		for _, attribute := range []struct {
			name  string
			value int
		}{{"Str", u.Hero.Str}, {"Agi", u.Hero.Agi}, {"Int", u.Hero.Int}} {
			if attribute.value > 0 {
				fmt.Fprintf(&out, "    SetHero%s(u, %d, true)\n", attribute.name, attribute.value)
			}
		}
		if u.Hitpoints >= 0 && u.Hitpoints != 100 {
			fmt.Fprintf(&out, "    SetUnitState(u, UNIT_STATE_LIFE, GetUnitState(u, UNIT_STATE_MAX_LIFE) * %d / 100)\n", u.Hitpoints)
		}
		if u.Mana >= 0 {
			fmt.Fprintf(&out, "    SetUnitState(u, UNIT_STATE_MANA, %d)\n", u.Mana)
		}
		if u.TargetAcquisition >= 0 {
			fmt.Fprintf(&out, "    SetUnitAcquireRange(u, %.9g)\n", u.TargetAcquisition)
		}
		count++
	}
	if count > 0 {
		out.WriteString("end\n")
	}
	out.WriteString("function WowConverterCreateUnits()\n")
	for i := 0; i < batch; i++ {
		fmt.Fprintf(&out, "    WowConverterCreateUnits%d()\n", i)
	}
	out.WriteString("end\n" + unitCreationEnd + "\n")
	result := append([]byte(out.String()), script[:main[0]]...)
	return append(result, tail...), nil
}
