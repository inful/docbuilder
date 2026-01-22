package frontmatterops

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"github.com/inful/mdfp"
	"github.com/stretchr/testify/require"
)

func trimSingleTrailingNewlineTest(s string) string {
	if before, ok := strings.CutSuffix(s, "\r\n"); ok {
		return before
	}
	if before, ok := strings.CutSuffix(s, "\n"); ok {
		return before
	}
	return s
}

func TestComputeFingerprint(t *testing.T) {
	t.Run("excludes fingerprint/lastmod/uid/aliases", func(t *testing.T) {
		fields := map[string]any{
			"title":       "Test",
			"fingerprint": "should-be-ignored",
			"lastmod":     "2026-01-01",
			"uid":         "123",
			"aliases":     []string{"/a"},
		}
		body := []byte("hello\n")

		got, err := ComputeFingerprint(fields, body)
		require.NoError(t, err)

		style := frontmatter.Style{Newline: "\n"}
		fmBytes, err := frontmatter.SerializeYAML(map[string]any{"title": "Test"}, style)
		require.NoError(t, err)
		fmForHash := trimSingleTrailingNewlineTest(string(fmBytes))
		expected := mdfp.CalculateFingerprintFromParts(fmForHash, string(body))

		require.Equal(t, expected, got)
	})

	t.Run("stable across map insertion order", func(t *testing.T) {
		// Both maps should serialize to the same canonical YAML and therefore hash the same.
		fieldsA := map[string]any{}
		fieldsA["title"] = "Test"
		fieldsA["weight"] = 10

		fieldsB := map[string]any{}
		fieldsB["weight"] = 10
		fieldsB["title"] = "Test"

		body := []byte("hello")

		fpA, err := ComputeFingerprint(fieldsA, body)
		require.NoError(t, err)
		fpB, err := ComputeFingerprint(fieldsB, body)
		require.NoError(t, err)

		require.Equal(t, fpA, fpB)
	})

	t.Run("trims exactly one trailing newline from serialized YAML before hashing", func(t *testing.T) {
		fields := map[string]any{"title": "Test"}
		body := []byte("hello")

		got, err := ComputeFingerprint(fields, body)
		require.NoError(t, err)

		style := frontmatter.Style{Newline: "\n"}
		fmBytes, err := frontmatter.SerializeYAML(fields, style)
		require.NoError(t, err)
		serialized := string(fmBytes)
		require.True(t, strings.HasSuffix(serialized, "\n"), "SerializeYAML is expected to end with a newline")

		expectedTrimmed := mdfp.CalculateFingerprintFromParts(trimSingleTrailingNewlineTest(serialized), string(body))
		expectedUntrimmed := mdfp.CalculateFingerprintFromParts(serialized, string(body))

		require.Equal(t, expectedTrimmed, got)
		require.NotEqual(t, expectedUntrimmed, got)
	})
}
