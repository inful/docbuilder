package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/lint"
	templating "git.home.luguber.info/inful/docbuilder/internal/templates"
)

// TemplateService wraps template operations needed by the doctemplate TUI.
type TemplateService struct{}

// TemplateRef is a simplified template discovery record.
type TemplateRef struct {
	Type string
	URL  string
	Name string
}

// NewTemplateService constructs a service instance.
func NewTemplateService() *TemplateService {
	return &TemplateService{}
}

// Discover fetches available templates from a base URL.
func (s *TemplateService) Discover(ctx context.Context, baseURL string) ([]TemplateRef, error) {
	client := templating.NewTemplateHTTPClient()
	links, err := templating.FetchTemplateDiscovery(ctx, baseURL, client)
	if err != nil {
		return nil, err
	}

	out := make([]TemplateRef, 0, len(links))
	for _, link := range links {
		out = append(out, TemplateRef{Type: link.Type, URL: link.URL, Name: link.Name})
	}
	return out, nil
}

// FetchPage fetches and parses a template page.
func (s *TemplateService) FetchPage(ctx context.Context, templateURL string) (*templating.TemplatePage, error) {
	client := templating.NewTemplateHTTPClient()
	return templating.FetchTemplatePage(ctx, templateURL, client)
}

// ParseSchema parses template schema metadata.
func (s *TemplateService) ParseSchema(raw string) (templating.TemplateSchema, error) {
	return templating.ParseTemplateSchema(raw)
}

// ParseDefaults parses template defaults metadata.
func (s *TemplateService) ParseDefaults(raw string) (map[string]any, error) {
	return templating.ParseTemplateDefaults(raw)
}

// BuildSequenceResolver creates a sequence resolver with ADR fallback.
func (s *TemplateService) BuildSequenceResolver(page *templating.TemplatePage, docsDir string) (func(string) (int, error), error) {
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
			return 0, derrors.NewError(derrors.CategoryValidation, "unknown sequence").
				WithContext("name", name).
				Build()
		}
		return templating.ComputeNextInSequence(def, docsDir)
	}, nil
}

// RenderPreview computes output path and rendered body.
func (s *TemplateService) RenderPreview(page *templating.TemplatePage, data map[string]any, nextInSequence func(string) (int, error)) (string, string, error) {
	outputPath, err := templating.RenderOutputPath(page.Meta.OutputPath, data, nextInSequence)
	if err != nil {
		return "", "", err
	}
	body, err := templating.RenderTemplateBody(page.Body, data, nextInSequence)
	if err != nil {
		return "", "", err
	}
	return outputPath, body, nil
}

// Write writes generated content to docs directory and runs lint --fix.
func (s *TemplateService) Write(docsDir, relPath, body string) (string, error) {
	writtenPath, err := templating.WriteGeneratedFile(docsDir, relPath, body)
	if err != nil {
		return "", err
	}
	if err := runLintFix(writtenPath); err != nil {
		return "", err
	}
	return writtenPath, nil
}

// ResolveDocsDir resolves docs directory from working directory.
func (s *TemplateService) ResolveDocsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, "docs"), nil
}

func runLintFix(path string) error {
	cfg := &lint.Config{Format: "text", Fix: true, Yes: true}
	linter := lint.NewLinter(cfg)
	fixer := lint.NewFixer(linter, false, false).WithAutoConfirm(true)
	_, err := fixer.Fix(path)
	return err
}
