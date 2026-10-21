package mapexporter

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

type creatureModelFields struct {
	LowerBodyRadius                   float64
	Death, Attack, Release, WalkSpeed float64
	Alternate, AttackSound            bool
	Launch                            [3]float64
	HasLaunch                         bool
}

// Read cached metadata and geometry for the collision fallback without a fresh
// export. Reuse the MDX object's typed decoders rather than editing asset bytes.
func readCreatureModelFields(ctx context.Context, path string) (creatureModelFields, error) {
	var out creatureModelFields
	f, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return out, err
	}
	magic := [4]byte{}
	if _, err = io.ReadFull(f, magic[:]); err != nil {
		return out, err
	}
	if string(magic[:]) != "MDLX" {
		return out, fmt.Errorf("expected MDX creature model: %s", filepath.Base(path))
	}
	version := 800
	var geosets []*mdx.Geoset
	var sequences []*mdx.Sequence
	var events []*mdx.EventObject
	var attachments []*mdx.Attachment
	var pivots [][3]float64
	for offset := int64(4); offset < info.Size(); {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		var header [8]byte
		if _, err := io.ReadFull(f, header[:]); err != nil {
			return out, err
		}
		size := int64(binary.LittleEndian.Uint32(header[4:]))
		offset += 8
		if size > info.Size()-offset {
			return out, fmt.Errorf("truncated creature model chunk %q", header[:4])
		}
		tag := string(header[:4])
		if tag != "SEQS" && tag != "EVTS" && tag != "ATCH" && tag != "PIVT" && tag != "GEOS" && tag != "VERS" {
			if _, err := f.Seek(size, io.SeekCurrent); err != nil {
				return out, err
			}
			offset += size
			continue
		}
		limit := int64(16 << 20)
		if tag == "GEOS" {
			limit = 64 << 20
		}
		if size > limit {
			return out, fmt.Errorf("oversized creature metadata chunk %q", tag)
		}
		raw := make([]byte, int(size))
		if _, err := io.ReadFull(f, raw); err != nil {
			return out, err
		}
		offset += size
		stream := mdx.NewBinaryStream(raw)
		for stream.Index() < len(raw) {
			before := stream.Index()
			switch tag {
			case "VERS":
				v, err := stream.ReadUint32()
				if err != nil {
					return out, err
				}
				version = int(v)
			case "GEOS":
				g := mdx.NewGeoset()
				if err := g.ReadMdx(stream, version); err != nil {
					return out, err
				}
				geosets = append(geosets, g)
			case "SEQS":
				s := mdx.NewSequence()
				if err := s.ReadMdx(stream); err != nil {
					return out, err
				}
				sequences = append(sequences, s)
			case "EVTS":
				e := mdx.NewEventObject()
				if err := e.ReadMdx(stream, 800); err != nil {
					return out, err
				}
				events = append(events, e)
			case "ATCH":
				a := mdx.NewAttachment()
				if err := a.ReadMdx(stream, 800); err != nil {
					return out, err
				}
				attachments = append(attachments, a)
			case "PIVT":
				var p [3]float64
				for i := range p {
					v, err := stream.ReadFloat32()
					if err != nil {
						return out, err
					}
					p[i] = float64(v)
				}
				pivots = append(pivots, p)
			}
			if stream.Index() <= before {
				return out, fmt.Errorf("invalid creature metadata chunk %q", tag)
			}
		}
	}
	out.LowerBodyRadius = lowerBodyCollisionRadius(geosets)
	hasStand, hasAlternate := false, false
	for _, s := range sequences {
		name := strings.ToLower(s.Name)
		duration := float64(s.Interval[1]-s.Interval[0]) / 1000
		if strings.HasPrefix(name, "death") {
			out.Death = math.Max(out.Death, duration+.2)
		}
		if strings.HasPrefix(name, "walk") && s.MoveSpeed > 0 {
			out.WalkSpeed = float64(s.MoveSpeed)
		}
		if strings.HasPrefix(name, "stand") {
			if strings.Contains(name, "alternate") {
				hasAlternate = true
			} else {
				hasStand = true
			}
		}
		if !strings.HasPrefix(name, "attack") {
			continue
		}
		if out.Attack == 0 || duration < out.Attack {
			out.Attack = duration
		}
		for _, e := range events {
			if e.GlobalSequenceID >= 0 || !strings.HasPrefix(strings.ToUpper(e.Name), "SND") {
				continue
			}
			for _, t := range e.Tracks {
				if t > s.Interval[0] && t < s.Interval[1] {
					out.AttackSound = true
					release := float64(t-s.Interval[0]) / 1000
					if out.Release == 0 || release < out.Release {
						out.Release = release
					}
				}
			}
		}
	}
	out.Alternate = !hasStand && hasAlternate
	for _, a := range attachments {
		if !strings.Contains(strings.ToLower(a.Name), "weapon") && !strings.Contains(strings.ToLower(a.Name), "hand") {
			continue
		}
		id := int(a.ObjectID)
		if id >= 0 && id < len(pivots) {
			out.Launch = pivots[id]
			out.HasLaunch = true
			break
		}
	}
	return out, nil
}

func enrichCreatureModelFields(ctx context.Context, outputDir string, mm *extra.MapManager, units []PlacedUnit, assets character.CreatureAssets) error {
	profiles := map[string]creatureModelFields{}
	byCode := map[string]PlacedUnit{}
	for _, u := range units {
		if u.TypeCode != "" {
			byCode[u.TypeCode] = u
		}
	}
	for i := range mm.UnitTypes {
		ut := &mm.UnitTypes[i]
		u, ok := byCode[ut.Code]
		if !ok {
			continue
		}
		model := u.ModelPath
		if model == "" {
			model = fmt.Sprintf("creature-%d.mdx", u.Creature.Model.CreatureDisplayID)
		}
		p, ok := profiles[model]
		if !ok {
			var err error
			p, err = readCreatureModelFields(ctx, filepath.Join(outputDir, filepath.FromSlash(model)))
			if err != nil {
				return err
			}
			profiles[model] = p
		}
		set := func(id string, kind data.ModificationType, v interface{}) {
			for j := range ut.Data {
				if ut.Data[j].ID == id {
					ut.Data[j].Type = kind
					ut.Data[j].Value = v
					return
				}
			}
			ut.Data = append(ut.Data, data.Modification{ID: id, Type: kind, Value: v})
		}
		real := func(id string, v float64) { set(id, data.ModificationUnreal, float32(v)) }
		radius := p.LowerBodyRadius
		if asset := assets.For(u.Creature); asset != nil && asset.CollisionRadius > 0 {
			radius = asset.CollisionRadius
		}
		modelScale := 1.0
		for _, field := range ut.Data {
			if field.ID == "usca" {
				if scale, ok := field.Value.(float32); ok {
					modelScale = float64(scale)
				}
			}
		}
		// Both candidates are already in model units. usca includes map fit,
		// source DisplayScale and export UnitScale; apply it exactly once.
		real("ucol", math.Min(1024, math.Max(8, radius*modelScale)))
		if p.Death > 0 {
			real("udtm", p.Death)
		}
		cooldown := sourceCreatureGameplay(u.Creature).AttackSeconds
		if creatureRangedWeapon(u.Creature) != nil {
			cooldown = sourceCreatureGameplay(u.Creature).RangedSeconds
		}
		if p.Attack > 0 {
			// Source sound events are the best available release cue; otherwise use
			// the first 35% of the swing. Never let damage point exceed the cooldown.
			release := p.Release
			if release == 0 {
				release = p.Attack * .35
			}
			release = math.Min(cooldown*.8, release)
			real("udp1", release)
			real("ubs1", math.Max(0, math.Min(cooldown-release, p.Attack-release)))
		}
		if p.AttackSound {
			set("ucs1", data.ModificationString, "")
		}
		if p.Alternate {
			set("uani", data.ModificationString, "alternate")
		}
		if p.HasLaunch {
			for j, id := range []string{"ulpx", "ulpy", "ulpz"} {
				real(id, p.Launch[j])
			}
		}
		// Model animation speed is in unscaled model units, like the WE fields.
		if p.WalkSpeed > 0 {
			set("uwal", data.ModificationReal, float32(p.WalkSpeed))
			set("urun", data.ModificationReal, float32(p.WalkSpeed))
		}
	}
	return nil
}

func lowerBodyCollisionRadius(geosets []*mdx.Geoset) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, g := range geosets {
		for i := 0; i+2 < len(g.Vertices); i += 3 {
			x, y, z := float64(g.Vertices[i]), float64(g.Vertices[i+1]), float64(g.Vertices[i+2])
			if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) || math.IsNaN(z) || math.IsInf(z, 0) {
				continue
			}
			lo, hi = math.Min(lo, z), math.Max(hi, z)
		}
	}
	if math.IsInf(lo, 0) {
		return 0
	}
	ceiling := lo + (hi-lo)*.25
	radius := 0.0
	for _, g := range geosets {
		for i := 0; i+2 < len(g.Vertices); i += 3 {
			x, y, z := float64(g.Vertices[i]), float64(g.Vertices[i+1]), float64(g.Vertices[i+2])
			if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) || math.IsNaN(z) || math.IsInf(z, 0) || z > ceiling {
				continue
			}
			radius = math.Max(radius, math.Hypot(x, y))
		}
	}
	return radius
}
