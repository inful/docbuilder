# docbuilder templates

The templating engine of [DocBuilder](https://git.home.luguber.info/inful/docbuilder),
as a library. It discovers documentation templates on a documentation
site, parses them, asks for their inputs, renders them and writes the
result as a Markdown file.

The library is the part of the `doctemplate` command that has no user
interface: the terminal UI, the lint-fix after the write and everything
else that belongs to the application stay in DocBuilder.

## Install

```
go get git.home.luguber.info/inful/docbuilder/templates
```

The module is still `0.x`: the exported symbols may change until the
first stable release.

## Use

```go
package main

import (
	"context"
	"fmt"
	"log"

	"git.home.luguber.info/inful/docbuilder/templates"
)

func main() {
	ctx := context.Background()

	// The templates of a documentation site.
	links, err := templates.FetchTemplateDiscovery(ctx, "https://docs.example.com", nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, link := range links {
		fmt.Println(link.Type, link.Name, link.URL)
	}

	// One template and its metadata.
	page, err := templates.FetchTemplatePage(ctx, links[0].URL, nil)
	if err != nil {
		log.Fatal(err)
	}

	schema, err := templates.ParseTemplateSchema(page.Meta.Schema)
	if err != nil {
		log.Fatal(err)
	}
	defaults, err := templates.ParseTemplateDefaults(page.Meta.Defaults)
	if err != nil {
		log.Fatal(err)
	}

	// The inputs: the defaults first, the overrides over them, and the
	// prompter for what is missing.
	answers := map[string]string{"Author": "Ada"}
	inputs, err := templates.ResolveTemplateInputs(schema, defaults,
		map[string]string{"Title": "My document"}, false, prompter{answers})
	if err != nil {
		log.Fatal(err)
	}

	// The builtins (`.Date`, `.DateTime`, `.UID`) are resolved once, so
	// the path of the document and the uid in its body are the same.
	data := templates.WithBuiltinTemplateData(inputs)

	next := func(name string) (int, error) {
		return 1, nil // the number of the sequence, e.g. the ADR number
	}

	path, err := templates.RenderOutputPath(page.Meta.OutputPath, data, next)
	if err != nil {
		log.Fatal(err)
	}
	body, err := templates.RenderTemplateBody(page.Body, data, next)
	if err != nil {
		log.Fatal(err)
	}

	written, err := templates.WriteGeneratedFile("docs", path, body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", written)
}

// prompter answers the questions with the answers it holds: a real
// application asks the user.
type prompter map[string]string

func (p prompter) Prompt(field templates.SchemaField) (string, error) {
	answer, ok := p[field.Key]
	if !ok {
		return "", fmt.Errorf("no answer for the field %s", field.Key)
	}
	return answer, nil
}
```

## The packages

| Import path                      | What it holds                                            |
|----------------------------------|----------------------------------------------------------|
| `.../docbuilder/templates`       | The engine: discovery, parsing, inputs, rendering, writing |
| `.../docbuilder/templates/field` | The tri-state boolean field of a form                    |
| `.../docbuilder/templates/suggest` | The index that suggests file names from the existing ones |

## The API

The engine (`package templates`):

| Symbol | What it does |
|---|---|
| `NewTemplateHTTPClient` | An HTTP client with a timeout, and a guard against redirects to another host |
| `FetchTemplateDiscovery` | The templates of the site, from its taxonomy page |
| `FetchTemplatePage` | One template page, parsed |
| `ParseTemplateDiscovery` | The same, from a reader instead of the network |
| `ParseTemplatePage` | The same, from a reader instead of the network |
| `CheckTemplateHost` | Refuses a template that is not on the host of the site |
| `TemplateLink`, `TemplatePage`, `TemplateMeta` | The discovered and the parsed templates |
| `ParseTemplateSchema`, `TemplateSchema`, `SchemaField`, `FieldType` | The fields a template asks for |
| `ParseTemplateDefaults` | The default values of the fields |
| `ResolveTemplateInputs`, `Prompter` | The values of the fields: the defaults, the overrides, and the questions for what is missing |
| `ParseSequenceDefinition`, `SequenceDefinition`, `ComputeNextInSequence`, `ErrNoSequenceDefinition` | The number of the next document of a sequence (the ADR number) |
| `RenderOutputPath`, `RenderTemplateBody` | The path of the file and its content |
| `WithBuiltinTemplateData` | The builtins of a template: `.Date`, `.DateTime`, `.UID` |
| `WriteGeneratedFile` | Writes the file under the documentation directory, without overwriting a file that is there |

The field (`package field`): `TriState`, `BoolField`, `NewBoolField`.

The suggestion index (`package suggest`): `Index`, `BuildFromDocs`, and
the two methods of the index, `Suggest` and `SuggestFromGlob`.

## What the library promises

The exported symbols above are the public surface of the library. They
ship as they are, without a promise of stability: the module is `0.x`,
and a change that breaks a consumer is a change that needs a version
bump.

The two helpers the engine needs of the application - the atomic write
and the redaction of the credentials of an address - are copies inside
the library. The library does not depend on any package of DocBuilder,
so it can be used without the application.

## The checks

`scripts/check-library-import.sh` builds a scratch module outside the
repository that imports the library, so that a package that goes back
into `internal/` is caught before it is published.

## The later move

The library lives in the module of DocBuilder today. When it needs a
version of its own, it moves into its own module and the import path
changes by one segment:

```
git.home.luguber.info/inful/docbuilder/templates
    → git.home.luguber.info/inful/doctemplate/templates
```

The decision is in [ADR-024](../../docs/adr/adr-024-template-engine-library.md).
