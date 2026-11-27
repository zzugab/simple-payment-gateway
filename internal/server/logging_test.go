package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRequestLogging(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	router := chi.NewRouter()
	router.Use(requestLogging)
	router.Post("/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, 400, "invalid_json", "private-response")
	})
	router.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("private-panic")
	})

	for _, tc := range []struct {
		method, path, route, level, code string
		status                           int
	}{
		{"POST", "/orders/private-id?token=private-query", "/orders/{id}", "WARN", "invalid_json", 400},
		{"GET", "/panic", "/panic", "ERROR", "handler_panic", 500},
		{"GET", "/private-missing", "unmatched", "WARN", "", 404},
	} {
		logs.Reset()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("private-body"))
		req.Header.Set("Authorization", "private-auth")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != tc.status {
			t.Fatalf("status: got %d, want %d", res.Code, tc.status)
		}
		id := res.Header().Get("X-Request-ID")
		if id == "" {
			t.Fatal("missing request ID")
		}
		var entry map[string]any
		if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["request_id"] != id || entry["route"] != tc.route || entry["level"] != tc.level || entry["status"] != float64(tc.status) {
			t.Fatalf("unexpected log: %v", entry)
		}
		if tc.code != "" && entry["error_code"] != tc.code {
			t.Fatalf("unexpected error code: %v", entry)
		}
		if strings.Contains(logs.String(), "private-") {
			t.Fatal("sensitive data present in logs")
		}
	}
}
