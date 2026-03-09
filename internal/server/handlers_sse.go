package server

import (
	"fmt"
	"net/http"
)

// handleSSE streams server-sent events to the client. It sends a minimal
// "update" event whenever the broadcaster fires, and exits when the request
// context is cancelled.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.broadcaster.subscribe()
	defer s.broadcaster.unsubscribe(ch)

	for {
		select {
		case <-ch:
			fmt.Fprint(w, "data: update\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
