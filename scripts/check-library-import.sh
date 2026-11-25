#!/usr/bin/env bash
# Prove that the template engine can be used from outside the repository.
#
# The packages of the engine lived in internal/, and Go refuses an
# import of internal/ from another module: an application outside could
# not use the engine at all. This script builds a scratch module outside
# the repository that imports the engine, so that a package that goes
# back into internal/ is caught before it is published.
#
# Usage: scripts/check-library-import.sh
set -euo pipefail

cd "$(dirname "$0")/.."

REPO="$(pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cat > "$WORK/go.mod" <<EOF
module example.com/consumer

go 1.26.0

require git.home.luguber.info/inful/docbuilder v0.0.0

replace git.home.luguber.info/inful/docbuilder => ${REPO}
EOF

mkdir -p "$WORK/consumer"
cat > "$WORK/consumer/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"

	"git.home.luguber.info/inful/docbuilder/templates"
	"git.home.luguber.info/inful/docbuilder/templates/field"
	"git.home.luguber.info/inful/docbuilder/templates/suggest"
)

// prompter answers the questions of the engine: the interface is part
// of the public surface.
type prompter struct{}

func (prompter) Prompt(field templates.SchemaField) (string, error) {
	return "X", nil
}

func main() {
	// The engine renders a body and a path.
	body, err := templates.RenderTemplateBody("# {{ .Title }}", map[string]any{"Title": "X"}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	path, err := templates.RenderOutputPath("adr/adr-001-{{ .Slug }}.md", map[string]any{"Slug": "x"}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// The engine asks for the inputs of a template.
	schema := templates.TemplateSchema{Fields: []templates.SchemaField{
		{Key: "Title", Type: templates.FieldTypeString, Required: true},
	}}
	inputs, err := templates.ResolveTemplateInputs(schema, nil, map[string]string{"Title": "X"}, false, prompter{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// The field of a form.
	required := true
	answer := false
	flag := field.NewBoolField(required, &answer)
	if err := flag.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// The index that suggests file names.
	dir, err := os.MkdirTemp("", "consumer")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	index, err := suggest.BuildFromDocs(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(body, path, inputs, index.Suggest("x", 5))
}
EOF

cd "$WORK"
go mod tidy
go vet ./...
go run ./consumer

echo "the engine can be imported from outside the repository"
