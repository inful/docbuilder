package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyAssetFile_MaxAssetSize(t *testing.T) {
	tempDir := t.TempDir()
	buildRoot := filepath.Join(tempDir, "build")
	// #nosec G301 -- test directory needs 0755 for test runner access
	err := os.MkdirAll(buildRoot, 0o755)
	require.NoError(t, err)

	cfg := &config.Config{
		Build: config.BuildConfig{
			MaxAssetSize: 1024, // 1KB limit
		},
	}
	gen := NewGenerator(cfg, buildRoot)

	t.Run("skips_file_exceeding_max_size", func(t *testing.T) {
		srcDir := t.TempDir()
		assetPath := filepath.Join(srcDir, "large.png")

		data := make([]byte, 2048)
		// #nosec G306 -- test file permissions OK
		require.NoError(t, os.WriteFile(assetPath, data, 0o644))

		docFile := docs.DocFile{
			Path:         assetPath,
			RelativePath: "large.png",
			Repository:   "test-repo",
			Extension:    ".png",
			IsAsset:      true,
		}

		err := gen.copyAssetFile(docFile, true)
		require.NoError(t, err)

		destPath := filepath.Join(buildRoot, docFile.GetHugoPath(true))
		_, err = os.Stat(destPath)
		assert.True(t, os.IsNotExist(err), "Asset exceeding limit should not have been copied")
	})

	t.Run("copies_file_within_limit", func(t *testing.T) {
		srcDir := t.TempDir()
		assetPath := filepath.Join(srcDir, "small.png")

		data := make([]byte, 512)
		// #nosec G306 -- test file permissions OK
		require.NoError(t, os.WriteFile(assetPath, data, 0o644))

		docFile := docs.DocFile{
			Path:         assetPath,
			RelativePath: "small.png",
			Repository:   "test-repo",
			Extension:    ".png",
			IsAsset:      true,
		}

		err := gen.copyAssetFile(docFile, true)
		require.NoError(t, err)

		destPath := filepath.Join(buildRoot, docFile.GetHugoPath(true))
		_, err = os.Stat(destPath)
		require.NoError(t, err, "Asset within limit should be copied")
	})
}
