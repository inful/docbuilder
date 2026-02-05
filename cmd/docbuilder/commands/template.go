package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/lint"
	templating "git.home.luguber.info/inful/docbuilder/internal/templates"
)

// TemplateCmd groups template-related commands.
type TemplateCmd struct {
	List TemplateListCmd `cmd:"" help:"List available templates"`
	New  TemplateNewCmd  `cmd:"" help:"Create a new document from a template"`
}

// TemplateListCmd implements 'docbuilder template list'.
type TemplateListCmd struct {
	BaseURL string `name:"base-url" help:"Base URL for template discovery"`
}

func (t *TemplateListCmd) Run(_ *Global, root *CLI) error {
	if err := LoadEnvFile(); err == nil && root.Verbose {
		_, _ = fmt.Fprintln(os.Stderr, "Loaded environment variables from .env file")
	}

	cfg, err := loadConfigForTemplates(root.Config)
	if err != nil {
		return err
	}

	baseURL, err := ResolveTemplateBaseURL(t.BaseURL, cfg)
	if err != nil {
		return err
	}

	client := templating.NewTemplateHTTPClient()
	templates, err := templating.FetchTemplateDiscovery(context.Background(), baseURL, client)
	if err != nil {
		return err
	}

	for i, tmpl := range templates {
		_, _ = fmt.Fprintf(os.Stdout, "%d) %s\t%s\n", i+1, tmpl.Type, tmpl.URL)
	}
	return nil
}

// TemplateNewCmd implements 'docbuilder template new'.
type TemplateNewCmd struct {
	BaseURL  string   `name:"base-url" help:"Base URL for template discovery"`
	Set      []string `name:"set" help:"Override template fields (key=value)"`
	Defaults bool     `help:"Use defaults and skip prompts"`
	Yes      bool     `short:"y" help:"Auto-confirm output path and file creation"`
}

func (t *TemplateNewCmd) Run(_ *Global, root *CLI) error {
	if err := LoadEnvFile(); err == nil && root.Verbose {
		_, _ = fmt.Fprintln(os.Stderr, "Loaded environment variables from .env file")
	}

	cfg, err := loadConfigForTemplates(root.Config)
	if err != nil {
		return err
	}

	baseURL, err := ResolveTemplateBaseURL(t.BaseURL, cfg)
	if err != nil {
		return err
	}

	client := templating.NewTemplateHTTPClient()
	templates, err := templating.FetchTemplateDiscovery(context.Background(), baseURL, client)
	if err != nil {
		return err
	}

	selected, err := selectTemplate(templates, t.Yes)
	if err != nil {
		return err
	}

	page, err := templating.FetchTemplatePage(context.Background(), selected.URL, client)
	if err != nil {
		return err
	}

	schema, err := templating.ParseTemplateSchema(page.Meta.Schema)
	if err != nil {
		return err
	}
	defaults, err := templating.ParseTemplateDefaults(page.Meta.Defaults)
	if err != nil {
		return err
	}

	overrides, err := parseSetFlags(t.Set)
	if err != nil {
		return err
	}

	prompter := &cliPrompter{reader: bufio.NewReader(os.Stdin), writer: os.Stdout}
	if t.Defaults {
		prompter = nil
	}

	inputs, err := templating.ResolveTemplateInputs(schema, defaults, overrides, t.Defaults, prompter)
	if err != nil {
		return err
	}

	docsDir, err := resolveDocsDir()
	if err != nil {
		return err
	}

	nextInSequence, err := buildSequenceResolver(page, docsDir)
	if err != nil {
		return err
	}

	outputPath, err := templating.RenderOutputPath(page.Meta.OutputPath, inputs, nextInSequence)
	if err != nil {
		return err
	}

	body, err := templating.RenderTemplateBody(page.Body, inputs, nextInSequence)
	if err != nil {
		return err
	}

	fullOutputPath := filepath.Join(docsDir, outputPath)
	if !t.Yes {
		ok, confirmErr := confirmOutputPath(fullOutputPath)
		if confirmErr != nil {
			return confirmErr
		} else if !ok {
			return errors.New("aborted")
		}
	}

	writtenPath, err := templating.WriteGeneratedFile(docsDir, outputPath, body)
	if err != nil {
		return err
	}

	if err := runLintFix(writtenPath); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "Created %s\n", writtenPath)
	return nil
}

func loadConfigForTemplates(path string) (*config.Config, error) {
	if path == "" {
		return &config.Config{}, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return &config.Config{}, nil
		}
		return nil, fmt.Errorf("stat config: %w", err)
	}

	result, cfg, err := config.LoadWithResult(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	for _, warning := range result.Warnings {
		_, _ = fmt.Fprintln(os.Stderr, warning)
	}
	return cfg, nil
}

func selectTemplate(templates []templating.TemplateLink, autoYes bool) (templating.TemplateLink, error) {
	if len(templates) == 0 {
		return templating.TemplateLink{}, errors.New("no templates discovered")
	}
	if len(templates) == 1 {
		return templates[0], nil
	}

	_, _ = fmt.Fprintln(os.Stdout, "Available templates:")
	for i, tmpl := range templates {
		_, _ = fmt.Fprintf(os.Stdout, "%d) %s\n", i+1, tmpl.Type)
	}
	if autoYes {
		return templating.TemplateLink{}, errors.New("multiple templates found; selection required")
	}

	_, _ = fmt.Fprint(os.Stdout, "Select a template by number: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return templating.TemplateLink{}, fmt.Errorf("read selection: %w", err)
	}

	line = strings.TrimSpace(line)
	index, err := strconv.Atoi(line)
	if err != nil || index < 1 || index > len(templates) {
		return templating.TemplateLink{}, errors.New("invalid template selection")
	}
	return templates[index-1], nil
}

func parseSetFlags(values []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, entry := range values {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid --set value: %s", entry)
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result, nil
}

type cliPrompter struct {
	reader *bufio.Reader
	writer io.Writer
}

func (c *cliPrompter) Prompt(field templating.SchemaField) (string, error) {
	label := field.Key
	if field.Required {
		label += " (required)"
	}
	if field.Type == templating.FieldTypeStringEnum && len(field.Options) > 0 {
		_, _ = fmt.Fprintf(c.writer, "%s [%s]: ", label, strings.Join(field.Options, ", "))
	} else {
		_, _ = fmt.Fprintf(c.writer, "%s: ", label)
	}

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func resolveDocsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return filepath.Join(cwd, "docs"), nil
}

func buildSequenceResolver(page *templating.TemplatePage, docsDir string) (func(string) (int, error), error) {
	defs := make(map[string]templating.SequenceDefinition)

	if page.Meta.Sequence != "" {
		def, err := templating.ParseSequenceDefinition(page.Meta.Sequence)
		if err != nil {
			if !errors.Is(err, templating.ErrNoSequenceDefinition) {
				return nil, err
			}
		} else if def != nil {
			defs[def.Name] = *def
		}
	}

	if _, ok := defs["adr"]; !ok && strings.EqualFold(page.Meta.Type, "adr") {
		defs["adr"] = templating.SequenceDefinition{
			Name:  "adr",
			Dir:   "adr",
			Glob:  "adr-*.md",
			Regex: "^adr-(\\d{3})-",
			Width: 3,
			Start: 1,
		}
	}

	return func(name string) (int, error) {
		def, ok := defs[name]
		if !ok {
			return 0, fmt.Errorf("unknown sequence: %s", name)
		}
		return templating.ComputeNextInSequence(def, docsDir)
	}, nil
}

func confirmOutputPath(path string) (bool, error) {
	_, _ = fmt.Fprintf(os.Stdout, "Write file to %s? [y/N]: ", path)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}

func runLintFix(path string) error {
	cfg := &lint.Config{
		Format: "text",
		Fix:    true,
		Yes:    true,
	}
	linter := lint.NewLinter(cfg)
	fixer := lint.NewFixer(linter, false, false).WithAutoConfirm(true)
	_, err := fixer.Fix(path)
	return err
}
