package casc

import (
	"reflect"
	"testing"
	"time"
)

func TestHealthyCDNHostsTemporarilyDefersFailedClosestHost(t *testing.T) {
	now := time.Now()
	source := NewCASCRemote("eu")
	source.cdnHosts = []string{"https://closest/", "https://fallback/"}
	source.deferCDNHost("https://closest/", now)

	if got, want := source.healthyCDNHosts(now), []string{"https://fallback/", "https://closest/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hosts during cooldown = %v, want %v", got, want)
	}
	if got, want := source.healthyCDNHosts(now.Add(31*time.Second)), []string{"https://closest/", "https://fallback/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hosts after cooldown = %v, want %v", got, want)
	}
}
