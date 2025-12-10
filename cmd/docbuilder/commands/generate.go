package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// GenerateCmd implements the 'generate' command for CI/CD pipelines.
type GenerateCmd struct {
	DocsDir string `name:"docs-dir" short:"d" help:"Path to local docs directory" default:"./docs"`
	Output  string `short:"o" help:"Output directory for generated site" default:"./public"`
	Theme   string `name:"theme" help:"Hugo theme to use (hextra or docsy)" default:"hextra"`
	Title   string `name:"title" help:"Site title" default:"Documentation"`
	BaseURL string `name:"base-url" help:"Base URL for the site" default:"/"`
	Render  bool   `name:"render" help:"Run Hugo to render the site" default:"true"`
}

func (g *GenerateCmd) Run(_ *Global, _ *CLI) error {
	fmt.Println("Starting static site generation")

	// Validate docs directory exists
	if _, err := os.Stat(g.DocsDir); os.IsNotExist(err) {
		return fmt.Errorf("docs directory does not exist: %s", g.DocsDir)
	}

	slog.Info("Generating static site from local docs",
		"docs_dir", g.DocsDir,
		"output", g.Output,
		"theme", g.Theme,
		"render", g.Render)

	// Use a temporary directory for the Hugo project if rendering
	hugoProjectDir := g.Output
	var tempDir string

	if g.Render {
		// Create temp directory for Hugo project
		tmp, err := os.MkdirTemp("", "docbuilder-generate-*")
		if err != nil {
			return fmt.Errorf("create temp directory: %w", err)
		}
		tempDir = tmp
		hugoProjectDir = tmp
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				slog.Warn("Failed to cleanup temp directory", "error", err)
			}
		}()
		slog.Debug("Using temporary Hugo project directory", "path", hugoProjectDir)
	}

	// Build minimal config for local generation
	cfg := &config.Config{}
	cfg.Version = "2.0"
	cfg.Output.Directory = hugoProjectDir
	cfg.Output.Clean = true
	cfg.Hugo.Title = g.Title
	cfg.Hugo.Description = "Generated documentation"
	cfg.Hugo.BaseURL = g.BaseURL
	cfg.Hugo.Theme = g.Theme

	if g.Render {
		cfg.Build.RenderMode = config.RenderModeAlways
		// Disable GitInfo for CI/CD generation (no git repo in temp dir)
		cfg.Build.LiveReload = true // This disables GitInfo
	} else {
		cfg.Build.RenderMode = config.RenderModeNever
	}

	// Single local repository entry
	cfg.Repositories = []config.Repository{{
		URL:    g.DocsDir,
		Name:   "docs",
		Branch: "",
		Paths:  []string{"."},
	}}

	// Create workspace for processing
	wsDir := cfg.Build.WorkspaceDir
	if wsDir == "" {
		wsDir = "" // Will use temp dir
	}
	wsManager := workspace.NewManager(wsDir)
	if err := wsManager.Create(); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	defer func() {
		if err := wsManager.Cleanup(); err != nil {
			slog.Warn("Failed to cleanup workspace", "error", err)
		}
	}()

	// Create Git client
	gitClient := git.NewClient(wsManager.GetPath())
	if err := gitClient.EnsureWorkspace(); err != nil {
		return fmt.Errorf("ensure workspace: %w", err)
	}

	// Process the local docs directory
	slog.Info("Processing local documentation", "path", g.DocsDir)

	// For local directories, we can just use the path directly
	absDocsDir, err := filepath.Abs(g.DocsDir)
	if err != nil {
		return fmt.Errorf("resolve docs directory: %w", err)
	}

	repoPaths := map[string]string{
		"docs": absDocsDir,
	}

	// Discover documentation files
	slog.Info("Discovering documentation files")
	discovery := docs.NewDiscovery(cfg.Repositories, &cfg.Build)
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return fmt.Errorf("discover docs: %w", err)
	}

	if len(docFiles) == 0 {
		slog.Warn("No documentation files found", "path", g.DocsDir)
		fmt.Println("Warning: No documentation files found")
		return nil
	}

	slog.Info("Documentation files discovered", "count", len(docFiles))

	// Generate Hugo site
	slog.Info("Generating Hugo site", "output", hugoProjectDir)
	generator := hugo.NewGenerator(cfg, hugoProjectDir)

	if err := generator.GenerateSite(docFiles); err != nil {
		return fmt.Errorf("generate site: %w", err)
	}

	// If rendering, move the public directory to the final output location
	if g.Render {
		publicDir := filepath.Join(hugoProjectDir, "public")

		// Verify the public directory exists
		if _, err := os.Stat(publicDir); os.IsNotExist(err) {
			return fmt.Errorf("hugo did not generate public directory")
		}

		// Clean the output directory if it exists
		if err := os.RemoveAll(g.Output); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean output directory: %w", err)
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(g.Output), 0o755); err != nil {
			return fmt.Errorf("failed to create output parent directory: %w", err)
		}

		// Copy public directory to final output using recursive copy
		slog.Info("Copying rendered site to output", "from", publicDir, "to", g.Output)
		if err := CopyDir(publicDir, g.Output); err != nil {
			return fmt.Errorf("failed to copy public directory: %w", err)
		}

		fmt.Printf("Static site generated successfully at: %s\n", g.Output)
	} else {
		fmt.Printf("Hugo project generated at: %s\n", g.Output)
		fmt.Println("Note: Run hugo in this directory to build the site")
	}

	return nil
}
