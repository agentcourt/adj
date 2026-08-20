package proceeding

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaseAPIHealthIdentifiesRun(t *testing.T) {
	api := &caseAPIServer{rc: &runContext{cfg: Config{CaseID: "case-1", RunID: "run-1"}}}
	response := httptest.NewRecorder()
	api.handleHealth(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"case_id":"case-1"`) || !strings.Contains(response.Body.String(), `"run_id":"run-1"`) {
		t.Fatalf("health response: %d %s", response.Code, response.Body.String())
	}
}

func TestCaseAPIServerReportsServeFailure(t *testing.T) {
	err := serveCaseAPI(&http.Server{}, failedListener{err: errors.New("accept failed")})
	if err == nil || !strings.Contains(err.Error(), "case API server failed") {
		t.Fatalf("serveCaseAPI error = %v", err)
	}
}

func TestCaseAPIServerRecordsResponseFailure(t *testing.T) {
	rc := &runContext{}
	w := &responseErrorWriter{
		ResponseWriter: &failedResponseWriter{header: make(http.Header), err: errors.New("write failed")},
		rc:             rc,
	}
	writeCaseAPIJSON(w, http.StatusOK, map[string]any{"ok": true})
	err := rc.takeResponseError()
	if err == nil || !strings.Contains(err.Error(), "write case API response") {
		t.Fatalf("response error = %v", err)
	}
	if err := rc.takeResponseError(); err != nil {
		t.Fatalf("response error was not cleared: %v", err)
	}
}

func TestCaseAPIServerIgnoresResponseFailureAfterDisconnect(t *testing.T) {
	rc := &runContext{}
	requestDone := make(chan struct{})
	close(requestDone)
	w := &responseErrorWriter{
		ResponseWriter: &failedResponseWriter{header: make(http.Header), err: errors.New("write failed")},
		rc:             rc,
		requestDone:    requestDone,
	}
	writeCaseAPIJSON(w, http.StatusOK, map[string]any{"ok": true})
	if err := rc.takeResponseError(); err != nil {
		t.Fatalf("response error after disconnect = %v, want nil", err)
	}
}

type failedListener struct {
	err error
}

func (ln failedListener) Accept() (net.Conn, error) { return nil, ln.err }
func (failedListener) Close() error                 { return nil }
func (failedListener) Addr() net.Addr               { return failedAddr("failed") }

type failedAddr string

func (addr failedAddr) Network() string { return string(addr) }
func (addr failedAddr) String() string  { return string(addr) }

type failedResponseWriter struct {
	header http.Header
	err    error
}

func (w *failedResponseWriter) Header() http.Header       { return w.header }
func (*failedResponseWriter) WriteHeader(int)             {}
func (w *failedResponseWriter) Write([]byte) (int, error) { return 0, w.err }
