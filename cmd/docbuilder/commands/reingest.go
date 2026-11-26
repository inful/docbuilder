package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	adminURL, err := cmd.resolveAdminURL(root)
	if err != nil {
		return err
	}

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
// The address comes from the flag, or from the configuration: the port
// and the bind address of the admin API. A daemon that listens on
// every interface is reached through the loopback.
func (cmd *ReingestCmd) resolveAdminURL(root *CLI) (string, error) {
	if cmd.AdminURL != "" {
		return cmd.AdminURL, nil
	}

	_, cfg, err := config.LoadWithResult(root.Config)
	if err != nil {
		return "", fmt.Errorf("load the configuration to find the admin API of the daemon: %w (name the daemon with --admin-url)", err)
	}
	if cfg.Daemon == nil {
		return "", errors.New("the configuration has no daemon block: the admin API of the daemon is not known (name the daemon with --admin-url)")
	}

	host := cfg.Daemon.HTTP.AdminBind
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.Daemon.HTTP.AdminPort), nil
}
