package config

import (
	"os"
	"strings"
)

// expandEnv expands environment variable references in a config file.
//
// Two forms are recognized:
//
//	${VAR}  always expanded; an unset variable becomes the empty string
//	$VAR    expanded only when the variable is set in the environment
//
// The distinction matters. A config value may legitimately contain a
// literal dollar sign — "Cost is $5", "p@$ecret" — and os.ExpandEnv eats
// it, silently changing the value. Only the braced form is unambiguous, so
// only the braced form is expanded unconditionally.
//
// The bare form is still supported for the common case where the variable
// really is set; when it is not set the reference is left alone instead of
// being replaced by nothing.
func expandEnv(s string) string {
	if !strings.ContainsRune(s, '$') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '$' {
			b.WriteByte(c)
			i++
			continue
		}
		// "${" — the braced form.
		if i+1 < len(s) && s[i+1] == '{' {
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				// Unterminated reference: leave the rest as written.
				b.WriteString(s[i:])
				return b.String()
			}
			name := s[i+2 : i+2+end]
			b.WriteString(os.Getenv(name))
			i += 2 + end + 1
			continue
		}
		// "$VAR" — the bare form.
		name, width := shellName(s[i+1:])
		if name == "" {
			// Not a variable reference: a literal dollar sign.
			b.WriteByte(c)
			i++
			continue
		}
		value, ok := os.LookupEnv(name)
		if !ok {
			// Unset: leave the reference as written rather than dropping it.
			b.WriteString(s[i : i+1+width])
			i += 1 + width
			continue
		}
		b.WriteString(value)
		i += 1 + width
	}
	return b.String()
}

// shellName reads a shell-style variable name from the start of s and
// returns the name with the number of bytes it spans. An empty name means
// s does not start with a variable reference.
//
// A name must start with a letter or an underscore, so "$5" is a literal
// dollar sign followed by a digit rather than a variable called "5".
func shellName(s string) (string, int) {
	if s == "" {
		return "", 0
	}
	if !isShellNameStart(s[0]) {
		return "", 0
	}
	i := 1
	for i < len(s) && isShellNameChar(s[i]) {
		i++
	}
	return s[:i], i
}

// isShellNameStart reports whether c may start a shell variable name.
func isShellNameStart(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c == '_':
		return true
	default:
		return false
	}
}

// isShellNameChar reports whether c may appear in a shell variable name.
func isShellNameChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_':
		return true
	default:
		return false
	}
}
