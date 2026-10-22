package creature

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func InjectCreatureDeathSounds(script []byte, sounds map[string]string) []byte {
	anchor := regexp.MustCompile(`(?m)^function Trig_LUA_map_init_Actions\(\)\r?\n`)
	if !anchor.Match(script) {
		return script
	}
	script = regexp.MustCompile(`(?ms)^-- WOW_CONVERTER_DEATH_SOUNDS_BEGIN\r?\n.*?^-- WOW_CONVERTER_DEATH_SOUNDS_END\r?\n`).ReplaceAll(script, nil)
	script = regexp.MustCompile(`(?m)^    WowConverterInitDeathSounds\(\)\r?\n`).ReplaceAll(script, nil)
	if len(sounds) == 0 {
		return script
	}
	var b strings.Builder
	b.WriteString("-- WOW_CONVERTER_DEATH_SOUNDS_BEGIN\nlocal wowDeathSounds = {\n")
	codes := make([]string, 0, len(sounds))
	for code := range sounds {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		fmt.Fprintf(&b, "    [FourCC(%q)] = %q,\n", code, strings.ReplaceAll(sounds[code], "/", `\`))
	}
	b.WriteString(`}

local function play3dSound(path, unit)
    local sound = CreateSound(path, false, true, true, 10, 10, "DefaultEAXON")
    SetSoundVolume(sound, 127)
    SetSoundPosition(sound, GetUnitX(unit), GetUnitY(unit), 0)
    SetSoundDistances(sound, 0, 7000)
    StartSound(sound)
    KillSoundWhenDone(sound)
end

local wowDeathTrigger
function WowConverterInitDeathSounds()
    if wowDeathTrigger then
        DestroyTrigger(wowDeathTrigger)
    end
    wowDeathTrigger = CreateTrigger()
    TriggerRegisterAnyUnitEventBJ(wowDeathTrigger, EVENT_PLAYER_UNIT_DEATH)
    TriggerAddAction(wowDeathTrigger, function()
        local unit = GetTriggerUnit()
        local path = wowDeathSounds[GetUnitTypeId(unit)]
        if path then
            play3dSound(path, unit)
        end
    end)
end
-- WOW_CONVERTER_DEATH_SOUNDS_END
`)
	return anchor.ReplaceAllLiteral(script, []byte(b.String()+"function Trig_LUA_map_init_Actions()\n    WowConverterInitDeathSounds()\n"))
}
