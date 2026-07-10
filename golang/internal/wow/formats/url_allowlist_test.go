package formats

import "testing"

func TestValidateFetchURL(t *testing.T) {
	t.Parallel()
	allowed := []string{
		"https://github.com/wowdev/wow-listfile/releases/latest/download/community-listfile.csv",
		"https://us.version.battle.net/indexes/wow/file",
		"https://raw.githubusercontent.com/wowdev/WoWDBDefs/master/definitions/Map.db2.dbd",
		"https://www.kruithne.net/wow.export/data/listfile/master",
	}
	for _, u := range allowed {
		if err := ValidateFetchURL(u); err != nil {
			t.Fatalf("expected allowed %q: %v", u, err)
		}
	}

	blocked := []string{
		"http://169.254.169.254/latest/meta-data/",
		"https://127.0.0.1/secret",
		"https://evil.example.com/payload",
		"file:///etc/passwd",
	}
	for _, u := range blocked {
		if err := ValidateFetchURL(u); err == nil {
			t.Fatalf("expected blocked %q", u)
		}
	}
}
