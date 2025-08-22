package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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

func TestIsLocal(t *testing.T) {
	// Save original value
	original := os.Getenv("GO_ENV")
	defer os.Setenv("GO_ENV", original)

	// Test local environment
	os.Setenv("GO_ENV", "local")
	if !isLocal() {
		t.Error("isLocal() should return true when GO_ENV=local")
	}

	// Test non-local environment
	os.Setenv("GO_ENV", "production")
	if isLocal() {
		t.Error("isLocal() should return false when GO_ENV!=local")
	}

	// Test unset environment
	os.Unsetenv("GO_ENV")
	if isLocal() {
		t.Error("isLocal() should return false when GO_ENV is unset")
	}
}

func TestIsValidAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{
			name:     "Valid API key",
			apiKey:   "AIzaSyA1234567890abcdefghijklmnopqrstuv",
			expected: true,
		},
		{
			name:     "Too short",
			apiKey:   "AIzaSyA123",
			expected: false,
		},
		{
			name:     "Too long",
			apiKey:   "AIzaSyA1234567890abcdefghijklmnopqrstuvwxyz1234567890",
			expected: false,
		},
		{
			name:     "Contains space",
			apiKey:   "AIzaSyA1234567890 abcdefghijklmnopqrstuv",
			expected: false,
		},
		{
			name:     "Contains newline",
			apiKey:   "AIzaSyA1234567890\nabcdefghijklmnopqrstuv",
			expected: false,
		},
		{
			name:     "Contains tab",
			apiKey:   "AIzaSyA1234567890\tabcdefghijklmnopqrstuv",
			expected: false,
		},
		{
			name:     "With dashes",
			apiKey:   "AIzaSyA-1234567890-abcdefghijklmnopqrstuv",
			expected: true,
		},
		{
			name:     "With underscores",
			apiKey:   "AIzaSyA_1234567890_abcdefghijklmnopqrstuv",
			expected: true,
		},
		{
			name:     "Invalid characters",
			apiKey:   "AIzaSyA@1234567890#abcdefghijklmnopqrstuv",
			expected: false,
		},
		{
			name:     "Empty string",
			apiKey:   "",
			expected: false,
		},
		{
			name:     "With leading/trailing spaces",
			apiKey:   "  AIzaSyA1234567890abcdefghijklmnopqrstuv  ",
			expected: true, // Should be trimmed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAPIKey(tt.apiKey)
			if result != tt.expected {
				t.Errorf("isValidAPIKey(%q) = %v, want %v", tt.apiKey, result, tt.expected)
			}
		})
	}
}

func TestGetProjectID(t *testing.T) {
	// Save original values
	origProjectID := os.Getenv("PROJECT_ID")
	origGoogleCloudProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
	defer func() {
		os.Setenv("PROJECT_ID", origProjectID)
		os.Setenv("GOOGLE_CLOUD_PROJECT", origGoogleCloudProject)
	}()

	tests := []struct {
		name                string
		projectID           string
		googleCloudProject  string
		expectedID          string
		expectError         bool
	}{
		{
			name:        "PROJECT_ID set",
			projectID:   "test-project-1",
			expectedID:  "test-project-1",
			expectError: false,
		},
		{
			name:               "GOOGLE_CLOUD_PROJECT set",
			googleCloudProject: "test-project-2",
			expectedID:         "test-project-2",
			expectError:        false,
		},
		{
			name:               "Both set, PROJECT_ID takes precedence",
			projectID:          "test-project-1",
			googleCloudProject: "test-project-2",
			expectedID:         "test-project-1",
			expectError:        false,
		},
		{
			name:        "Neither set",
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv("PROJECT_ID")
			os.Unsetenv("GOOGLE_CLOUD_PROJECT")

			// Set test values
			if tt.projectID != "" {
				os.Setenv("PROJECT_ID", tt.projectID)
			}
			if tt.googleCloudProject != "" {
				os.Setenv("GOOGLE_CLOUD_PROJECT", tt.googleCloudProject)
			}

			// Test
			id, err := getProjectID()
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if id != tt.expectedID {
					t.Errorf("getProjectID() = %v, want %v", id, tt.expectedID)
				}
			}
		})
	}
}

func TestHandler_MissingEnvVars(t *testing.T) {
	// Save original values
	origValues := map[string]string{
		"GOOGLE_CLOUD_PROJECT": os.Getenv("GOOGLE_CLOUD_PROJECT"),
		"YOUTUBE_API_KEY":      os.Getenv("YOUTUBE_API_KEY"),
		"CHANNEL_CONFIG_PATH":  os.Getenv("CHANNEL_CONFIG_PATH"),
	}
	defer func() {
		for k, v := range origValues {
			os.Setenv(k, v)
		}
	}()

	// Clear required environment variables
	os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	os.Unsetenv("YOUTUBE_API_KEY")
	os.Unsetenv("CHANNEL_CONFIG_PATH")

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

	expectedBody := "Server configuration error"
	if body := rr.Body.String(); body != expectedBody+"\n" {
		t.Errorf("handler returned unexpected body: got %v want %v",
			body, expectedBody)
	}
}