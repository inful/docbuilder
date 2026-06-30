package frontmatterops

import (
	"bytes"
)

// UpsertFields reads content's frontmatter, calls mutate to modify
// fields, and writes the result back. It bundles the boilerplate that
// the lint/fix and daemon packages used to repeat three times (Read
// frontmatter, mutate, fix the body's leading newline if we just
// created one, Write).
//
// createIfMissing controls whether to fabricate a brand-new frontmatter
// block when content had none. Pass true for callers that legitimately
// want to introduce a frontmatter block (the UID-insertion helpers);
// pass false for callers that require an existing block to work with
// (the UID-alias helper).
//
// Behavior:
//   - mutate returns (changed=false, err=nil): short-circuit. Returns
//     (content, false, nil); no write happens.
//   - mutate returns err != nil: short-circuit. Returns the original
//     (content, false, err); no write.
//   - Read fails on malformed frontmatter: returns (content, false, err);
//     mutate is not invoked.
//   - Write fails after a successful mutate: returns (content, false, err).
//   - success: returns (newContent, true, nil). Body is preserved with
//     the style newline as separator when createIfMissing synthesizes
//     a fresh block.
//
// mutate must NOT touch body, had, or style. Only fields is mutated.
// Returning changed=true on an empty mutation is allowed but pointless;
// the helper treats it the same as a real change.
func UpsertFields(content string, createIfMissing bool, mutate func(fields map[string]any) (changed bool, err error)) (string, bool, error) {
	fields, body, had, style, err := Read([]byte(content))
	if err != nil {
		return content, false, err
	}
	if style.Newline == "" {
		style.Newline = "\n"
	}
	if fields == nil {
		fields = map[string]any{}
	}

	changed, err := mutate(fields)
	if err != nil || !changed {
		return content, false, err
	}

	if !had {
		if !createIfMissing {
			return content, false, nil
		}
		had = true
		// Frontmatter freshly synthesized: prepend exactly one newline so
		// the YAML closing delimiter doesn't fuse into the first body
		// character. Skip when body already starts with that newline.
		if len(body) > 0 && !bytes.HasPrefix(body, []byte(style.Newline)) {
			body = append([]byte(style.Newline), body...)
		} else if len(body) == 0 {
			body = append([]byte(style.Newline), body...)
		}
	}

	out, err := Write(fields, body, had, style)
	if err != nil {
		return content, false, err
	}
	return string(out), true, nil
}
