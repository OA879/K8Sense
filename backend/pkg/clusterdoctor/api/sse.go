package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

// ScanStatus handles GET /cluster-doctor/scan/:id/status. It's a
// Server-Sent Events stream — the frontend must use EventSource, not fetch,
// per K8SENSE_CONTEXT.md. Event payloads are clusterdoctor.ScanProgressEvent
// JSON; a late subscriber first receives every event already buffered for
// this scan, then live events until the scan finishes.
func (s *Server) ScanStatus(w http.ResponseWriter, r *http.Request) {
	scanID := mux.Vars(r)["id"]

	live, ok := s.lookupActive(scanID)
	if !ok {
		http.Error(w, `{"error": "scan not found"}`, http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error": "streaming unsupported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ch, replay, done := live.subscribe()

	for _, ev := range replay {
		writeSSE(w, ev)
	}

	flusher.Flush()

	if done {
		return
	}

	defer live.unsubscribe(ch)

	for {
		select {
		case ev, open := <-ch:
			if !open {
				return
			}

			writeSSE(w, ev)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, ev clusterdoctor.ScanProgressEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", data)
}
