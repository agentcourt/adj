package proceeding

import (
	"context"
	"encoding/json"
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
	serveDone  chan error
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
		serveDone: make(chan error, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(caseAPIHealthPath, api.handleHealth)
	api.lawyerAPI.register(mux)
	if includeCouncil {
		api.councilAPI = newCouncilAPIServer(rc)
		api.councilAPI.register(mux)
	}
	api.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(&responseErrorWriter{ResponseWriter: w, rc: rc}, r)
	})}
	go func() {
		api.serveDone <- serveCaseAPI(api.server, ln)
	}()
	return api, nil
}

func serveCaseAPI(server *http.Server, ln net.Listener) error {
	if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("case API server failed: %w", err)
	}
	return nil
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
		writeCaseAPIJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": apiError("method_not_allowed", "use GET"),
		})
		return
	}
	writeCaseAPIJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"case_id": api.rc.cfg.CaseID,
		"run_id":  api.rc.cfg.RunID,
	})
}

func (api *caseAPIServer) Close(ctx context.Context) error {
	if api == nil || api.server == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	shutdownErr := api.server.Shutdown(shutdownCtx)
	return errors.Join(shutdownErr, <-api.serveDone, api.rc.takeResponseError())
}

type responseErrorWriter struct {
	http.ResponseWriter
	rc *runContext
}

func (w *responseErrorWriter) recordResponseError(err error) {
	w.rc.recordResponseError(err)
}

func (rc *runContext) recordResponseError(err error) {
	if err == nil {
		return
	}
	rc.responseErrMu.Lock()
	defer rc.responseErrMu.Unlock()
	rc.responseErr = errors.Join(rc.responseErr, fmt.Errorf("write case API response: %w", err))
}

func (rc *runContext) takeResponseError() error {
	rc.responseErrMu.Lock()
	defer rc.responseErrMu.Unlock()
	err := rc.responseErr
	rc.responseErr = nil
	return err
}

func writeCaseAPIJSON(w http.ResponseWriter, status int, value map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		recorder, ok := w.(interface{ recordResponseError(error) })
		if !ok {
			panic(fmt.Errorf("write case API response: %w", err))
		}
		recorder.recordResponseError(err)
	}
}
