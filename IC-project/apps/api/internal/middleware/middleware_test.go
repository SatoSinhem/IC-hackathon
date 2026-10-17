package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func testLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestRequestIDIsGeneratedAndExposed(t *testing.T) {
	var seen string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "forged\nid")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	got := rr.Header().Get("X-Request-ID")
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(got) {
		t.Fatalf("unexpected request id %q", got)
	}
	if seen != got {
		t.Fatalf("context id %q differs from header id %q", seen, got)
	}
}

func TestRecoverReturnsGenericErrorAndLogsDetails(t *testing.T) {
	var buf bytes.Buffer
	h := Recover(testLogger(&buf))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("db password=hunter2") }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "hunter2") {
		t.Fatalf("panic details leaked to the client: %s", rr.Body.String())
	}
	var body struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Error.Code != "internal_error" {
		t.Fatalf("unexpected body %q (%v)", rr.Body.String(), err)
	}
	if !strings.Contains(buf.String(), "hunter2") || !strings.Contains(buf.String(), "panic recovered") {
		t.Fatalf("panic must be logged server-side: %s", buf.String())
	}
}

func TestRecoverDoesNotOverwriteStartedResponse(t *testing.T) {
	var buf bytes.Buffer
	h := Recover(testLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		panic("late")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusAccepted || strings.Contains(rr.Body.String(), "internal_error") {
		t.Fatalf("status = %d body = %q", rr.Code, rr.Body.String())
	}
}

func TestRecoverRepanicsOnAbortHandler(t *testing.T) {
	var buf bytes.Buffer
	h := Recover(testLogger(&buf))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("http.ErrAbortHandler must be propagated")
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestAccessLogRecordsStatusAndOmitsQuery(t *testing.T) {
	var buf bytes.Buffer
	h := AccessLog(testLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/x?token=abc123", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not JSON: %v (%s)", err, buf.String())
	}
	if entry["status"] != float64(http.StatusTeapot) || entry["path"] != "/api/v1/x" || entry["method"] != "GET" {
		t.Fatalf("unexpected entry: %v", entry)
	}
	if strings.Contains(buf.String(), "abc123") {
		t.Fatalf("query string leaked into the log: %s", buf.String())
	}
}

func TestChainOrderAndPanicStatusIsLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := testLogger(&buf)
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("x") }), RequestID(), AccessLog(logger), Recover(logger))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusInternalServerError || rr.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status = %d, request id = %q", rr.Code, rr.Header().Get("X-Request-ID"))
	}
	if !strings.Contains(buf.String(), `"status":500`) {
		t.Fatalf("access log must record the 500 produced by Recover: %s", buf.String())
	}
}
