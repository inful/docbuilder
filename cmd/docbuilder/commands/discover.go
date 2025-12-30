package commands

import (
	"context"
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// DiscoverCmd implements the 'discover' command.
type DiscoverCmd struct {
	Repository string `short:"r" help:"Specific repository to discover (optional)"`
}

func (d *DiscoverCmd) Run(_ *Global, root *CLI) error {
	cfg, err := config.Load(root.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := ApplyAutoDiscovery(context.Background(), cfg); err != nil {
		return err
	}
	return RunDiscover(cfg, d.Repository)
}

func RunDiscover(cfg *config.Config, specificRepo string) error {
	slog.Info("Starting documentation discovery", "repositories", len(cfg.Repositories))

	// Create workspace manager
	wsManager, err := CreateWorkspace(cfg)
	if err != nil {
		return err
	}
	defer CleanupWorkspace(wsManager)

	// Create Git client
	gitClient, err := CreateGitClient(wsManager, cfg)
	if err != nil {
		return err
	}

	// Filter repositories if specific one requested
	var reposToProcess []config.Repository
	if specificRepo != "" {
		for i := range cfg.Repositories {
			repo := &cfg.Repositories[i]
			if repo.Name == specificRepo {
				reposToProcess = []config.Repository{*repo}
				break
			}
		}
		if len(reposToProcess) == 0 {
			return fmt.Errorf("repository '%s' not found in configuration", specificRepo)
		}
	} else {
		reposToProcess = cfg.Repositories
	}

	// Clone repositories
	repoPaths := make(map[string]string)
	for i := range reposToProcess {
		repo := &reposToProcess[i]
		slog.Info("Cloning repository", "name", repo.Name, "url", repo.URL)

		var repoPath string
		repoPath, err = gitClient.CloneRepo(*repo)
		if err != nil {
			slog.Error("Failed to clone repository", "name", repo.Name, "error", err)
			return err
		}

		repoPaths[repo.Name] = repoPath
	}

	// Discover documentation files
	discovery := docs.NewDiscovery(reposToProcess, &cfg.Build)
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return err
	}

	// Print discovery results
	slog.Info("Discovery completed", "total_files", len(docFiles))

	filesByRepo := discovery.GetDocFilesByRepository()
	for repoName, files := range filesByRepo {
		slog.Info("Repository files", "repository", repoName, "count", len(files))
		for i := range files {
			file := &files[i]
			slog.Info("  File discovered",
				"path", file.RelativePath,
				"section", file.Section,
				"hugo_path", file.GetHugoPath())
		}
	}

	return nil
}
