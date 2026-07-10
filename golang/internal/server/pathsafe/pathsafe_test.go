package pathsafe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRelativeRefRejectsTraversal(t *testing.T) {
	cases := []string{
		"",
		"../secret",
		"foo/../../etc/passwd",
		"/etc/passwd",
		`C:\Windows\System32`,
		"foo\x00bar",
	}
	for _, c := range cases {
		if err := ValidateRelativeRef(c); err == nil {
			t.Fatalf("expected reject for %q", c)
		}
	}
}

func TestValidateRelativeRefAllowsNormal(t *testing.T) {
	cases := []string{"hero", "nested/file", "model__abc123.mdl"}
	for _, c := range cases {
		if err := ValidateRelativeRef(c); err != nil {
			t.Fatalf("expected allow for %q: %v", c, err)
		}
	}
}

func TestResolveUnderBaseStaysInside(t *testing.T) {
	base := t.TempDir()
	inside := filepath.Join(base, "nested", "file.txt")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveUnderBase(base, filepath.Join("nested", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got != inside {
		t.Fatalf("got %q want %q", got, inside)
	}

	if _, err := ResolveUnderBase(base, ".."+string(filepath.Separator)+"outside"); err == nil {
		t.Fatal("expected traversal reject")
	}
}

func TestResolveUnderBaseRejectsSymlinkEscape(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("symlink escape test skipped on Windows")
	}
	base := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link.txt")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveUnderBase(base, "link.txt"); err == nil {
		t.Fatal("expected symlink escape reject")
	}
}
