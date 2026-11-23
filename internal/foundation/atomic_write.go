package foundation

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes a file without a window in which it is half
// written: the data goes into a file next to it, and that file takes
// its place.
//
// A write that stops in the middle - the editor is killed, the machine
// sleeps - leaves the old file alone. A write that truncates the file
// first leaves it empty, and the content is gone.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	// A file that the owner marked read-only is not replaced. The write
	// of the old way failed on it, and the mark says that the file is
	// not to be changed.
	if info, err := os.Stat(path); err == nil && info.Mode().Perm()&0o200 == 0 {
		return fmt.Errorf("the file %s is read-only", path)
	}

	dir := filepath.Dir(path)

	// The temporary file is in the same directory, so the rename does
	// not cross a file system and stays atomic.
	temp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create the temporary file: %w", err)
	}
	name := temp.Name()
	defer func() { _ = os.Remove(name) }()

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write the temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync the temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close the temporary file: %w", err)
	}
	if err := os.Chmod(name, perm); err != nil {
		return fmt.Errorf("set the permissions of the temporary file: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("put the file in place: %w", err)
	}
	return nil
}
