package icon

import "testing"

func TestGetWc3Path(t *testing.T) {
	got := GetWc3Path("interface/icons/inv_belt_18.blp", FrameBtn)
	want := "ReplaceableTextures/CommandButtons/BTNinv_belt_18.blp"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMergeOptionsDefaults(t *testing.T) {
	merged := MergeOptions(ConversionOptions{Size: Size64})
	if merged.Style != StyleClassicHD20 {
		t.Fatalf("expected default style classic-hd-2.0, got %s", merged.Style)
	}
	if merged.Frame != FrameNone {
		t.Fatalf("expected default frame none, got %s", merged.Frame)
	}
	if merged.ResizeMode != ResizeNormal {
		t.Fatalf("expected default resize normal, got %s", merged.ResizeMode)
	}
}

func TestGetCustomFrameData(t *testing.T) {
	entry := getCustomFrameData(FrameAtt, Size64, StyleClassicSD)
	if entry == nil || entry.Size[0] != 48 {
		t.Fatalf("unexpected custom frame data: %+v", entry)
	}
}
