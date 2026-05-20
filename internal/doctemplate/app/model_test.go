package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"git.home.luguber.info/inful/docbuilder/internal/doctemplate/field"
	"git.home.luguber.info/inful/docbuilder/internal/doctemplate/suggest"
	templating "git.home.luguber.info/inful/docbuilder/internal/templates"
)

func TestCollectData_RequiredBool(t *testing.T) {
	fields := []formField{{
		spec:      templating.SchemaField{Key: "Published", Type: templating.FieldTypeBool, Required: true},
		boolValue: field.NewBoolField(true, nil),
	}}

	_, err := collectData(fields, map[string]any{})
	if err == nil {
		t.Fatalf("expected validation error for required bool")
	}

	fields[0].boolValue.SetTrue()
	data, err := collectData(fields, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	value, ok := data["Published"].(bool)
	if !ok || !value {
		t.Fatalf("expected Published=true in collected data, got %#v", data["Published"])
	}
}

func TestApplySuggestion_StringList(t *testing.T) {
	f := &formField{
		spec:      templating.SchemaField{Key: "Tags", Type: templating.FieldTypeStringList},
		textValue: "alpha, be",
	}
	applySuggestion(f, "beta")
	if f.textValue != "alpha, beta" {
		t.Fatalf("unexpected list suggestion result: %q", f.textValue)
	}
}

func TestMaybeAutoSuggestSlug_FromTitle(t *testing.T) {
	m := &model{
		fields: []formField{
			{spec: templating.SchemaField{Key: "Title", Type: templating.FieldTypeString}, textValue: "My Fancy Title"},
			{spec: templating.SchemaField{Key: "Slug", Type: templating.FieldTypeString}, textValue: ""},
		},
	}

	m.maybeAutoSuggestSlug()
	if got := m.fields[1].textValue; got != "my-fancy-title" {
		t.Fatalf("expected slug my-fancy-title, got %q", got)
	}
	if m.autoSlug != "my-fancy-title" {
		t.Fatalf("expected autoSlug tracker to be updated, got %q", m.autoSlug)
	}
}

func TestMaybeAutoSuggestSlug_DoesNotOverrideManualSlug(t *testing.T) {
	m := &model{
		autoSlug: "my-fancy-title",
		fields: []formField{
			{spec: templating.SchemaField{Key: "Title", Type: templating.FieldTypeString}, textValue: "Different Title"},
			{spec: templating.SchemaField{Key: "Slug", Type: templating.FieldTypeString}, textValue: "custom-slug"},
		},
	}

	m.maybeAutoSuggestSlug()
	if got := m.fields[1].textValue; got != "custom-slug" {
		t.Fatalf("expected manual slug to be preserved, got %q", got)
	}
}

func TestHandleFormFieldInput_AllowsSpaces(t *testing.T) {
	m := &model{}
	current := &formField{spec: templating.SchemaField{Key: "Title", Type: templating.FieldTypeString}}

	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H', 'i'}})
	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t', 'h', 'e', 'r', 'e'}})

	if current.textValue != "Hi there" {
		t.Fatalf("expected spaces to be preserved, got %q", current.textValue)
	}
}

func TestHandleFormFieldInput_AllowsHandL(t *testing.T) {
	m := &model{}
	current := &formField{spec: templating.SchemaField{Key: "Title", Type: templating.FieldTypeString}}

	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m.handleFormFieldInput(current, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if current.textValue != "hello" {
		t.Fatalf("expected h/l to be preserved in text entry, got %q", current.textValue)
	}
}

func TestRenderMarkdownForPreview_ReturnsStyledContent(t *testing.T) {
	markdown := "# Hello\n\nThis is **bold** text."
	rendered := renderMarkdownForPreview(markdown, 80)
	if rendered == "" {
		t.Fatalf("expected rendered markdown, got empty string")
	}
	if rendered == markdown {
		t.Fatalf("expected rendered output to differ from raw markdown")
	}
}

func TestUpdateKey_QInStringFieldDoesNotQuit(t *testing.T) {
	m := &model{
		step: stepForm,
		fields: []formField{
			{spec: templating.SchemaField{Key: "Title", Type: templating.FieldTypeString}},
		},
		fieldIndex: 0,
	}

	_, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.fields[0].textValue != "q" {
		t.Fatalf("expected q to be inserted into text field, got %q", m.fields[0].textValue)
	}
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatalf("expected q in text field to not quit")
		}
	}
}

func TestRefreshSuggestions_GlobSuggestionFallback(t *testing.T) {
	docsDir := t.TempDir()
	mustWrite(t, filepath.Join(docsDir, "guides", "alpha.md"), "# alpha")
	mustWrite(t, filepath.Join(docsDir, "guides", "beta.md"), "# beta")

	idx, err := suggest.BuildFromDocs(docsDir)
	if err != nil {
		t.Fatalf("BuildFromDocs failed: %v", err)
	}

	m := &model{
		suggIndex: idx,
		fields: []formField{
			{
				spec: templating.SchemaField{Key: "Slug", Type: templating.FieldTypeString, GlobSuggestion: "guides/*.md"},
				// Non-matching input should still show unfiltered fallback suggestions.
				textValue: "does-not-match",
			},
		},
		fieldIndex: 0,
	}

	m.refreshSuggestions()
	if len(m.suggestions) == 0 {
		t.Fatalf("expected fallback glob suggestions, got none")
	}
}

func TestRefreshSuggestions_MergesRemoteTagTaxonomies(t *testing.T) {
	docsDir := t.TempDir()
	mustWrite(t, filepath.Join(docsDir, "docs", "alpha.md"), "# alpha")

	idx, err := suggest.BuildFromDocs(docsDir)
	if err != nil {
		t.Fatalf("BuildFromDocs failed: %v", err)
	}

	m := &model{
		suggIndex: idx,
		taxTags:   []string{"remote-tag", "alpha"},
		fields: []formField{{
			spec:      templating.SchemaField{Key: "Tags", Type: templating.FieldTypeStringList},
			textValue: "",
		}},
		fieldIndex: 0,
	}

	m.refreshSuggestions()
	if len(m.suggestions) == 0 {
		t.Fatalf("expected suggestions")
	}
	if m.suggestions[0] != "remote-tag" {
		t.Fatalf("expected taxonomy suggestions to be prioritized, got %#v", m.suggestions)
	}

	foundRemote := slices.Contains(m.suggestions, "remote-tag")
	if !foundRemote {
		t.Fatalf("expected remote taxonomy suggestion in %#v", m.suggestions)
	}
}

func TestRefreshSuggestions_TagTaxonomiesNotTruncatedToEight(t *testing.T) {
	tags := make([]string, 0, 12)
	for i := 1; i <= 12; i++ {
		tags = append(tags, "tag-"+fmt.Sprintf("%02d", i))
	}

	m := &model{
		taxTags: tags,
		fields: []formField{{
			spec: templating.SchemaField{Key: "tags", Type: templating.FieldTypeStringList},
		}},
		fieldIndex: 0,
	}

	m.refreshSuggestions()
	if len(m.suggestions) != len(tags) {
		t.Fatalf("expected %d taxonomy suggestions, got %d (%#v)", len(tags), len(m.suggestions), m.suggestions)
	}
}

func TestSuggestionWindow(t *testing.T) {
	start, end := suggestionWindow(3, 0)
	if start != 0 || end != 3 {
		t.Fatalf("expected full window for small set, got %d-%d", start, end)
	}

	start, end = suggestionWindow(20, 0)
	if start != 0 || end != 8 {
		t.Fatalf("expected leading window, got %d-%d", start, end)
	}

	start, end = suggestionWindow(20, 10)
	if start != 6 || end != 14 {
		t.Fatalf("expected centered window around cursor, got %d-%d", start, end)
	}

	start, end = suggestionWindow(20, 19)
	if start != 12 || end != 20 {
		t.Fatalf("expected trailing window, got %d-%d", start, end)
	}
}

func TestRefreshSuggestions_UsesRemoteCategoriesWhenKeyNotNormallySuggested(t *testing.T) {
	m := &model{
		taxCats: []string{"guides", "api"},
		fields: []formField{{
			spec:      templating.SchemaField{Key: "Categories", Type: templating.FieldTypeStringList},
			textValue: "g",
		}},
		fieldIndex: 0,
	}

	m.refreshSuggestions()
	if len(m.suggestions) == 0 {
		t.Fatalf("expected taxonomy suggestions")
	}
	if m.suggestions[0] != "guides" {
		t.Fatalf("expected filtered category suggestion guides, got %#v", m.suggestions)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir parent %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
