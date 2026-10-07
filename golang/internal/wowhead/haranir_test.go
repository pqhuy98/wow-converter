package wowhead

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type haranirGathererTransport struct{}

func (haranirGathererTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/meta/character/") {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"Character":{"ChrModelFlags":1}}`))}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`WH.Gatherer.addData(3, 1, {"209902":{"json":{"displayid":671002}},"235587":{"json":{"displayid":696565}}});`))}, nil
}

func TestDecodeHaranirSeparateShoulders(t *testing.T) {
	hash := "fzn80k0zg89c8Mex8wgy8Meg8weT8MeC8wgU8MeD8wgX8MeF8wg28MeP8web8MeW8wen8Mee8weE8MeH8we08MeX8weg8Mv28wOQ8MBK8iTL8MBP8ij08MBS8inY8MBW8iyA8MBX8iyt8MB28igY8MeU8wet8MeY8we18MBQ8inQ8MBU8iyh8Mev8wej8MeJ8weV8MeK8wek8Mv58wO18MBC8iAd8MBY8ite8MB38iee8MB48ieh87OzmA7MzmnK808zmNJ87kzmNS808zbzZ808zmN18082mN808zmlm87Mzmlm87MzbzW87cz"
	client := &HTTPClient{client: &http.Client{Transport: haranirGathererTransport{}}}
	meta, err := DecodeDressingRoom(client, ExpansionLive, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !meta.SeparateShoulders || meta.Equipment["3"] != 671002 || meta.Equipment["20"] != 696565 {
		t.Fatalf("separate shoulder appearances were lost: %+v", meta)
	}
	if meta.Character == nil || meta.Character.ChrModelFlags != 1 {
		t.Fatal("dressing-room decoding discarded the model's foot-painting flag")
	}
}

func TestPlayableHaranirModelMapping(t *testing.T) {
	if err := initDressingRoomData(); err != nil {
		t.Fatal(err)
	}
	for gender, want := range []int{200, 201} {
		if got := chrModelIDFor(91, gender); got != want {
			t.Fatalf("race 91 gender %d: got model %d, want %d", gender, got, want)
		}
	}
}
