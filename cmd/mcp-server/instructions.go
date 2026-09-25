package main

// serverInstructions is the guidance string sent to the LLM during
// initialize. It tells the host's model how to use docbuilder-mcp well:
// the recommended workflow for the common doc-maintenance tasks, the safety
// model the mutating tools enforce, and the gotchas that are easy to hit
// without prior exposure to this server.
//
// Keep this in one place — every edit should be reviewed against the actual
// tool catalog so we don't drift from what's exposed.
const serverInstructions = `This server maintains documentation files inside --docs-dir (default ./docs). It does NOT build, serve, or preview docs — that is out of scope; use the docbuilder CLI for that.

Workflow for creating a doc from a template:
  1. list_templates — discover what is available
  2. describe_template {url} — see the schema + defaults
  3. resolve_template_inputs {url, inputs, use_defaults} — preview the output (no write)
  4. create_from_template {url, inputs, confirm: true} — write + auto-lint-fix

Workflow for editing an existing doc:
  1. read_doc {path} — get parsed frontmatter + body
  2. update_doc {path, content?, frontmatter_patch?, merge_strategy?, confirm: true}
  3. lint_fix {path, confirm: true} — only if lint_after was not set on update_doc

Workflow for a from-scratch doc (no template):
  1. create_doc {path, content, frontmatter?, overwrite?, lint_after?, confirm: true}

Read-only discovery tools (no confirm needed): get_config, list_templates, describe_template, resolve_template_inputs, lint_docs, read_doc.

Before create_doc / create_from_template / update_doc, read the structure://doc-page resource for the canonical schema (required + optional frontmatter, body rules, filename conventions, worked example). Do not invent your own frontmatter shape.

Before authoring or invoking templates, also read the structure://template resource for the canonical template shape (params.docbuilder.template.* fields, schema field types, output-path template variables, sequence configuration, body block rules). The schema returned by describe_template is a subset of this.

Safety:
- Every mutating tool requires confirm: true. The host will prompt the user.
- create_doc / update_doc / create_from_template / read_doc refuse to operate outside --docs-dir; path-escape attempts return an error.
- lint_docs / lint_fix accept any path; they only touch markdown files, so the practical risk is small, but treat them as unconfined.
- Every response masks Auth.token / password / key_path as "***". Do not try to bypass.

Pitfalls:
- create_from_template already runs lint-fix on the result; do not call lint_fix again immediately afterward.
- create_doc refuses to overwrite an existing file unless overwrite: true.
- update_doc fails if the file does not exist; use create_doc for new files.
- describe_template returns JSON with a top-level "schema" field (an array of fields with key/type/required) and a "defaults" object. To feed create_from_template, pass defaults as the starting point and overlay user-provided overrides into a flat {field_key: value} map.
- lint_docs is read-only; lint_fix is the version that mutates files.
- resolve_template_inputs never writes — use it to dry-run before create_from_template.
- Use merge_strategy: "merge" (default) to add/overwrite individual frontmatter keys; use "replace" to overwrite the whole frontmatter block.
`
