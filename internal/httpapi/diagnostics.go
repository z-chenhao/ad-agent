package httpapi

import (
	"log"
	"net/http"
	"time"

	"github.com/z-chenhao/ad-agent/internal/store"
)

type observedResponse struct {
	http.ResponseWriter
	status, bytes int
}

func (w *observedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *observedResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
func (w *observedResponse) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *observedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) diagnosticStore() *store.Store {
	if s.diagnostics != nil {
		return s.diagnostics
	}
	if s.App != nil {
		return s.App.Store
	}
	return s.Manager.Store
}
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := store.ID("request")
		r = r.WithContext(store.WithRequestID(r.Context(), id))
		w.Header().Set("X-Request-ID", id)
		out := &observedResponse{ResponseWriter: w}
		defer func() {
			status := out.status
			if status == 0 {
				status = http.StatusOK
			}
			// ServeMux patterns contain parameter names, never user IDs or query strings.
			route := r.Pattern
			if route == "" {
				route = "unmatched_or_rejected"
			}
			if err := s.diagnosticStore().RecordDiagnostic("server", store.Diagnostic{Type: "http.completed", RequestID: id, Method: r.Method, Route: route, Status: status, Bytes: out.bytes, DurationMS: time.Since(started).Milliseconds()}); err != nil {
				log.Print("private server diagnostic write failed")
			}
		}()
		next.ServeHTTP(out, r)
	})
}
