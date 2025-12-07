package fmcore

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestComputeBaseFrontMatter_CreatesTitleDateAndMetadata ensures baseline fields are populated
// with expected normalization behavior and do not overwrite existing keys.
func TestComputeBaseFrontMatter_CreatesTitleDateAndMetadata(t *testing.T) {
	existing := map[string]any{"title": "Keep", "custom": "orig"}
	meta := map[string]any{"custom": "meta", "description": "Desc"}
	now := time.Date(2025, 9, 30, 12, 34, 56, 0, time.UTC)
	cfg := &config.Config{}
	fm := ComputeBaseFrontMatter("sample_page", "repo1", "github", "sectionA", meta, existing, cfg, now)

	// Title preserved (existing wins)
	if fm["title"].(string) != "Keep" {
		t.Fatalf("expected existing title preserved, got %v", fm["title"])
	}
	// Date injected (exact formatting RFC3339-like with offset - compare prefix since offset may differ under local tz logic)
	if d, ok := fm["date"].(string); !ok || len(d) < 19 || d[:19] != "2025-09-30T12:34:56" {
		t.Fatalf("expected date injected with timestamp, got %v", fm["date"])
	}
	if fm["repository"].(string) != "repo1" {
		t.Fatalf("expected repository repo1, got %v", fm["repository"])
	}
	if fm["forge"].(string) != "github" {
		t.Fatalf("expected forge github, got %v", fm["forge"])
	}
	if fm["section"].(string) != "sectionA" {
		t.Fatalf("expected section sectionA, got %v", fm["section"])
	}
	// Metadata passthrough only when missing
	if fm["custom"].(string) != "orig" { // meta should not override existing
		t.Fatalf("expected existing custom key retained, got %v", fm["custom"])
	}
	if fm["description"].(string) != "Desc" {
		t.Fatalf("expected description passthrough, got %v", fm["description"])
	}
}

// TestComputeBaseFrontMatter_TitleGeneration ensures snake/dash naming normalization.
func TestComputeBaseFrontMatter_TitleGeneration(t *testing.T) {
	fm := ComputeBaseFrontMatter("my_sample-page", "r", "", "", nil, map[string]any{}, &config.Config{}, time.Now())
	if fm["title"].(string) != "My Sample Page" {
		t.Fatalf("expected normalized title, got %v", fm["title"])
	}
}

// helper config for edit link tests
// (Edit link resolution tests moved to hugo package with consolidated resolver.)

// TestComputeBaseFrontMatterTyped mirrors ComputeBaseFrontMatter tests for the typed builder.
func TestComputeBaseFrontMatterTyped_Basic(t *testing.T) {
	existing := map[string]any{"title": "Keep", "custom": "orig"}
	meta := map[string]any{"custom": "meta", "description": "Desc"}
	now := time.Date(2025, 9, 30, 12, 34, 56, 0, time.UTC)
	cfg := &config.Config{}
	fm := ComputeBaseFrontMatterTyped("sample_page", "repo1", "github", "sectionA", meta, existing, cfg, now)

	if fm.Title != "Keep" {
		t.Fatalf("expected existing title preserved, got %v", fm.Title)
	}
	if fm.Date.IsZero() || !fm.Date.Equal(now) {
		t.Fatalf("expected date injected to now, got %v", fm.Date)
	}
	if fm.Repository != "repo1" {
		t.Fatalf("expected repository repo1, got %v", fm.Repository)
	}
	if fm.Forge != "github" {
		t.Fatalf("expected forge github, got %v", fm.Forge)
	}
	if fm.Section != "sectionA" {
		t.Fatalf("expected section sectionA, got %v", fm.Section)
	}
	// Metadata passthrough only when missing: existing wins
	if val, ok := fm.GetCustom("custom"); !ok || val != "orig" {
		t.Fatalf("expected existing custom retained, got %v", val)
	}
	if fm.Description != "Desc" {
		t.Fatalf("expected description passthrough, got %v", fm.Description)
	}
}

func TestComputeBaseFrontMatterTyped_TitleGeneration(t *testing.T) {
	fm := ComputeBaseFrontMatterTyped("my_sample-page", "r", "", "", nil, map[string]any{}, &config.Config{}, time.Now())
	if fm.Title != "My Sample Page" {
		t.Fatalf("expected normalized title, got %v", fm.Title)
	}
}
