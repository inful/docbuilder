package lint

import (
	"fmt"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

// TemplateUIDBindingRule enforces that when a documentation template
// body uses the {{ .UID }} builtin, the body's embedded frontmatter
// binds it via `uid: "{{ .UID }}"`. Otherwise the rendered document
// will end up with a uid in the frontmatter that disagrees with the
// uids referenced in the body (e.g. a `/_uid/{{ .UID }}/` canonical
// link).
//
// The rule applies to all markdown files. Its Check function returns
// no issues for non-template files (i.e. files that do not declare
// `params.docbuilder.template.*` in their frontmatter), so the
// AppliesTo check is permissive.
type TemplateUIDBindingRule struct{}

// Name returns the rule identifier.
func (r *TemplateUIDBindingRule) Name() string {
	return ruleTemplateUIDBinding
}

// AppliesTo returns true for all documentation markdown files. The
// Check function narrows further by inspecting frontmatter for
// `params.docbuilder.template.*`.
func (r *TemplateUIDBindingRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// isTemplateFile reports whether the markdown file declares
// `params.docbuilder.template.*` in its frontmatter — i.e. it is a
// docbuilder template (as opposed to a regular document).
func isTemplateFile(data []byte) bool {
	fields, _, _, _, err := frontmatterops.Read(data)
	if err != nil {
		return false
	}
	params, ok := fields["params"].(map[string]any)
	if !ok {
		return false
	}
	db, ok := params["docbuilder"].(map[string]any)
	if !ok {
		return false
	}
	tpl, ok := db["template"].(map[string]any)
	if !ok {
		return false
	}
	// A template must declare at least a `type` or `name` to count.
	if t, _ := tpl["type"].(string); strings.TrimSpace(t) != "" {
		return true
	}
	if n, _ := tpl["name"].(string); strings.TrimSpace(n) != "" {
		return true
	}
	return false
}

// Check scans a template file's body for `{{ .UID }}` references and
// reports an issue when the embedded frontmatter does not bind `uid`
// to `{{ .UID }}`.
//
// Severity rationale: a body that references `{{ .UID }}` without
// binding it produces a rendered document whose body and frontmatter
// disagree on the document's identifier. That breaks any
// `/_uid/<uid>/` link the template was trying to emit — a real
// user-visible bug — so this is an Error, not a Warning.
func (r *TemplateUIDBindingRule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath is derived from the current lint target.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if !isTemplateFile(data) {
		return nil, nil
	}

	text := string(data)
	body, _, _, ok := extractTemplateBody(text)
	if !ok {
		// No fenced code block → not a well-formed template.
		// Other rules (frontmatter-required-fields) already cover
		// the malformed-template case.
		return nil, nil
	}

	if !hasUIDReference(body) {
		return nil, nil
	}

	// Body references {{ .UID }} — the embedded frontmatter must
	// bind `uid` to `{{ .UID }}`.
	fields, _, had, _, readErr := frontmatterops.Read([]byte(body))
	if readErr != nil {
		// Malformed inner frontmatter — other rules will flag it.
		//nolint:nilerr // reported as lint issue, not a hard error
		return nil, nil
	}
	if !had || fields == nil {
		// No inner frontmatter at all; the body can't bind uid.
		return []Issue{r.missingUIDIssue(filePath, "no embedded frontmatter")}, nil
	}

	uidRaw, present := fields["uid"]
	if !present {
		return []Issue{r.missingUIDIssue(filePath, "no `uid` field in embedded frontmatter")}, nil
	}
	uidStr, ok := uidRaw.(string)
	if !ok {
		return []Issue{r.missingUIDIssue(filePath, fmt.Sprintf("`uid` must be a string, got %T", uidRaw))}, nil
	}
	if !uidFieldIsTemplate(uidStr) {
		return []Issue{r.literalUIDIssue(filePath, uidStr)}, nil
	}
	return nil, nil
}

func (r *TemplateUIDBindingRule) missingUIDIssue(filePath, detail string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Template body references {{ .UID }} but embedded frontmatter does not bind `uid`",
		Explanation: strings.TrimSpace(strings.Join([]string{
			"This template body uses the `{{ .UID }}` builtin (e.g. in a `/_uid/{{ .UID }}/` link), but the embedded YAML frontmatter does not set `uid: \"{{ .UID }}\"`.",
			"At render time the lint fixer will generate a different uid for the frontmatter, so the body's references will point at a different uuid than the frontmatter's `uid` field.",
			"",
			"Add the binding to the embedded frontmatter:",
			"",
			"    uid: \"{{ .UID }}\"",
			"",
			"Detail: " + detail,
		}, "\n")),
		Fix: "Add `uid: \"{{ .UID }}\"` to the embedded frontmatter in the fenced code block.",
	}
}

func (r *TemplateUIDBindingRule) literalUIDIssue(filePath, literal string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Template body references {{ .UID }} but embedded frontmatter has a literal uid",
		Explanation: strings.TrimSpace(strings.Join([]string{
			"This template body uses the `{{ .UID }}` builtin, but the embedded frontmatter sets `uid` to a literal value:",
			"",
			"    uid: " + literal,
			"",
			"At render time the body's `{{ .UID }}` will resolve to a uuid that disagrees with the literal in the frontmatter.",
			"",
			"Since the rule only fires on template bodies that already use `{{ .UID }}`, the literal can be safely replaced with `uid: \"{{ .UID }}\"` so both are computed at render time.",
		}, "\n")),
		Fix: "Run: docbuilder lint --fix (replaces the literal uid with `uid: \"{{ .UID }}\"`).",
	}
}
