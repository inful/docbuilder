package daemon

import "testing"

func TestCopyDaemonStateIsolation(t *testing.T) {
	orig := &DaemonState{Repositories: map[string]*RepoState{"r1": {Name: "repo1", URL: "u1"}}, Builds: map[string]*BuildState{"b1": {ID: "b1", Status: BuildStatusCompleted}}, Schedules: map[string]*Schedule{"s1": {ID: "s1", Name: "sched"}}, Statistics: &DaemonStats{TotalBuilds: 5}, Configuration: map[string]interface{}{"k": "v"}}
	cp := CopyDaemonState(orig)
	if cp == orig {
		t.Fatalf("expected distinct pointer")
	}
	// mutate copy
	cp.Repositories["r1"].Name = "mutated"
	cp.Builds["b1"].Status = BuildStatusFailed
	cp.Schedules["s1"].Name = "changed"
	cp.Statistics.TotalBuilds = 42
	cp.Configuration["k"] = "nv"
	// original should remain
	if orig.Repositories["r1"].Name != "repo1" {
		t.Fatalf("repo mutation leaked")
	}
	if orig.Builds["b1"].Status != BuildStatusCompleted {
		t.Fatalf("build mutation leaked")
	}
	if orig.Schedules["s1"].Name != "sched" {
		t.Fatalf("schedule mutation leaked")
	}
	if orig.Statistics.TotalBuilds != 5 {
		t.Fatalf("stats mutation leaked")
	}
	if orig.Configuration["k"].(string) != "v" {
		t.Fatalf("config mutation leaked")
	}
}
