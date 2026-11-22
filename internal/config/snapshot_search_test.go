package config

import "testing"

// The provider decides what the site ships: the search client and the
// index. A change of it must rebuild the site.
func TestSnapshotHasTheSearchProvider(t *testing.T) {
	one := &Config{Search: &SearchConfig{Provider: SearchProviderServer}}
	two := &Config{Search: &SearchConfig{Provider: SearchProviderLunr}}
	if one.Snapshot() == two.Snapshot() {
		t.Fatal("the two providers have the same snapshot: a change of the provider would not rebuild the site")
	}
}
