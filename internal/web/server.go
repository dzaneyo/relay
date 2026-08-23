package web

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/dzaneyo/relay/internal/app"
	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
	"github.com/go-chi/chi/v5"
)

//go:embed all:dist
var frontendFS embed.FS

type Server struct{ app *app.App }

func NewServer(a *app.App) *Server { return &Server{app: a} }
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]bool{"ok": true}) })
	r.Get("/api/records", s.listRecords)
	r.Post("/api/records", s.createRecord)
	r.Get("/api/records/{id}", s.getRecord)
	r.Put("/api/records/{id}", s.updateRecord)
	r.Delete("/api/records/{id}", s.deleteRecord)
	r.Get("/api/routes", s.listRoutes)
	r.Post("/api/routes", s.createRoute)
	r.Put("/api/routes/{id}", s.updateRoute)
	r.Delete("/api/routes/{id}", s.deleteRoute)
	static, _ := fs.Sub(frontendFS, "dist")
	r.Handle("/*", http.FileServer(http.FS(static)))
	return r
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		writeError(w, 400, e)
		return false
	}
	return true
}
func statusFor(e error) int {
	if errors.Is(e, repository.ErrNotFound) {
		return 404
	}
	m := strings.ToLower(e.Error())
	if strings.Contains(m, "already exists") || strings.Contains(m, "unique constraint") || strings.Contains(m, "in use") {
		return 409
	}
	if strings.Contains(m, "required") || strings.Contains(m, "requires") || strings.Contains(m, "invalid") || strings.Contains(m, "must") || strings.Contains(m, "unsupported") || strings.Contains(m, "not found") || strings.Contains(m, "cannot") || strings.Contains(m, "nested") {
		return 400
	}
	return 500
}
func writeError(w http.ResponseWriter, status int, e error) {
	writeJSON(w, status, map[string]string{"error": e.Error()})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func redact(d *model.RecordDetail) *model.RecordDetail {
	for i := range d.Credentials {
		d.Credentials[i].SecretValue = ""
	}
	if d.Credential != nil {
		c := *d.Credential
		c.SecretValue = ""
		d.Credential = &c
	}
	return d
}
func (s *Server) listRecords(w http.ResponseWriter, r *http.Request) {
	x, e := s.app.RecordService.List(r.Context(), r.URL.Query().Get("q"))
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	writeJSON(w, 200, x)
}
func (s *Server) getRecord(w http.ResponseWriter, r *http.Request) {
	x, e := s.app.RecordService.DetailByID(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	writeJSON(w, 200, redact(x))
}
func (s *Server) createRecord(w http.ResponseWriter, r *http.Request) {
	var in model.RecordInput
	if !decode(w, r, &in) {
		return
	}
	x, e := s.app.RecordService.Create(r.Context(), in)
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	writeJSON(w, 201, redact(x))
}
func (s *Server) updateRecord(w http.ResponseWriter, r *http.Request) {
	var in model.RecordInput
	if !decode(w, r, &in) {
		return
	}
	x, e := s.app.RecordService.Update(r.Context(), chi.URLParam(r, "id"), in)
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	writeJSON(w, 200, redact(x))
}
func (s *Server) deleteRecord(w http.ResponseWriter, r *http.Request) {
	e := s.app.RecordService.Delete(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) listRoutes(w http.ResponseWriter, r *http.Request) {
	x, e := s.app.RouteService.List(r.Context())
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	writeJSON(w, 200, x)
}
func (s *Server) createRoute(w http.ResponseWriter, r *http.Request) {
	var in model.RouteInput
	if !decode(w, r, &in) {
		return
	}
	x, e := s.app.RouteService.Create(r.Context(), in)
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	writeJSON(w, 201, x)
}
func (s *Server) updateRoute(w http.ResponseWriter, r *http.Request) {
	var in model.RouteInput
	if !decode(w, r, &in) {
		return
	}
	x, e := s.app.RouteService.Update(r.Context(), chi.URLParam(r, "id"), in)
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	writeJSON(w, 200, x)
}
func (s *Server) deleteRoute(w http.ResponseWriter, r *http.Request) {
	e := s.app.RouteService.Delete(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		writeError(w, statusFor(e), e)
		return
	}
	w.WriteHeader(204)
}
