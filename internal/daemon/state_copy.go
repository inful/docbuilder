package daemon

// Manual deep copy helpers for daemon state structures. These replace the previous
// JSON marshal/unmarshal cloning approach to reduce allocations and CPU use.
// NOTE: Metadata maps (map[string]interface{}) are shallow-copied because values are
// arbitrary user-provided types; deep copying them safely would require reflection
// and is outside current scope. Mutating nested reference types inside Metadata in
// the returned copy will therefore affect the original.

// copyStringInterfaceMap shallow copies a map[string]interface{} (nil-safe).
func copyStringInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func CopyRepoState(in *RepoState) *RepoState {
	if in == nil {
		return nil
	}
	out := *in // copy value (includes pointers to time values as-is)
	if in.Metadata != nil {
		out.Metadata = copyStringInterfaceMap(in.Metadata)
	}
	return &out
}

func CopyBuildState(in *BuildState) *BuildState {
	if in == nil {
		return nil
	}
	out := *in
	if in.Metadata != nil {
		out.Metadata = copyStringInterfaceMap(in.Metadata)
	}
	return &out
}

func CopySchedule(in *Schedule) *Schedule {
	if in == nil {
		return nil
	}
	out := *in
	if in.Metadata != nil {
		out.Metadata = copyStringInterfaceMap(in.Metadata)
	}
	return &out
}

func CopyDaemonStats(in *Stats) *Stats {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

// CopyDaemonState creates a deep copy of the top-level daemon state. Nested maps
// and slices are recreated; pointer fields referencing time.Time retain the same
// underlying values (copy of pointer target not needed as they are immutable after set).
func CopyDaemonState(in *State) *State {
	if in == nil {
		return nil
	}
	out := *in
	// Copy repositories map
	if in.Repositories != nil {
		out.Repositories = make(map[string]*RepoState, len(in.Repositories))
		for k, v := range in.Repositories {
			out.Repositories[k] = CopyRepoState(v)
		}
	}
	if in.Builds != nil {
		out.Builds = make(map[string]*BuildState, len(in.Builds))
		for k, v := range in.Builds {
			out.Builds[k] = CopyBuildState(v)
		}
	}
	if in.Schedules != nil {
		out.Schedules = make(map[string]*Schedule, len(in.Schedules))
		for k, v := range in.Schedules {
			out.Schedules[k] = CopySchedule(v)
		}
	}
	if in.Statistics != nil {
		out.Statistics = CopyDaemonStats(in.Statistics)
	}
	if in.Configuration != nil {
		out.Configuration = copyStringInterfaceMap(in.Configuration)
	}
	return &out
}
