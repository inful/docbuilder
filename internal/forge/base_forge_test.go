package forge

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBaseForge_NewRequest(t *testing.T) {
	tests := []struct {
		name              string
		apiURL            string
		endpoint          string
		body              interface{}
		authPrefix        string
		customHeaders     map[string]string
		wantPath          string
		wantQuery         string
		wantAuth          string
		wantCustomHeaders map[string]string
		wantErr           bool
	}{
		{
			name:       "simple endpoint no body",
			apiURL:     "https://api.example.com/v1",
			endpoint:   "/user/repos",
			body:       nil,
			authPrefix: "Bearer ",
			wantPath:   "/v1/user/repos",
			wantAuth:   "Bearer test-token",
			wantErr:    false,
		},
		{
			name:       "endpoint with leading slash trimmed",
			apiURL:     "https://api.example.com",
			endpoint:   "/repos/owner/name",
			authPrefix: "token ",
			wantPath:   "/repos/owner/name",
			wantAuth:   "token test-token",
			wantErr:    false,
		},
		{
			name:       "endpoint with query string",
			apiURL:     "https://api.example.com/api/v1",
			endpoint:   "/user/orgs?page=2&limit=50",
			authPrefix: "Bearer ",
			wantPath:   "/api/v1/user/orgs",
			wantQuery:  "page=2&limit=50",
			wantAuth:   "Bearer test-token",
			wantErr:    false,
		},
		{
			name:       "with custom headers",
			apiURL:     "https://api.github.com",
			endpoint:   "/user",
			authPrefix: "Bearer ",
			customHeaders: map[string]string{
				"Accept":               "application/vnd.github+json",
				"X-GitHub-Api-Version": "2022-11-28",
			},
			wantPath: "/user",
			wantAuth: "Bearer test-token",
			wantCustomHeaders: map[string]string{
				"Accept":               "application/vnd.github+json",
				"X-GitHub-Api-Version": "2022-11-28",
			},
			wantErr: false,
		},
		{
			name:       "with JSON body",
			apiURL:     "https://api.example.com",
			endpoint:   "/repos",
			body:       map[string]string{"name": "test-repo"},
			authPrefix: "Bearer ",
			wantPath:   "/repos",
			wantAuth:   "Bearer test-token",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{}
			bf := NewBaseForge(client, tt.apiURL, "test-token")

			if tt.authPrefix != "" {
				bf.SetAuthHeaderPrefix(tt.authPrefix)
			}

			for k, v := range tt.customHeaders {
				bf.SetCustomHeader(k, v)
			}

			req, err := bf.NewRequest(context.Background(), http.MethodGet, tt.endpoint, tt.body)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if req.URL.Path != tt.wantPath {
				t.Errorf("NewRequest() path = %v, want %v", req.URL.Path, tt.wantPath)
			}

			if tt.wantQuery != "" && req.URL.RawQuery != tt.wantQuery {
				t.Errorf("NewRequest() query = %v, want %v", req.URL.RawQuery, tt.wantQuery)
			}

			if auth := req.Header.Get("Authorization"); auth != tt.wantAuth {
				t.Errorf("NewRequest() Authorization = %v, want %v", auth, tt.wantAuth)
			}

			if userAgent := req.Header.Get("User-Agent"); userAgent != "DocBuilder/1.0" {
				t.Errorf("NewRequest() User-Agent = %v, want DocBuilder/1.0", userAgent)
			}

			for k, wantV := range tt.wantCustomHeaders {
				if gotV := req.Header.Get(k); gotV != wantV {
					t.Errorf("NewRequest() header %s = %v, want %v", k, gotV, wantV)
				}
			}

			if tt.body != nil {
				if ct := req.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("NewRequest() Content-Type = %v, want application/json", ct)
				}
			}
		})
	}
}

func TestBaseForge_DoRequest(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		statusCode     int
		result         interface{}
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful JSON response",
			serverResponse: `{"id": 123, "name": "test"}`,
			statusCode:     200,
			result:         &map[string]interface{}{},
			wantErr:        false,
		},
		{
			name:           "successful empty response",
			serverResponse: "",
			statusCode:     204,
			result:         nil,
			wantErr:        false,
		},
		{
			name:           "error response",
			serverResponse: `{"message": "Not Found"}`,
			statusCode:     404,
			result:         &map[string]interface{}{},
			wantErr:        true,
			errContains:    "404",
		},
		{
			name:           "server error with HTML body",
			serverResponse: `<html><body>Internal Server Error</body></html>`,
			statusCode:     500,
			result:         nil,
			wantErr:        true,
			errContains:    "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := server.Client()
			bf := NewBaseForge(client, server.URL, "test-token")

			req, err := http.NewRequest(http.MethodGet, server.URL+"/test", http.NoBody)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			err = bf.DoRequest(req, tt.result)

			if (err != nil) != tt.wantErr {
				t.Errorf("DoRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("DoRequest() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestBaseForge_DoRequestWithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://api.example.com?page=2>; rel="next"`)
		w.Header().Set("X-Total-Count", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id": 1}]`))
	}))
	defer server.Close()

	client := server.Client()
	bf := NewBaseForge(client, server.URL, "test-token")

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result := []map[string]interface{}{}
	headers, err := bf.DoRequestWithHeaders(req, &result)

	if err != nil {
		t.Errorf("DoRequestWithHeaders() error = %v", err)
		return
	}

	if link := headers.Get("Link"); link == "" {
		t.Errorf("DoRequestWithHeaders() Link header missing")
	}

	if count := headers.Get("X-Total-Count"); count != "100" {
		t.Errorf("DoRequestWithHeaders() X-Total-Count = %v, want 100", count)
	}

	if len(result) != 1 {
		t.Errorf("DoRequestWithHeaders() result length = %v, want 1", len(result))
	}
}

func TestBaseForge_Integration(t *testing.T) {
	// Simulates a complete request cycle like forge clients do
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization header = %v, want Bearer prefix", auth)
		}

		if ua := r.Header.Get("User-Agent"); ua != "DocBuilder/1.0" {
			t.Errorf("User-Agent = %v, want DocBuilder/1.0", ua)
		}

		// Verify body if present
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "test-data") {
				t.Errorf("request body = %v, want to contain test-data", string(body))
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	client := server.Client()
	bf := NewBaseForge(client, server.URL, "test-token")
	bf.SetCustomHeader("Accept", "application/json")

	// Test GET
	req, err := bf.NewRequest(context.Background(), http.MethodGet, "/repos", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	var getResult map[string]bool
	if requestErr := bf.DoRequest(req, &getResult); requestErr != nil {
		t.Errorf("DoRequest() GET error = %v", requestErr)
	}

	if !getResult["success"] {
		t.Errorf("DoRequest() GET result = %v, want success=true", getResult)
	}

	// Test POST with body
	body := map[string]string{"data": "test-data"}
	req, err = bf.NewRequest(context.Background(), http.MethodPost, "/repos", body)
	if err != nil {
		t.Fatalf("NewRequest() POST error = %v", err)
	}

	var postResult map[string]bool
	if err := bf.DoRequest(req, &postResult); err != nil {
		t.Errorf("DoRequest() POST error = %v", err)
	}

	if !postResult["success"] {
		t.Errorf("DoRequest() POST result = %v, want success=true", postResult)
	}
}
