package server

import "testing"

func TestIsSettableConfigKey(t *testing.T) {
	t.Parallel()
	if !IsSettableConfigKey("copyMode") {
		t.Fatal("copyMode should be settable")
	}
	for _, key := range []string{"listfileURL", "exportDirectory", "dbdURL", "__proto__"} {
		if IsSettableConfigKey(key) {
			t.Fatalf("%q must not be settable", key)
		}
	}
}
