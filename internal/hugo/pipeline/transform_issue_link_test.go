package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The issue link must point at the tracker of the repository the page
// came from, and every forge names that page differently.
func TestGenerateIssueURL_PerForge(t *testing.T) {
	tests := []struct {
		name        string
		doc         *Document
		expectedURL string
	}{
		{
			name: "github",
			doc: &Document{
				Forge:     "github",
				SourceURL: "https://github.com/org/repo",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "https://github.com/org/repo/issues/new?title=Installing+the+daemon",
		},
		{
			name: "gitlab",
			doc: &Document{
				Forge:     "gitlab",
				SourceURL: "https://gitlab.com/group/project",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "https://gitlab.com/group/project/-/issues/new?issue%5Btitle%5D=Installing+the+daemon",
		},
		{
			name: "forgejo",
			doc: &Document{
				Forge:     "forgejo",
				SourceURL: "https://code.example.com/org/repo",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "https://code.example.com/org/repo/issues/new?title=Installing+the+daemon",
		},
		{
			name: "gitea is forgejo",
			doc: &Document{
				Forge:     "gitea",
				SourceURL: "https://gitea.example.com/org/repo",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "https://gitea.example.com/org/repo/issues/new?title=Installing+the+daemon",
		},
		{
			name: "the forge is detected from the address",
			doc: &Document{
				SourceURL: "https://gitlab.example.com/group/project",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "https://gitlab.example.com/group/project/-/issues/new?issue%5Btitle%5D=Installing+the+daemon",
		},
		{
			name: "the .git suffix is stripped",
			doc: &Document{
				Forge:     "github",
				SourceURL: "https://github.com/org/repo.git",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "https://github.com/org/repo/issues/new?title=Installing+the+daemon",
		},
		{
			name: "an ssh address becomes a web address",
			doc: &Document{
				Forge:     "github",
				SourceURL: "git@github.com:org/repo.git",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "https://github.com/org/repo/issues/new?title=Installing+the+daemon",
		},
		{
			name: "a page without a repository has no tracker",
			doc: &Document{
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "",
		},
		{
			name: "a local repository has no tracker",
			doc: &Document{
				Forge:     "local",
				SourceURL: "https://code.example.com/org/repo",
				FrontMatter: map[string]any{
					"title": "Installing the daemon",
				},
			},
			expectedURL: "",
		},
		{
			name: "a page without a title falls back to its file name",
			doc: &Document{
				Forge:     "github",
				SourceURL: "https://github.com/org/repo",
				Name:      "install-the-daemon",
				FrontMatter: map[string]any{
					"title": "",
				},
			},
			expectedURL: "https://github.com/org/repo/issues/new?title=install-the-daemon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedURL, generateIssueURL(tt.doc))
		})
	}
}

// A title with characters that mean something in a URL must not break
// the address.
func TestGenerateIssueURL_EscapesTheTitle(t *testing.T) {
	doc := &Document{
		Forge:     "github",
		SourceURL: "https://github.com/org/repo",
		FrontMatter: map[string]any{
			"title": "ADR-003: Fixed Transform & Pipeline",
		},
	}

	url := generateIssueURL(doc)
	assert.Equal(t, "https://github.com/org/repo/issues/new?title=ADR-003%3A+Fixed+Transform+%26+Pipeline", url)
}

// The transform puts the address in the front matter.
func TestAddIssueLink_SetsTheFrontMatter(t *testing.T) {
	doc := &Document{
		Forge:     "github",
		SourceURL: "https://github.com/org/repo",
		FrontMatter: map[string]any{
			"title": "Installing the daemon",
		},
	}

	transform := addIssueLink(&config.Config{})
	_, err := transform(doc)
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/org/repo/issues/new?title=Installing+the+daemon",
		doc.FrontMatter["issueURL"])
}

// An address the author put in the front matter wins.
func TestAddIssueLink_KeepsTheAuthorAddress(t *testing.T) {
	doc := &Document{
		Forge:     "github",
		SourceURL: "https://github.com/org/repo",
		FrontMatter: map[string]any{
			"title":    "Installing the daemon",
			"issueURL": "https://issues.example.com/new",
		},
	}

	transform := addIssueLink(&config.Config{})
	_, err := transform(doc)
	require.NoError(t, err)

	assert.Equal(t, "https://issues.example.com/new", doc.FrontMatter["issueURL"])
}

// A public site links to no tracker: the tracker wants a login.
func TestAddIssueLink_PublicOnlyHasNoAddress(t *testing.T) {
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			Content: config.DaemonContentConfig{PublicOnly: true},
		},
	}
	doc := &Document{
		Forge:     "github",
		SourceURL: "https://github.com/org/repo",
		FrontMatter: map[string]any{
			"title": "Installing the daemon",
		},
	}

	transform := addIssueLink(cfg)
	_, err := transform(doc)
	require.NoError(t, err)

	_, exists := doc.FrontMatter["issueURL"]
	assert.False(t, exists, "a public site must not link to the tracker")
}
