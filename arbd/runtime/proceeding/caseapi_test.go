package proceeding

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestCaseAPIServerReportsServeFailure(t *testing.T) {
	err := serveCaseAPI(&http.Server{}, failedListener{err: errors.New("accept failed")})
	if err == nil || !strings.Contains(err.Error(), "case API server failed") {
		t.Fatalf("serveCaseAPI error = %v", err)
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
