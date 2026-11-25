package lint

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The list of the issues in the JSON report is a list, and not a null:
// the schema says it is an array, and the recipe of the documentation
// fails on a null.
func TestTheJSONReportHoldsAnEmptyListOfIssues(t *testing.T) {
	var buf bytes.Buffer
	err := NewJSONFormatter().Format(&buf, &Result{Issues: []Issue{}}, "docs", false)
	require.NoError(t, err)

	var report struct {
		Issues *[]json.RawMessage `json:"issues"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &report))
	require.NotNil(t, report.Issues, "the issues of the report are null")
	require.Empty(t, *report.Issues)
}
