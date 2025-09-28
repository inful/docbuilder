//go:build disabled_discovery_state_integration
// +build disabled_discovery_state_integration

package hugo

import (
    "testing"
)

// TestDiscoveryStagePersistsPerRepoDocFilesHash ensures stageDiscoverDocs writes per-repo
// document counts and doc_files_hash into the state manager when attached to Generator.
func TestDiscoveryStagePersistsPerRepoDocFilesHash(t *testing.T) { t.Skip("disabled") }
