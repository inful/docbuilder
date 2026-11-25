package middleware

import (
	"bufio"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// A handler behind the chain that streams its answer: the wrapper of
// the response holds the answer in a buffer, and the reader waits for
// the end of it.
func TestTheHandlerBehindTheChainCanStream(t *testing.T) {
	chain := Chain(logger(), derrors.NewHTTPErrorAdapter(logger()))

	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "the wrapper of the response cannot flush")

		_, _ = w.Write([]byte("first\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("second\n"))
	}))

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL) //nolint:noctx // the test does not need a context
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	reader := bufio.NewReader(resp.Body)
	first, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "first\n", first)
}

// The wrapper of the response holds the answer of the handler, and the
// handler reaches the writer of the server through it.
func TestTheHandlerBehindTheChainReachesTheWriter(t *testing.T) {
	chain := Chain(logger(), derrors.NewHTTPErrorAdapter(logger()))

	var unwrapper interface{ Unwrap() http.ResponseWriter }
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		unwrapper, _ = w.(interface{ Unwrap() http.ResponseWriter })
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	require.NotNil(t, unwrapper, "the handler cannot reach the writer of the server")
	require.Equal(t, http.ResponseWriter(rec), unwrapper.Unwrap())
}

// The writer of the server is found through the wrapper: a handler that
// needs to take the connection over finds it.
func TestTheHandlerBehindTheChainFindsTheWriterOfTheServer(t *testing.T) {
	chain := Chain(logger(), derrors.NewHTTPErrorAdapter(logger()))

	var flushed bool
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The controller of the response looks for the abilities of
		// the writer through the wrapper.
		flushed = http.NewResponseController(w).Flush() == nil
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	require.True(t, flushed, "the handler cannot flush the answer")
}

// logger returns the logger of the middleware: the answers of the
// requests are not the subject of the test.
func logger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
