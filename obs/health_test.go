package obs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_EmptyChecks(t *testing.T) {
	h := HealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var result HealthResult
	if err := jsonDecode(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status: got %q", result.Status)
	}
}

func TestHealthHandler_AllOk(t *testing.T) {
	h := HealthHandler(map[string]HealthCheckFunc{
		"a": func(ctx context.Context) error { return nil },
		"b": func(ctx context.Context) error { return nil },
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var result HealthResult
	if err := jsonDecode(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status: got %q", result.Status)
	}
	if result.Checks["a"] != "ok" || result.Checks["b"] != "ok" {
		t.Fatalf("checks: %v", result.Checks)
	}
}

func TestHealthHandler_Unhealthy(t *testing.T) {
	h := HealthHandler(map[string]HealthCheckFunc{
		"ok":  func(ctx context.Context) error { return nil },
		"bad": func(ctx context.Context) error { return errFake },
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var result HealthResult
	if err := jsonDecode(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != "unhealthy" {
		t.Fatalf("status: got %q", result.Status)
	}
	if result.Checks["bad"] == "ok" {
		t.Fatalf("bad check should be error")
	}
}

var errFake = errors.New("fake error")

func jsonDecode(b []byte, v *HealthResult) error {
	return json.Unmarshal(b, v)
}
