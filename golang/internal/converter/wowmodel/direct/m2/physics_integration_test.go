//go:build integration_tests

package directm2

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	archiveclient "github.com/pqhuy98/wow-converter/internal/wow/archive/client"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

func TestPhysicsRealLiadrinAndLorthemar(t *testing.T) {
	transport.ConfigureBundled()
	src := cascSource{client: client.NewHTTPClient("")}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	buildKey := os.Getenv("WOW_PHYSICS_BUILD_KEY")
	if buildKey == "" {
		info, err := src.client.GetCASCInfo(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if info.Build.Product != "wow" {
			t.Fatal("load Retail WoW data or set WOW_PHYSICS_BUILD_KEY to a cached Retail build")
		}
		buildKey = info.BuildKey
	}
	getFile := func(ctx context.Context, id int) ([]byte, error) {
		raw, err := archiveclient.ReadRawCachedFile(buildKey, id)
		if err == nil && len(raw) > 0 {
			return raw, nil
		}
		return src.GetRawFile(ctx, id)
	}
	for _, tc := range []struct {
		name              string
		id, root, dynamic int
	}{{"Liadrin", 6225127, 87, 5}, {"Lorthemar", 2445592, 109, 27}} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := getFile(ctx, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			loader := m2.NewLoader(buffer.From(raw), func(ctx context.Context, id uint32) ([]byte, error) { return getFile(ctx, int(id)) })
			if err := loader.Load(ctx); err != nil {
				t.Fatal(err)
			}
			if loader.Physics == nil {
				t.Fatal("source physics not loaded")
			}
			count := 0
			for _, b := range loader.Physics.Bodies {
				if b.Type == 1 {
					count++
				}
			}
			if count != tc.dynamic {
				t.Fatalf("got %d dynamic bodies", count)
			}
			anim, err := BuildBonesData(ctx, loader, nil, tc.id, buildKey)
			if err != nil {
				t.Fatal(err)
			}
			m := mdl.New(mdl.NewMDLOptions{FormatVersion: 1000, Name: tc.name})
			a := anim.ToMdl(&m.GlobalSequences)
			m.Bones, m.Sequences = a.Bones, a.Sequences
			// Match the ordinary Character Export policy; cinematic-only clips are
			// also baked in production when retained, but not needed for this check.
			var selected []components.Sequence
			for _, s := range m.Sequences {
				if !strings.Contains(s.Name, "Cinematic") {
					selected = append(selected, s)
				}
			}
			m.Sequences = selected
			start := time.Now()
			if err := bakePhysics(ctx, m, loader.Physics, 1); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %d sequences baked in %s", tc.name, len(m.Sequences), time.Since(start))
			m.Modify.FreezeHeadOutsidePortrait().ReduceKeyFramesConservative()
			root := m.Bones[tc.root]
			if root.Rotation == nil || !root.PhysicsDriven {
				t.Fatal("hair motion missing")
			}
			for _, v := range root.Rotation.KeyFrames {
				q := v.(imath.QuaternionRotation)
				for _, x := range q {
					if math.IsNaN(x) || math.IsInf(x, 0) {
						t.Fatal("nonfinite baked rotation")
					}
				}
			}
			for _, s := range m.Sequences {
				if s.NonLooping {
					continue
				}
				first, ok := root.Rotation.KeyFrames[s.Interval[0]]
				last, okLast := root.Rotation.KeyFrames[s.Interval[1]]
				if !ok || !okLast || physicsQuaternionDistance(first.(imath.QuaternionRotation), last.(imath.QuaternionRotation)) > 1e-10 {
					t.Fatalf("loop %s %v lost matching endpoints: %v/%v %v/%v", s.Name, s.Interval, ok, okLast, first, last)
				}
			}
		})
	}
}

func physicsQuaternionDistance(a, b imath.QuaternionRotation) float64 {
	sum := 0.0
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}
