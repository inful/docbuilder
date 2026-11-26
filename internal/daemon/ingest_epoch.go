package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// The ingest epoch is the counter that tells the ingest path that the
// whole corpus must be sent to ragabast again, whatever the content
// says.
//
// ragabast skips a document whose fingerprint did not change, and the
// dispatcher skips the document it uploaded before. A reingestion needs
// a fingerprint that ragabast has not seen: the epoch is mixed into the
// fingerprint of every document, so a new epoch makes every document
// look changed, and ragabast embeds the whole corpus again.
//
// The epoch lives in the state directory: it must survive a restart of
// the daemon, or every restart would send the whole corpus again.

// ingestEpochFile is what the state directory holds.
type ingestEpochFile struct {
	Epoch     int64     `json:"epoch"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ingestEpochStore keeps the epoch of the ingest.
type ingestEpochStore struct {
	path string

	mu    sync.Mutex
	epoch int64
}

// newIngestEpochStore reads the epoch of the ingest from the state
// directory. A state directory that holds no epoch yet starts at zero:
// the first documents are ingested the way they always were.
func newIngestEpochStore(stateDir string) *ingestEpochStore {
	store := &ingestEpochStore{}
	if stateDir == "" {
		return store
	}
	store.path = filepath.Join(stateDir, "ragabast-ingest-epoch.json")
	store.load()
	return store
}

// Epoch returns the epoch of the ingest.
func (s *ingestEpochStore) Epoch() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch
}

// Bump moves the ingest to the next epoch and keeps it. The documents
// sent to ragabast carry the epoch in their fingerprint, so ragabast
// embeds them again.
//
// The epoch is kept even when the write fails: the corpus is sent
// again in that case, and a daemon that restarted before the write
// would send it again too. The error is reported, not swallowed.
func (s *ingestEpochStore) Bump() (int64, error) {
	if s == nil {
		return 0, errors.New("outbound dispatcher: the ingest has no epoch")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.epoch++
	if s.path == "" {
		return s.epoch, nil
	}

	content, err := json.Marshal(ingestEpochFile{Epoch: s.epoch, UpdatedAt: time.Now().UTC()})
	if err != nil {
		return s.epoch, fmt.Errorf("outbound dispatcher: marshal the ingest epoch: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return s.epoch, fmt.Errorf("outbound dispatcher: create the state directory: %w", err)
	}
	// #nosec G306 -- the epoch of the ingest holds no secret.
	if err := os.WriteFile(s.path, content, 0o600); err != nil {
		return s.epoch, fmt.Errorf("outbound dispatcher: write the ingest epoch: %w", err)
	}
	return s.epoch, nil
}

// load reads the epoch of the ingest. A file that is not there is not
// an error: the daemon ingests for the first time.
func (s *ingestEpochStore) load() {
	content, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var file ingestEpochFile
	if err := json.Unmarshal(content, &file); err != nil {
		slog.Warn("Failed to read the ingest epoch; starting at zero", "error", err)
		return
	}
	s.epoch = file.Epoch
}

// saltedFingerprint returns the fingerprint of the document with the
// epoch of the ingest mixed into it.
//
// ragabast compares the fingerprint of the document it receives with
// the one it holds, and it embeds the document again only when they
// differ. The epoch is what makes them differ after a reingestion: the
// content of the document did not change, but the corpus must be
// embedded again.
//
// The epoch zero is the epoch of the ordinary ingest: the fingerprint
// of the document is its own.
func saltedFingerprint(fingerprint string, epoch int64) string {
	if fingerprint == "" || epoch == 0 {
		return fingerprint
	}
	sum := sha256.Sum256([]byte(strconv.FormatInt(epoch, 10) + "\x00" + fingerprint))
	return hex.EncodeToString(sum[:])
}
