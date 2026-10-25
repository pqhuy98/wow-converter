package character

import (
	"context"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

func TestNPCStandReadyReplacesIdlesAndPrunesTheirTracks(t *testing.T) {
	m := mdl.New(mdl.NewMDLOptions{})
	b := components.NewBone("Body")
	b.Translation = &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{}}
	for i, name := range []string{"Stand", "Stand", "Stand Alternate", "Stand Ready", "Attack", "Portrait"} {
		lo := i * 200
		m.Sequences = append(m.Sequences, components.Sequence{Name: name, Interval: [2]int{lo, lo + 100}})
		b.Translation.KeyFrames[lo] = imath.Vector3{float64(i), 0, 0}
		b.Translation.KeyFrames[lo+100] = imath.Vector3{float64(i), 0, 0}
	}
	m.Bones = []*components.Bone{b}
	e := components.NewEventObject("SND")
	e.Track = []components.EventTrackEntry{{Sequence: &m.Sequences[0], Offset: 10}, {Sequence: &m.Sequences[3], Offset: 20}}
	m.EventObjects = []*components.EventObject{e}
	useNPCStandReady(m)
	m.Modify.OptimizeNPCSequences()
	if len(m.Sequences) != 3 || m.Sequences[0].Name != "Stand" || m.Sequences[0].Data.WC3Name != "Stand" || m.Sequences[0].Interval != [2]int{600, 700} || m.Sequences[0].NonLooping {
		t.Fatalf("wrong default idle: %+v", m.Sequences)
	}
	if len(b.Translation.KeyFrames) != 6 || len(e.Track) != 1 || e.Track[0].Sequence != &m.Sequences[0] {
		t.Fatal("removed idle tracks/events survived or ready events were lost")
	}
	if text := m.ToMdl(); !strings.Contains(text, `Anim "Stand 1"`) || strings.Contains(text, `Stand Ready`) || strings.Contains(text, `Stand Alternate`) {
		t.Fatal("serialized model did not use Ready as ordinary Stand")
	}
	noReady := mdl.New(mdl.NewMDLOptions{})
	noReady.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 100}}}
	useNPCStandReady(noReady)
	if len(noReady.Sequences) != 1 || noReady.Sequences[0].Name != "Stand" {
		t.Fatal("missing Ready removed the original idle")
	}
}

func TestNPCStandReadyPlanningSeparatesCombatCapabilityAndCachePolicies(t *testing.T) {
	a := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1}, Gameplay: &azerothcore.CreatureGameplay{Faction: 1, DamageMax: 5}}
	b := a
	b.Gameplay = &azerothcore.CreatureGameplay{Faction: 7, DamageMax: 5}
	noncombat := b
	noncombat.Gameplay = &azerothcore.CreatureGameplay{Faction: 7}
	creatures := []azerothcore.Creature{a, b, noncombat}
	model := 123
	for _, characterModel := range []bool{false, true} {
		resolve := func(azerothcore.Creature) (CharacterData, error) {
			meta := CharacterData{Model: &model}
			if characterModel {
				meta.Character = &wowhead.CharacterMeta{Race: 1}
			}
			return meta, nil
		}
		jobs, assets, err := planCreatureExports(context.Background(), creatures, config.DefaultConfig(), "build", func(c azerothcore.Creature) bool { return c.Gameplay.DamageMax > 0 }, resolve)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if characterModel {
			want = 2
		}
		if len(jobs) != want || len(assets) != 3 || assets.For(a) != assets.For(b) || (assets.For(b) != assets.For(noncombat)) != characterModel {
			t.Fatal("combat capability was ignored, faction split identical idles, or creature models were split")
		}
		ordinaryJobs, ordinary, err := planCreatureExports(context.Background(), creatures, config.DefaultConfig(), "build", nil, resolve)
		if err != nil || len(ordinaryJobs) != 1 || ordinary.For(noncombat).ConfigKey != assets.For(noncombat).ConfigKey || (ordinary.For(b).ConfigKey != assets.For(b).ConfigKey) != characterModel {
			t.Fatal("disabled experiment did not restore ordinary sharing/cache policy")
		}
	}
}
