package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestParseOmittedNestedTracksDoNotCreateGlobalSequence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
	}{
		{"missing-color", `"colors":[{"alpha":{"timestamps":[[0,500]],"values":[[[32767],[0]]]}}]`},
		{"missing-alpha", `"colors":[{"color":{"timestamps":[[0,500]],"values":[[[1,1,1],[0,0,0]]]}}]`},
		{"missing-rotation-and-scaling", `"textureTransforms":[{"translation":{"timestamps":[[0,500]],"values":[[[0,0,0],[1,0,0]]]}}]`},
		{"all-tracks-omitted", `"colors":[{}],"textureTransforms":[{}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := parseTrackFixture(t, `{"fileType":"m2","m2Animations":[{"duration":1000}],`+tc.fields+`}`)
			model := &mdl.MDL{}
			f.BindMdl(model)
			f.buildTextureAnims()
			f.GeosetAnimation(0, &components.Geoset{})
			if len(model.GlobalSequences) != 0 {
				t.Fatalf("omitted nested tracks created %d global sequences, want none", len(model.GlobalSequences))
			}
		})
	}
}

func TestParseExplicitGlobalZeroStillCreatesGlobalSequence(t *testing.T) {
	f := parseTrackFixture(t, `{"fileType":"m2","globalLoops":[900],"textureTransforms":[{"translation":{"globalSeq":0,"timestamps":[[0,500]],"values":[[[0,0,0],[1,0,0]]]}}]}`)
	model := &mdl.MDL{}
	f.BindMdl(model)
	anims := f.buildTextureAnims()
	if len(model.GlobalSequences) != 1 || model.GlobalSequences[0].Duration != 900 || len(anims) != 1 || anims[0].Translation == nil || anims[0].Translation.GlobalSeq != model.GlobalSequences[0] {
		t.Fatalf("explicit source global sequence zero was lost: %#v", model.GlobalSequences)
	}
	if anims[0].Rotation != nil || anims[0].Scaling != nil {
		t.Fatal("omitted rotation/scaling produced animations")
	}
}

func parseTrackFixture(t *testing.T, content string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tracks.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	f := NewFile(path, config.Config{}, nil)
	if err := f.Parse(); err != nil {
		t.Fatal(err)
	}
	return f
}
