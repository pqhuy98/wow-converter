package env_test

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/env"
)

func TestCascLocalWowStripsQuotes(t *testing.T) {
	t.Setenv("CASC_LOCAL_WOW", `"D:\Games\World of Warcraft"`)

	got := env.CascLocalWow()
	want := `D:\Games\World of Warcraft`
	if got != want {
		t.Fatalf("CascLocalWow() = %q, want %q", got, want)
	}
}

func TestCascLocalWowUnescapesDotenvBackslashes(t *testing.T) {
	t.Setenv("CASC_LOCAL_WOW", `D:\\Games\\World of Warcraft`)

	got := env.CascLocalWow()
	want := `D:\Games\World of Warcraft`
	if got != want {
		t.Fatalf("CascLocalWow() = %q, want %q", got, want)
	}
}
