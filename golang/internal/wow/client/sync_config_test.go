package client

import "testing"

func TestDesiredConfigMatchesTypeScript(t *testing.T) {
	if DesiredConfig["copyMode"] != "FULL" {
		t.Fatalf("unexpected copyMode: %v", DesiredConfig["copyMode"])
	}
	if DesiredConfig["exportM2Bones"] != true {
		t.Fatal("expected exportM2Bones true")
	}
	if DesiredConfig["modelsExportCollision"] != true {
		t.Fatal("expected modelsExportCollision true")
	}
}
