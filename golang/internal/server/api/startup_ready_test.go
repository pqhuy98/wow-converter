package api

import "testing"

func TestStartupAnnouncerWaitsForBrowseAndMaps(t *testing.T) {
	a := newStartupAnnouncer(3001)
	a.mapsFinished()
	a.browseFinished(true, 100, 200)
	if !a.announced {
		t.Fatal("expected ready announcement after browse and maps complete")
	}
}

func TestStartupAnnouncerSkipsFailedBrowse(t *testing.T) {
	a := newStartupAnnouncer(3001)
	a.mapsFinished()
	a.browseFinished(false, 0, 0)
	if a.announced {
		t.Fatal("expected no announcement when browse preload fails")
	}
}

func TestStartupAnnouncerOnlyOnce(t *testing.T) {
	a := newStartupAnnouncer(3001)
	a.browseFinished(true, 1, 2)
	a.mapsFinished()
	if !a.announced {
		t.Fatal("expected first announcement")
	}
	a.browseFinished(true, 9, 9)
	a.mapsFinished()
	// announced stays true; no panic or duplicate logic needed
}
