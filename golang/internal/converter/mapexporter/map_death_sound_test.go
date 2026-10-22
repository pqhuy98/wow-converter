package mapexporter

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	creature "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/creature"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestDeathSoundExportAndPersistentStartup(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultMapExportConfig()
	mm := extra.NewMapManager()
	mm.SetTerrain(mapterrain.GetInitialTerrain(2, 2))
	downloader := &deathAudioClient{raw: deathTestWAV(1, []int16{100, 200})}
	e := &MapExporter{MapExportConfig: &cfg, MapManager: mm, wowClient: downloader}
	unit := func(code string, display int, trigger bool) creature.PlacedUnit {
		c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: display}}
		if trigger {
			c.Template.FlagsExtra = 0x80
		}
		return creature.PlacedUnit{TypeCode: code, Creature: c}
	}
	units := []creature.PlacedUnit{unit("n001", 49, false), unit("n000", 50, false), unit("n001", 49, false), unit("n002", 51, false), unit("n003", 49, true)}
	dir := t.TempDir()
	if sounds, err := creature.SaveCreatureDeathSounds(ctx, downloader, dir, units, map[int]int{49: 123, 50: 123}); err != nil {
		t.Fatal(err)
	} else {
		e.deathSounds = sounds
	}
	if downloader.downloads != 1 || len(e.deathSounds) != 2 {
		t.Fatalf("audio not deduplicated or silent/trigger types included: %d %v", downloader.downloads, e.deathSounds)
	}
	if _, err := os.Stat(filepath.Join(dir, "Sounds", "WoW", "Death", "123.wav")); err != nil {
		t.Fatal(err)
	}
	injected := creature.InjectCreatureDeathSounds([]byte("function Trig_LUA_map_init_Actions()\nend\n"), e.deathSounds)
	if !bytes.Equal(injected, creature.InjectCreatureDeathSounds(injected, e.deathSounds)) {
		t.Fatal("death injection is not idempotent")
	}
	for range 2 {
		if err := e.SaveWar3mapFiles(ctx, dir, "death"); err != nil {
			t.Fatal(err)
		}
		lua, err := os.ReadFile(filepath.Join(dir, "war3map.lua"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(lua)
		for _, fragment := range []string{`[FourCC("n000")] = "Sounds\\WoW\\Death\\123.wav"`, "wowDeathSounds[GetUnitTypeId(unit)]", "EVENT_PLAYER_UNIT_DEATH", `CreateSound(path, false, true, true, 10, 10, "DefaultEAXON")`, "SetSoundVolume(sound, 127)", "SetSoundPosition(sound, GetUnitX(unit), GetUnitY(unit), 0)", "SetSoundDistances(sound, 0, 7000)", "KillSoundWhenDone(sound)"} {
			if !strings.Contains(text, fragment) {
				t.Fatalf("missing death startup: %s", fragment)
			}
		}
		if strings.Index(text, `[FourCC("n000")]`) > strings.Index(text, `[FourCC("n001")]`) {
			t.Fatal("unordered unit lookup")
		}
		wct, err := os.ReadFile(filepath.Join(dir, "war3map.wct"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = rewriteCustomTriggerText(wct, func(script []byte) ([]byte, error) {
			if len(script) > 0 && !bytes.Contains(lua, bytes.TrimRight(script, "\x00")) {
				t.Fatal("WCT differs from runnable Lua")
			}
			return script, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		imports, err := os.ReadFile(filepath.Join(dir, "war3map.imp"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Count(imports, []byte("Sounds\\WoW\\Death\\123.wav\x00")) != 1 {
			t.Fatal("death sound import missing/duplicated")
		}
	}
	for _, destructible := range []bool{false, true} {
		cfg.Creatures.Enable = destructible
		cfg.Creatures.AllAreDoodads = destructible
		if err := e.SaveWar3mapFiles(ctx, dir, "death-off"); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"war3map.lua", "war3map.wct"} {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte("WowConverterInitDeathSounds")) {
				t.Fatal("stale death trigger when NPC unit export is off")
			}
		}
	}
}

type deathAudioClient struct {
	client.Client
	downloads int
	raw       []byte
}

func (c *deathAudioClient) DownloadCascFile(context.Context, int) ([]byte, error) {
	c.downloads++
	return c.raw, nil
}

func deathTestWAV(channels int, samples []int16) []byte {
	out := append([]byte("RIFF"), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(out[4:], uint32(36+2*len(samples)))
	out = append(out, []byte("WAVEfmt ")...)
	out = binary.LittleEndian.AppendUint32(out, 16)
	for _, v := range []uint16{1, uint16(channels)} {
		out = binary.LittleEndian.AppendUint16(out, v)
	}
	out = binary.LittleEndian.AppendUint32(out, 22050)
	out = binary.LittleEndian.AppendUint32(out, uint32(22050*channels*2))
	for _, v := range []uint16{uint16(channels * 2), 16} {
		out = binary.LittleEndian.AppendUint16(out, v)
	}
	out = append(out, []byte("data")...)
	out = binary.LittleEndian.AppendUint32(out, uint32(2*len(samples)))
	for _, v := range samples {
		out = binary.LittleEndian.AppendUint16(out, uint16(v))
	}
	return out
}
