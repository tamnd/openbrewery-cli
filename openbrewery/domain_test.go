package openbrewery

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "openbrewery" {
		t.Errorf("Scheme = %q, want openbrewery", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "openbrewery" {
		t.Errorf("Identity.Binary = %q, want openbrewery", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"b54b16e1", "brewery", "b54b16e1"},
		{"anvil-brewery", "brewery", "anvil-brewery"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("brewery", "b54b16e1")
	want := "https://" + Host + "/v1/breweries/b54b16e1"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestDomainRegistered(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range h.Domains() {
		if scheme == "openbrewery" {
			return
		}
	}
	t.Error("openbrewery domain not registered in kit")
}
