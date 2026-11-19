package main

import "testing"

// The MCP server fetches the template URL the caller names. Without a
// check, the LLM could make the server fetch any address on the
// machine or on the network - the cloud metadata service with the
// credentials in it, the daemon's own admin port - and read the answer
// out of the tool result.
func TestCheckTemplateHost(t *testing.T) {
	state := &serverState{baseURL: "https://docs.example.com"}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "same host", url: "https://docs.example.com/templates/adr/", wantErr: false},
		{name: "same host, other path", url: "https://docs.example.com/other", wantErr: false},
		{name: "other host", url: "https://evil.example.com/x", wantErr: true},
		{name: "the metadata service", url: "http://169.254.169.254/latest/meta-data/", wantErr: true},
		{name: "the local machine", url: "http://127.0.0.1:8082/api/daemon/config", wantErr: true},
		{name: "the base host as user info", url: "http://docs.example.com@169.254.169.254/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTemplateHost(state, tt.url)
			if tt.wantErr && err == nil {
				t.Fatalf("checkTemplateHost(%q) = nil, want an error", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("checkTemplateHost(%q) = %v, want nil", tt.url, err)
			}
		})
	}
}

// Without a base URL there is no host to allow, so no template can be
// fetched.
func TestCheckTemplateHost_WithoutBaseURL(t *testing.T) {
	if err := checkTemplateHost(&serverState{}, "https://docs.example.com/x"); err == nil {
		t.Fatal("expected an error without a base URL")
	}
}
