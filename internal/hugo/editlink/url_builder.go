package editlink

import (
	"fmt"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// StandardEditURLBuilder builds edit URLs using the standard forge patterns.
type StandardEditURLBuilder struct{}

// NewStandardEditURLBuilder creates a new standard URL builder.
func NewStandardEditURLBuilder() *StandardEditURLBuilder {
	return &StandardEditURLBuilder{}
}

// BuildURL constructs an edit URL using the forge package's GenerateEditURL function.
func (b *StandardEditURLBuilder) BuildURL(forgeType config.ForgeType, baseURL, fullName, branch, repoRel string) string {
	if forgeType == "" || fullName == "" {
		return ""
	}

	// Handle special case for VS Code local preview mode
	if forgeType == "vscode" {
		// fullName contains the relative path for VS Code URLs
		return fmt.Sprintf("/_edit/%s", fullName)
	}

	// Clean up the base URL
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Handle special case for Bitbucket (not currently supported in config but preserved from original)
	if strings.Contains(baseURL, "bitbucket.org") {
		return fmt.Sprintf("%s/%s/src/%s/%s?mode=edit", baseURL, fullName, branch, repoRel)
	}

	// Use the standard forge URL generation
	return forge.GenerateEditURL(forgeType, baseURL, fullName, branch, repoRel)
}
