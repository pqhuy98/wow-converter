package casc

import "testing"

func TestEncodingTableLookup(t *testing.T) {
	content := CascKeyFromHex("0123456789abcdef0123456789abcdef")
	encoding := CascKeyFromHex("fedcba9876543210fedcba9876543210")

	table := newEncodingTable(16, 16)
	table.set(content, encoding, 12345)

	gotEnc, gotSize, ok := table.lookup(content)
	if !ok {
		t.Fatal("expected lookup hit")
	}
	if gotEnc != encoding {
		t.Fatalf("encoding key mismatch: got %q want %q", CascKeyToHex(gotEnc), CascKeyToHex(encoding))
	}
	if gotSize != 12345 {
		t.Fatalf("size mismatch: got %d want 12345", gotSize)
	}

	gotEncOnly, ok := table.lookupEncodingKey(content)
	if !ok || gotEncOnly != encoding {
		t.Fatalf("lookupEncodingKey failed: got %q ok=%v", CascKeyToHex(gotEncOnly), ok)
	}

	if _, _, ok := table.lookup(CascKeyFromHex("00000000000000000000000000000000")); ok {
		t.Fatal("expected lookup miss")
	}
}

func TestEncodingTableVariableKeyLengths(t *testing.T) {
	content := CascKeyFromHex("01234567")
	encoding := CascKeyFromHex("89abcdef")

	table := newEncodingTable(4, 4)
	table.set(content, encoding, 99)

	gotEnc, gotSize, ok := table.lookup(content)
	if !ok || gotEnc != encoding || gotSize != 99 {
		t.Fatalf("lookup failed: enc=%q size=%d ok=%v", CascKeyToHex(gotEnc), gotSize, ok)
	}
}
