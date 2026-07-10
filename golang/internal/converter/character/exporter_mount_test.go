package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestApplyMountRiderAttachmentSeatOffsetAndDeathHide(t *testing.T) {
	mountMdl := &mdl.MDL{
		Sequences: []components.Sequence{
			{Name: "Death", Interval: [2]int{0, 1000}},
			{Name: "Stand", Interval: [2]int{0, 2000}},
		},
	}
	atm := &components.AttachmentPoint{
		NodeBase: components.NodeBase{Name: "Item_rider.mdx", Type: "AttachmentPoint"},
		Path:     "rider.mdx",
	}
	applyMountRiderAttachment(mountMdl, atm, []float64{1, 2, 3}, 2)
	if atm.Translation == nil {
		t.Fatal("expected seat offset translation")
	}
	if got := atm.Translation.KeyFrames[0].(imath.Vector3); got[0] != 2 || got[1] != 4 || got[2] != 6 {
		t.Fatalf("unexpected seat offset translation: %+v", got)
	}
	if atm.Scaling == nil {
		t.Fatal("expected death hide scaling animation")
	}
	if _, ok := atm.Scaling.KeyFrames[0]; !ok {
		t.Fatal("expected scaling keyframe at death start")
	}
	if _, ok := atm.Scaling.KeyFrames[1000]; !ok {
		t.Fatal("expected scaling keyframe at death end")
	}
}
