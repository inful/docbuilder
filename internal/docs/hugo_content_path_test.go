package docs

import (
	"path/filepath"
	"testing"
)

// TestHugoContentPath_LowercasesAllComponents asserts the single source
// of truth for the content-tree path normalizes every component to
// lowercase. This is the property the case-collision bug (Drift/ vs
// drift/ on case-insensitive filesystems) relied on.
//
// All code that writes a file into the content/ tree MUST go through
// HugoContentPath (or DocFile.GetHugoPath, which delegates to it) so
// that the doc-copy stage and the index generators agree on the case of
// every path component.
func TestHugoContentPath_LowercasesAllComponents(t *testing.T) {
	cases := []struct {
		name       string
		forge      string
		group      string
		repository string
		section    string
		fileName   string
		ext        string
		isSingle   bool
		want       string
	}{
		{
			name:  "single repo, lowercase components",
			forge: "", group: "", repository: "drift", section: "img/nett", fileName: "page", ext: ".md", isSingle: true,
			want: filepath.Join("content", "img", "nett", "page.md"),
		},
		{
			name:  "multi repo, mixed case",
			forge: "", group: "", repository: "Drift", section: "Img/Nett", fileName: "Page", ext: ".md", isSingle: false,
			want: filepath.Join("content", "drift", "img", "nett", "page.md"),
		},
		{
			name:  "multi repo with forge namespace, mixed case",
			forge: "GitHub", group: "", repository: "Drift", section: "Img", fileName: "Page", ext: ".md", isSingle: false,
			want: filepath.Join("content", "github", "drift", "img", "page.md"),
		},
		{
			name:  "multi repo with group for collision resolution",
			forge: "", group: "acme", repository: "drift", section: "img", fileName: "page", ext: ".md", isSingle: false,
			want: filepath.Join("content", "acme", "drift", "img", "page.md"),
		},
		{
			name:  "multi repo with forge and group",
			forge: "GitHub", group: "acme", repository: "drift", section: "img", fileName: "page", ext: ".md", isSingle: false,
			want: filepath.Join("content", "github", "acme", "drift", "img", "page.md"),
		},
		{
			name:  "index name becomes _index",
			forge: "", group: "", repository: "Drift", section: "", fileName: "index", ext: ".md", isSingle: false,
			want: filepath.Join("content", "drift", "_index.md"),
		},
		{
			name:  "single-repo mode skips the repository segment",
			forge: "", group: "", repository: "Drift", section: "img", fileName: "page", ext: ".md", isSingle: true,
			want: filepath.Join("content", "img", "page.md"),
		},
		{
			name:  "empty section is omitted",
			forge: "", group: "", repository: "Drift", section: "", fileName: "page", ext: ".md", isSingle: false,
			want: filepath.Join("content", "drift", "page.md"),
		},
		{
			name:  "extension is preserved as given",
			forge: "", group: "", repository: "Drift", section: "img", fileName: "pic", ext: ".PNG", isSingle: false,
			want: filepath.Join("content", "drift", "img", "pic.PNG"),
		},
		{
			name:  "no namespace, no section, root-level file",
			forge: "", group: "", repository: "", section: "", fileName: "page", ext: ".md", isSingle: true,
			want: filepath.Join("content", "page.md"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HugoContentPath(tc.forge, tc.group, tc.repository, tc.section, tc.fileName, tc.ext, tc.isSingle)
			if got != tc.want {
				t.Errorf("HugoContentPath(%q, %q, %q, %q, %q, %q, %v) = %q, want %q",
					tc.forge, tc.group, tc.repository, tc.section, tc.fileName, tc.ext, tc.isSingle, got, tc.want)
			}
		})
	}
}

// TestHugoContentPath_AgreesWithGetHugoPath asserts that DocFile.GetHugoPath
// and the free HugoContentPath function return identical results, so the
// two callers (doc-copy stage and index generators) cannot drift.
func TestHugoContentPath_AgreesWithGetHugoPath(t *testing.T) {
	df := &DocFile{
		Forge:        "GitHub",
		Group:        "acme",
		Repository:   "Drift",
		Section:      "Img/Nett",
		Name:         "Page",
		Extension:    ".md",
		RelativePath: "Img/Nett/Page.md",
	}
	cases := []bool{true, false}
	for _, isSingle := range cases {
		gotFromMethod := df.GetHugoPath(isSingle)
		gotFromFunc := HugoContentPath(df.Forge, df.Group, df.Repository, df.Section, df.Name, df.Extension, isSingle)
		if gotFromMethod != gotFromFunc {
			t.Errorf("isSingle=%v: GetHugoPath=%q HugoContentPath=%q (must agree)",
				isSingle, gotFromMethod, gotFromFunc)
		}
	}
}
