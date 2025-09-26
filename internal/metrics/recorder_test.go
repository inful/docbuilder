package metrics

import "time"

type testRecorder struct {
	stageDurations map[string]int
	stageResults   map[string]map[ResultLabel]int
	buildDurations int
	buildOutcomes  map[string]int
}

func newTestRecorder() *testRecorder {
	return &testRecorder{stageDurations: map[string]int{}, stageResults: map[string]map[ResultLabel]int{}, buildOutcomes: map[string]int{}}
}

func (t *testRecorder) ObserveStageDuration(stage string, _ time.Duration) {
	t.stageDurations[stage]++
}
func (t *testRecorder) ObserveBuildDuration(_ time.Duration) { t.buildDurations++ }
func (t *testRecorder) IncStageResult(stage string, result ResultLabel) {
	m, ok := t.stageResults[stage]
	if !ok {
		m = map[ResultLabel]int{}
		t.stageResults[stage] = m
	}
	m[result]++
}
func (t *testRecorder) IncBuildOutcome(outcome string) { t.buildOutcomes[outcome]++ }
