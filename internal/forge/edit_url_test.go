package forge

import (
    "testing"
    "git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestGenerateEditURL(t *testing.T) {
    tests := []struct {
        name      string
        forgeType config.ForgeType
        baseURL   string
        fullName  string
        branch    string
        filePath  string
        want      string
    }{
        {
            name:      "GitHub basic",
            forgeType: config.ForgeGitHub,
            baseURL:   "https://github.com",
            fullName:  "org/repo",
            branch:    "main",
            filePath:  "docs/readme.md",
            want:      "https://github.com/org/repo/edit/main/docs/readme.md",
        },
        {
            name:      "GitHub trims trailing slash",
            forgeType: config.ForgeGitHub,
            baseURL:   "https://github.com/",
            fullName:  "org/repo",
            branch:    "dev",
            filePath:  "README.md",
            want:      "https://github.com/org/repo/edit/dev/README.md",
        },
        {
            name:      "GitLab basic",
            forgeType: config.ForgeGitLab,
            baseURL:   "https://gitlab.example.com",
            fullName:  "group/subgroup/repo",
            branch:    "main",
            filePath:  "guide/intro.md",
            want:      "https://gitlab.example.com/group/subgroup/repo/-/edit/main/guide/intro.md",
        },
        {
            name:      "Forgejo basic",
            forgeType: config.ForgeForgejo,
            baseURL:   "https://code.example.org",
            fullName:  "team/project",
            branch:    "feature/x",
            filePath:  "docs/section/page.md",
            want:      "https://code.example.org/team/project/_edit/feature/x/docs/section/page.md",
        },
        {
            name:      "Empty file path returns empty",
            forgeType: config.ForgeGitHub,
            baseURL:   "https://github.com",
            fullName:  "org/repo",
            branch:    "main",
            filePath:  "",
            want:      "",
        },
        {
            name:      "Unsupported forge type returns empty",
            forgeType: config.ForgeType("bitbucket"),
            baseURL:   "https://bitbucket.org",
            fullName:  "team/repo",
            branch:    "main",
            filePath:  "file.md",
            want:      "",
        },
        {
            name:      "Missing base URL returns empty",
            forgeType: config.ForgeGitHub,
            baseURL:   "",
            fullName:  "org/repo",
            branch:    "main",
            filePath:  "file.md",
            want:      "",
        },
        {
            name:      "Missing full name returns empty",
            forgeType: config.ForgeGitHub,
            baseURL:   "https://github.com",
            fullName:  "",
            branch:    "main",
            filePath:  "file.md",
            want:      "",
        },
        {
            name:      "Missing branch returns empty",
            forgeType: config.ForgeGitHub,
            baseURL:   "https://github.com",
            fullName:  "org/repo",
            branch:    "",
            filePath:  "file.md",
            want:      "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := GenerateEditURL(tt.forgeType, tt.baseURL, tt.fullName, tt.branch, tt.filePath)
            if got != tt.want {
                t.Errorf("GenerateEditURL() = %q, want %q", got, tt.want)
            }
        })
    }
}
