package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lancelop89/youtube-trend-tracker/internal/config"
)

func TestHealthzHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthzHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestInfoHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/info", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(infoHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check content type
	expected := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expected {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ct, expected)
	}

	// Parse response
	var info map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &info); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"version", "commit", "buildTime", "goVersion", "os", "arch"}
	for _, field := range requiredFields {
		if _, ok := info[field]; !ok {
			t.Errorf("Response missing required field: %s", field)
		}
	}
}

func TestHandler_NoChannels(t *testing.T) {
	// Save original config
	originalCfg := cfg
	defer func() {
		cfg = originalCfg
	}()

	// Create config with no enabled channels
	cfg = config.DefaultConfig()
	cfg.YouTube.APIKey = "test-api-key"
	cfg.GCP.ProjectID = "test-project"
	cfg.Channels = []config.ChannelConfig{}

	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}

	expectedBody := "No channels configured"
	if body := rr.Body.String(); body != expectedBody+"\n" {
		t.Errorf("handler returned unexpected body: got %v want %v",
			body, expectedBody)
	}
}
