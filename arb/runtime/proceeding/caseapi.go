package proceeding

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultCaseAPIAddr = "127.0.0.1:0"
	caseAPIHealthPath  = "/health"
)

type caseAPIServer struct {
	rc         *runContext
	server     *http.Server
	ln         net.Listener
	baseURL    string
	lawyerAPI  *lawyerAPIServer
	councilAPI *councilAPIServer
}

func startCaseAPIServer(rc *runContext, includeCouncil bool) (*caseAPIServer, error) {
	addr := strings.TrimSpace(rc.cfg.CaseAPIAddr)
	if addr == "" {
		addr = DefaultCaseAPIAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("start caseapi listener: %w", err)
	}
	api := &caseAPIServer{
		rc:        rc,
		ln:        ln,
		baseURL:   "http://" + listenerHostPort(ln.Addr()),
		lawyerAPI: newLawyerAPIServer(rc),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(caseAPIHealthPath, api.handleHealth)
	api.lawyerAPI.register(mux)
	if includeCouncil {
		api.councilAPI = newCouncilAPIServer(rc)
		api.councilAPI.register(mux)
	}
	api.server = &http.Server{Handler: mux}
	go func() {
		if err := api.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = rc.recordEvent("caseapi_error", "system", currentPhase(rc.state), map[string]any{"error": err.Error()})
		}
	}()
	return api, nil
}

func listenerHostPort(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	switch host {
	case "", "::", "0.0.0.0", "[::]":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func (api *caseAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"method_not_allowed","message":"use GET"}}` + "\n"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *caseAPIServer) Close(ctx context.Context) error {
	if api == nil || api.server == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return api.server.Shutdown(shutdownCtx)
}
