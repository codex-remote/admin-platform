package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNonNilEncodesNilAsEmptySlice(t *testing.T) {
	if values := nonNil[string](nil); values == nil || len(values) != 0 {
		t.Fatalf("expected a non-nil empty slice, got %#v", values)
	}
}

func TestLegacyAndUnknownAPIRoutesReturnJSONNotFound(t *testing.T) {
	handler := New(nil, Config{}).Handler()
	paths := []string{
		"/api/overview",
		"/api/events",
		"/api/services",
		"/api/v1/unknown",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d with body %q", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("expected JSON response, got %q", contentType)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Error.Code != "not_found" {
				t.Fatalf("expected not_found error, got %#v", body)
			}
		})
	}
}
