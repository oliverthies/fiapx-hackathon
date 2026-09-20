package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	localfs "github.com/oliverthies/fiapx-video-processor/internal/adapter/fs"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/httpjwt"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/metrics"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
	"github.com/oliverthies/fiapx-video-processor/web"
)

type Server struct {
	auth   *application.AuthService
	videos *application.VideoService
	store  *localfs.Storage
	jwt    *httpjwt.Issuer
	ready  func(context.Context) error
}

func New(auth *application.AuthService, videos *application.VideoService, store *localfs.Storage, jwt *httpjwt.Issuer, ready func(context.Context) error) http.Handler {
	s := &Server{auth: auth, videos: videos, store: store, jwt: jwt, ready: ready}
	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware)
	r.Handle("/metrics", metrics.Handler())
	r.Get("/health", s.health)
	r.Post("/auth/register", s.register)
	r.Post("/auth/login", s.login)
	r.Group(func(r chi.Router) {
		r.Use(s.authn)
		r.Post("/videos", s.upload)
		r.Get("/videos", s.list)
		r.Get("/videos/{id}", s.get)
		r.Get("/videos/{id}/zip", s.download)
	})
	r.Get("/", serveIndex)
	assets, err := fs.Sub(web.Files, "assets")
	if err != nil {
		panic(err)
	}
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	return r
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := web.Files.ReadFile("index.html")
	if err != nil {
		http.Error(w, "ui missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if s.ready != nil {
		if err := s.ready(r.Context()); err != nil {
			writeErr(w, http.StatusServiceUnavailable, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := s.auth.Register(r.Context(), in.Email, in.Password)
	if err != nil {
		mapDomain(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": user.ID, "email": user.Email})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	token, err := s.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		mapDomain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	userID := userFrom(r)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "multipart required")
		return
	}
	file, hdr, err := r.FormFile("video")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "field video is required")
		return
	}
	defer file.Close()
	corr := r.Header.Get("X-Correlation-ID")
	if corr == "" {
		corr = uuid.NewString()
	}
	job, err := s.videos.Upload(r.Context(), userID, hdr.Filename, file, corr)
	if err != nil {
		mapDomain(w, err)
		return
	}
	if hdr.Size > 0 {
		metrics.UploadBytes.Add(float64(hdr.Size))
	}
	metrics.JobsAdmitted.Inc()
	writeJSON(w, http.StatusAccepted, jobDTO(job))
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.videos.List(r.Context(), userFrom(r))
	if err != nil {
		mapDomain(w, err)
		return
	}
	out := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobDTO(j))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	job, err := s.videos.Get(r.Context(), userFrom(r), id)
	if err != nil {
		mapDomain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, jobDTO(job))
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	job, err := s.videos.Download(r.Context(), userFrom(r), id)
	if err != nil {
		mapDomain(w, err)
		return
	}
	f, err := s.store.Open(r.Context(), job.ZipPath)
	if err != nil {
		writeErr(w, http.StatusNotFound, "zip missing")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename="+id.String()+".zip")
	_, _ = io.Copy(w, f)
}

type ctxKey int

const userKey ctxKey = 1

func (s *Server) authn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		id, err := s.jwt.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil || id == uuid.Nil {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, id)))
	})
}

func userFrom(r *http.Request) uuid.UUID {
	id, _ := r.Context().Value(userKey).(uuid.UUID)
	return id
}

func jobDTO(j *domain.VideoJob) map[string]any {
	return map[string]any{
		"id":                j.ID,
		"status":            j.Status,
		"original_filename": j.OriginalFilename,
		"frame_count":       j.FrameCount,
		"error_message":     j.ErrorMessage,
		"correlation_id":    j.CorrelationID,
		"created_at":        j.CreatedAt,
		"updated_at":        j.UpdatedAt,
	}
}

func mapDomain(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail), errors.Is(err, domain.ErrInvalidPassword), errors.Is(err, domain.ErrUnsupportedMedia):
		writeErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrUserExists):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeErr(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeErr(w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrJobNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrJobNotReady):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "internal error")
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
