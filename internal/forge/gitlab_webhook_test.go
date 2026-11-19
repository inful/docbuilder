package forge

import "testing"

// GitLab sends the secret in a header, not a signature. The
// comparison has to take the same time for every byte, so a caller
// cannot learn the secret by timing the answer.
func TestGitLabClient_ValidateWebhook(t *testing.T) {
	client := &GitLabClient{}
	const secret = "s3cret"

	if !client.ValidateWebhook(nil, secret, secret) {
		t.Error("the right token was rejected")
	}
	if client.ValidateWebhook(nil, "wrong", secret) {
		t.Error("a wrong token was accepted")
	}
	if client.ValidateWebhook(nil, "", secret) {
		t.Error("an empty token was accepted")
	}
}
