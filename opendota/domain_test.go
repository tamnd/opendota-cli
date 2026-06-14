package opendota

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// HTTP behaviour is covered in opendota_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "opendota" {
		t.Errorf("Scheme = %q, want opendota", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "opendota" {
		t.Errorf("Identity.Binary = %q, want opendota", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	typ, id, err := Domain{}.Classify("232564659")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if typ != "player" {
		t.Errorf("type = %q, want player", typ)
	}
	if id != "232564659" {
		t.Errorf("id = %q, want 232564659", id)
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestLocatePlayer(t *testing.T) {
	got, err := Domain{}.Locate("player", "232564659")
	want := "https://www.opendota.com/players/232564659"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateHero(t *testing.T) {
	got, err := Domain{}.Locate("hero", "1")
	want := "https://www.opendota.com/heroes/1"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "xyz")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
