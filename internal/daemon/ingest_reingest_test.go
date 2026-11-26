package daemon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The epoch of the ingest must survive a restart of the daemon: without
// it every restart would send the whole corpus to ragabast again.
func TestTheEpochOfTheIngestSurvivesARestart(t *testing.T) {
	dir := t.TempDir()

	store := newIngestEpochStore(dir)
	epoch, err := store.Bump()
	require.NoError(t, err)
	require.Equal(t, int64(1), epoch)

	restarted := newIngestEpochStore(dir)
	require.Equal(t, int64(1), restarted.Epoch(),
		"a restarted daemon must not send the whole corpus again")
}

// A reingestion must send the document again: ragabast skips a
// document whose fingerprint did not change, so the fingerprint of the
// document carries the epoch of the ingest.
func TestTheReingestionSendsTheDocumentAgain(t *testing.T) {
	var (
		mu           sync.Mutex
		fingerprints []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/fingerprint") {
			// ragabast does not hold the document.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		//nolint:gosec // 1<<20 sets an explicit 1 MiB memory bound
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(file)
		_ = file.Close()

		_, fingerprint := parseUIDAndFingerprint(body)
		mu.Lock()
		fingerprints = append(fingerprints, fingerprint)
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		BaseURL:   srv.URL,
		Workers:   1,
		QueueSize: 4,
		Timeout:   2 * time.Second,
		StateDir:  t.TempDir(),
	})
	require.NoError(t, err)
	d.Start(t.Context())
	defer d.Stop()

	doc := []byte("---\nuid: doc-1\nfingerprint: abc123\n---\n# Hello\n")
	d.Enqueue(doc, "docs/index.md")
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(fingerprints) == 1
	}, 2*time.Second, 5*time.Millisecond)

	mu.Lock()
	require.Equal(t, "abc123", fingerprints[0],
		"the first ingest sends the fingerprint of the document")
	mu.Unlock()

	// The dispatcher uploaded the document already: the same document
	// needs no request.
	d.Enqueue(doc, "docs/index.md")
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	require.Len(t, fingerprints, 1, "the document was sent twice without a reingestion")
	mu.Unlock()

	// The reingestion moves the epoch: the document is sent again, and
	// it carries a fingerprint that ragabast has not seen.
	epoch, err := d.Reingest()
	require.NoError(t, err)
	require.Equal(t, int64(1), epoch)

	d.Enqueue(doc, "docs/index.md")
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(fingerprints) == 2
	}, 2*time.Second, 5*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.NotEqual(t, fingerprints[0], fingerprints[1],
		"the reingestion must send a fingerprint that ragabast has not seen")
}

// The reingestion asks for a build that is not skipped: the corpus is
// what is sent to ragabast, and a skipped build sends nothing.
func TestTheReingestionAsksForABuildThatIsNotSkipped(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	dispatcher, err := NewOutboundDispatcher(DispatcherConfig{
		BaseURL:  "http://ragabast.invalid",
		StateDir: t.TempDir(),
	})
	require.NoError(t, err)

	d := daemonForTest(t, daemonForTestOpts{
		Config:             &config.Config{Build: config.BuildConfig{SkipIfUnchanged: true}},
		OrchestrationBus:   bus,
		OutboundDispatcher: dispatcher,
	})

	jobID := d.TriggerReingest()
	require.NotEmpty(t, jobID, "the reingestion must ask for a build")
	require.Equal(t, int64(1), dispatcher.IngestEpoch(),
		"the reingestion must move the epoch of the ingest")
	require.True(t, d.build.forceFullBuild.Load(),
		"the build of a reingestion must not be skipped")

	job := &queue.BuildJob{
		ID:        jobID,
		TypedMeta: &queue.BuildJobMetadata{V2Config: d.config},
	}
	req, err := d.jobToBuildRequest(job)
	require.NoError(t, err)
	require.False(t, req.Config.Build.SkipIfUnchanged,
		"the build of a reingestion must not be skipped")
	require.True(t, d.config.Build.SkipIfUnchanged,
		"the configuration of the daemon must not be changed")
}
