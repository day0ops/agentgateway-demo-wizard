package api

import (
	"io/fs"
	"net/http"
)

// WithSPA registers the embedded single-page app's static assets at "/".
// fsys's root must contain index.html.
func WithSPA(fsys fs.FS) Option {
	return func(s *Server) {
		s.spaFS = fsys
	}
}

func (s *Server) registerSPA() {
	if s.spaFS == nil {
		return
	}
	s.mux.Handle("/", http.FileServerFS(s.spaFS))
}
