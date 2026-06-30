package forge

import (
	"net/url"
	"strings"
)

// SplitCloneURL returns (baseURL, fullName) for a clone URL.
//
// baseURL is the scheme://host form, e.g. "https://github.com". fullName is
// the path part (without leading slash or ".git" suffix), e.g. "owner/repo"
// or "group/subgroup/repo". If the URL cannot be parsed, returns ("", "").
//
// SSH URLs (git@github.com:owner/repo.git) are normalized to HTTPS form
// first. This consolidates the two pieces of logic that were inlined in
// editlink/resolver.go (determineBaseURL + extractFullNameFromURL) into a
// single helper used by both call sites.
func SplitCloneURL(cloneURL string) (baseURL, fullName string) {
	normalized := normalizeSSH(cloneURL)

	if strings.Contains(cloneURL, "bitbucket.org") {
		// Special case: bitbucket URLs get the full name extracted without
		// scheme parsing (matches the prior editlink behavior).
		u, err := url.Parse(normalized)
		if err != nil {
			return "", ""
		}
		path := strings.Trim(u.Path, "/")
		path = strings.TrimSuffix(path, ".git")
		return "https://bitbucket.org", path
	}

	u, err := url.Parse(normalized)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", ""
	}
	baseURL = u.Scheme + "://" + u.Host
	fullName = strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	return baseURL, fullName
}

// NormalizeCloneURL converts git@host:owner/repo.git to
// https://host/owner/repo.git. Non-SSH URLs pass through unchanged.
// Exposed for callers that need the SSH-normalized form without
// splitting into (base, fullName).
func NormalizeCloneURL(repoURL string) string {
	if !strings.HasPrefix(repoURL, "git@") {
		return repoURL
	}
	rest := strings.TrimPrefix(repoURL, "git@")
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return repoURL
	}
	return "https://" + parts[0] + "/" + parts[1]
}

// normalizeSSH is the unexported form used inside SplitCloneURL.
func normalizeSSH(repoURL string) string { return NormalizeCloneURL(repoURL) }

// SplitCloneURLHelper is preserved for callers that only need a cloneURL's
// host-normalized form without splitting into (base, fullName). It is a
// thin alias for NormalizeCloneURL kept for source compatibility with
// the previous editlink.normalizeSSHURL.
func SplitCloneURLHelper(repoURL string) string { return NormalizeCloneURL(repoURL) }
