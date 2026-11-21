// archivebox-quick-add
// 2022 emschu[aet]mailbox.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License along with this program.
// If not, see <https://www.gnu.org/licenses/>.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testAPIKey = "test-api-key-123"

func setupAPIClientTest(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.Close()
		appConfig.InstanceURL = ""
		appConfig.APIKey = ""
		appSessionState.IsConnected = false
		appSessionState.ConnectionErr = nil
	})
	appConfig.initI18n()
	// a trailing slash in the configuration must not produce a broken API path
	appConfig.InstanceURL = server.URL + "/"
	appConfig.APIKey = testAPIKey
}

func assertBearerAuth(t *testing.T, request *http.Request) {
	t.Helper()
	if got := request.Header.Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("unexpected Authorization header: %q", got)
	}
}

func TestCheckArchiveBoxConnectionSuccess(t *testing.T) {
	setupAPIClientTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/auth/check_api_token" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearerAuth(t, r)
		var payload struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("cannot decode request body: %v", err)
		}
		if payload.Token != testAPIKey {
			t.Errorf("unexpected token in request body: %q", payload.Token)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "user_id": "2", "username": "admin"}`))
	})

	if err := checkArchiveBoxConnection(); err != nil {
		t.Fatalf("connection check failed: %v", err)
	}
	if !appSessionState.IsConnected {
		t.Error("connection state was not set to connected")
	}
}

func TestCheckArchiveBoxConnectionInvalidKey(t *testing.T) {
	setupAPIClientTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": false, "user_id": null, "username": null}`))
	})

	if err := checkArchiveBoxConnection(); err == nil {
		t.Fatal("expected an error for an invalid API key")
	}
	if appSessionState.IsConnected {
		t.Error("connection state must not be connected for an invalid API key")
	}
}

func TestCheckArchiveBoxConnectionEmptyKey(t *testing.T) {
	setupAPIClientTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected when the API key is empty")
	})
	appConfig.APIKey = ""

	if err := checkArchiveBoxConnection(); err == nil {
		t.Fatal("expected an error for an empty API key")
	}
}

func TestAddURLToArchiveBox(t *testing.T) {
	setupAPIClientTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cli/add" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearerAuth(t, r)
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("unexpected content type: %q", got)
		}
		var payload struct {
			URLs   []string `json:"urls"`
			Parser string   `json:"parser"`
			Tag    string   `json:"tag"`
			Depth  int      `json:"depth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("cannot decode request body: %v", err)
		}
		if len(payload.URLs) != 1 || payload.URLs[0] != "https://example.com/page" {
			t.Errorf("unexpected urls: %v", payload.URLs)
		}
		if payload.Parser != "auto" || payload.Depth != 0 {
			t.Errorf("unexpected parser/depth: %q/%d", payload.Parser, payload.Depth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "errors": [], "result": [], "stdout": "", "stderr": ""}`))
	})

	worked, err := addURLToArchiveBox("  https://example.com/page  ")
	if err != nil {
		t.Fatalf("adding url failed: %v", err)
	}
	if !worked {
		t.Error("adding url did not report success")
	}
}

func TestAddURLToArchiveBoxReportsAPIErrors(t *testing.T) {
	setupAPIClientTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": false, "errors": ["invalid url"], "result": null, "stdout": "", "stderr": ""}`))
	})

	worked, err := addURLToArchiveBox("https://example.com")
	if err == nil || !strings.Contains(err.Error(), "invalid url") {
		t.Fatalf("expected API error to be reported, got: %v", err)
	}
	if worked {
		t.Error("failed add must not report success")
	}
}

func TestIsURLAlreadyArchived(t *testing.T) {
	wantCount := 1
	setupAPIClientTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/core/snapshots" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearerAuth(t, r)
		if got := r.URL.Query().Get("url"); got != "https://example.com" && got != "https://example.com/" {
			t.Errorf("unexpected url filter: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"count": %d, "items": []}`, wantCount)))
	})
	appSessionState.IsConnected = true

	if !isURLAlreadyArchived("https://example.com") {
		t.Error("expected existing snapshot to be found")
	}
	wantCount = 0
	if isURLAlreadyArchived("https://example.com") {
		t.Error("expected no snapshot to be found")
	}
}

func TestIsURLAlreadyArchivedTrailingSlash(t *testing.T) {
	setupAPIClientTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		count := 0
		if r.URL.Query().Get("url") == "https://example.com" {
			count = 1
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"count": %d, "items": []}`, count)))
	})
	appSessionState.IsConnected = true

	if !isURLAlreadyArchived("https://example.com/") {
		t.Error("expected the normalized URL without trailing slash to be found")
	}
}
