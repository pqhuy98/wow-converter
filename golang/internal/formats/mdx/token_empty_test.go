package mdx

import "testing"

func TestTokenStreamEmptyString(t *testing.T) {
	s := NewTokenStream(`Bitmap { Image "", }`)
	expect := []string{"Bitmap", "{", "Image", "", "}"}
	for i, want := range expect {
		got, err := s.Read()
		if err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
		if got != want {
			t.Fatalf("token %d: got %q want %q", i, got, want)
		}
	}
	_, err := s.Read()
	if err == nil {
		t.Fatal("expected EOF error")
	}
}
