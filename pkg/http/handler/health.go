package handler

import (
	"net/http"
	"sync/atomic"
)

type healthHandler struct {
	isHealthy int32
}

func NewHealthHandler() *healthHandler {
	return &healthHandler{

	}
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&h.isHealthy) == 1 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (h *healthHandler) SetHealth(state bool) {
	if state {
		atomic.StoreInt32(&h.isHealthy, 1)
		return
	}

	atomic.StoreInt32(&h.isHealthy, 0)
}
