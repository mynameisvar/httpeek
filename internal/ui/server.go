package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"httpeek/internal/har"
	"httpeek/internal/proxy"
	"httpeek/internal/replay"
	"httpeek/internal/storage"
)

type Server struct {
	store *storage.Store
	mux   *http.ServeMux
}

func NewServer(store *storage.Store) http.Handler {
	s := &Server{store: store, mux: http.NewServeMux()}

	s.mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(staticFS))))

	s.mux.HandleFunc("/api/entries", s.handleEntries)
	s.mux.HandleFunc("/api/entry/", s.handleEntry)
	s.mux.HandleFunc("/api/replay/", s.handleReplay)
	s.mux.HandleFunc("/api/export/har", s.handleExportHAR)
	s.mux.HandleFunc("/events", s.handleEvents)
	s.mux.HandleFunc("/api/clear", s.handleClear)

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})

	return s.mux
}
func (s *Server) handleEntries(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	list, err := s.store.List(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (s *Server) handleEntry(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/entry/")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	e, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, e)
}

func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/replay/")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	res, err := replay.Replay(s.store, id, r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleExportHAR(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.List(1000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	doc := har.FromEntries(list)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=httpeek.har")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}

	ch := proxy.Subscribe()
	defer proxy.Unsubscribe(ch)

	fmt.Fprintf(w, "event: ping\ndata: ok\n\n")
	flusher.Flush()

	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: entry\ndata: %s\n\n", string(b))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.DeleteAll(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("writeJSON:", err)
	}
}
