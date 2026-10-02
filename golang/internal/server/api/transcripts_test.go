package api

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestTranscriptsFromZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("transcripts.final.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("{\"id\":5,\"text\":\"Hello there.\",\"ai\":0}\n"))
	_, _ = w.Write([]byte("{\"id\":6,\"text\":\"  \",\"ai\":1}\n"))
	_, _ = w.Write([]byte("not json\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := transcriptsFromZip(r)
	if got[5] != "Hello there." || len(got) != 1 {
		t.Fatalf("got %#v", got)
	}
}
