package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
)

// ReingestCmd implements the 'reingest' command.
type ReingestCmd struct {
	AdminURL string `name:"admin-url" default:"" env:"DOCBUILDER_ADMIN_URL" help:"Address of the admin API of the daemon (default: from the configuration)"`
}

// reingestTimeout bounds the wait for the answer of the daemon.
const reingestTimeout = 30 * time.Second

// defaultAdminURL is where the daemon listens when nothing says
// otherwise: the loopback interface and the default port of the admin
// API.
const defaultAdminURL = "http://127.0.0.1:8082"

// adminURLEnv names the address of the admin API of the daemon. It is
// the environment variable of the --admin-url flag.
const adminURLEnv = "DOCBUILDER_ADMIN_URL"

// maxReingestAnswer bounds the answer the daemon sends.
const maxReingestAnswer = 64 * 1024

// Run executes the reingest command.
//
// The daemon moves the epoch of the ingest and asks for a build: the
// corpus is sent to ragabast again, whatever the content says. The
// daemon that runs the site is the one that holds the corpus, so the
// command asks that daemon and not ragabast.
//
//nolint:forbidigo // fmt is used for user-facing messages
func (cmd *ReingestCmd) Run(_ *Global, root *CLI) error {
	adminURL := cmd.resolveAdminURL(root)

	endpoint := strings.TrimSuffix(adminURL, "/") + "/api/ragabast/reingest"
	ctx, cancel := context.WithTimeout(context.Background(), reingestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build the request for %s: %w", endpoint, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("the daemon at %s does not answer: %w", adminURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReingestAnswer))
	if err != nil {
		return fmt.Errorf("read the answer of %s: %w", endpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the daemon refused the reingestion (%s): %s",
			resp.Status, strings.TrimSpace(string(body)))
	}

	var trigger responses.TriggerResponse
	if err := json.Unmarshal(body, &trigger); err == nil && trigger.JobID != "" {
		fmt.Printf("Reingestion requested: the build %s carries the corpus to ragabast.\n", trigger.JobID)
		return nil
	}
	fmt.Printf("Reingestion requested: %s\n", strings.TrimSpace(string(body)))
	return nil
}

// resolveAdminURL returns the address of the admin API of the daemon.
//
// The address comes from the flag, from the environment, or from the
// configuration:
//
//  1. --admin-url, or DOCBUILDER_ADMIN_URL: the whole address.
//  2. The configuration: the port and the bind address of
//     daemon.http. The values may hold ${VAR} references, and the
//     environment and the .env files fill them.
//  3. The default: the loopback interface and the default port of the
//     admin API.
//
// A configuration that cannot be read is not an error: the daemon that
// runs the site may be named by the flag or by the environment, and a
// configuration that the daemon itself would refuse says nothing about
// where it listens.
func (cmd *ReingestCmd) resolveAdminURL(root *CLI) string {
	// The environment variables of the .env files are read before the
	// configuration: the configuration holds ${VAR} references, and
	// they name the variables of the .env files.
	if err := LoadEnvFile(); err == nil && root.Verbose {
		slog.Info("Loaded environment variables from .env file")
	}

	// The flag, or the environment variable of the flag.
	if cmd.AdminURL != "" {
		return cmd.AdminURL
	}
	if url := strings.TrimSpace(os.Getenv(adminURLEnv)); url != "" {
		return url
	}

	// The configuration.
	_, cfg, err := config.LoadWithResult(root.Config)
	if err != nil {
		slog.Warn("The configuration could not be read; the default address of the daemon is used",
			"config", root.Config,
			"error", err)
		return defaultAdminURL
	}
	if cfg.Daemon == nil {
		slog.Warn("The configuration has no daemon block; the default address of the daemon is used",
			"config", root.Config)
		return defaultAdminURL
	}

	host := cfg.Daemon.HTTP.AdminBind
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.Daemon.HTTP.AdminPort)
}
