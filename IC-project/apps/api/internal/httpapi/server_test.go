package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeDB struct {
	err     error
	ctxSeen context.Context
}

func (f *fakeDB) Ping(ctx context.Context) error {
	f.ctxSeen = ctx
	return f.err
}

func newTestHandler(db pinger) http.Handler {
	return New(Deps{DB: db, WebOrigin: "http://localhost:3000", Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))})
}

func TestHealthReturnsOKWithRequestID(t *testing.T) {
	rr := httptest.NewRecorder()
	newTestHandler(&fakeDB{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok"`) {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}
}

func TestReadyReportsDatabaseState(t *testing.T) {
	rr := httptest.NewRecorder()
	newTestHandler(&fakeDB{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "ready") {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	newTestHandler(&fakeDB{err: errors.New("dial tcp 10.0.0.5: password authentication failed")}).
		ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "10.0.0.5") || strings.Contains(rr.Body.String(), "password") {
		t.Fatalf("driver error leaked: %s", rr.Body.String())
	}
}

func TestReadyUsesRequestContext(t *testing.T) {
	db := &fakeDB{}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil).WithContext(ctx)
	cancel()
	newTestHandler(db).ServeHTTP(httptest.NewRecorder(), req)
	if db.ctxSeen == nil || db.ctxSeen.Err() == nil {
		t.Fatal("ready must derive its context from the request so a client disconnect cancels the ping")
	}
}

func TestCORSAllowsOnlyConfiguredOrigin(t *testing.T) {
	h := newTestHandler(&fakeDB{})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent || rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("allowed origin: status = %d, header = %q", rr.Code, rr.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("foreign origin must not be allowed, got %q", got)
	}
}

func TestPanicInHandlerBecomesGeneric500(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := New(Deps{DB: panicDB{}, WebOrigin: "http://localhost:3000", Logger: logger})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body["error"]["code"] != "internal_error" {
		t.Fatalf("unexpected body %q (%v)", rr.Body.String(), err)
	}
}

type panicDB struct{}

func (panicDB) Ping(context.Context) error { panic("boom") }
