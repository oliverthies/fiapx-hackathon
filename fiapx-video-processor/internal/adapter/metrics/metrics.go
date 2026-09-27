package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests by route, method and status",
	}, []string{"method", "path", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	}, []string{"method", "path"})

	UploadBytes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "upload_bytes_total",
		Help: "Bytes received on POST /videos",
	})

	JobsAdmitted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_admitted_total",
		Help: "Jobs accepted with HTTP 202",
	})

	JobsProcessing = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "jobs_processing",
		Help: "Jobs currently extracting frames on this worker",
	})

	JobsReady = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_ready_total",
		Help: "Jobs that finished READY on this worker",
	})

	JobsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_failed_total",
		Help: "Jobs that finished FAILED on this worker",
	})

	FFmpegDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ffmpeg_duration_seconds",
		Help:    "FFmpeg extract+zip duration",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 15, 30, 60},
	})

	GStreamerDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "gstreamer_duration_seconds",
		Help:    "GStreamer extract+zip duration",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 15, 30, 60},
	})
)

func Handler() http.Handler {
	return promhttp.Handler()
}

func Serve(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler())
	go func() {
		_ = http.ListenAndServe(addr, mux)
	}()
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/metrics") || strings.HasPrefix(r.URL.Path, "/assets/") {
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, code: 200}
		start := time.Now()
		next.ServeHTTP(sw, r)
		path := chi.RouteContext(r.Context()).RoutePattern()
		if path == "" {
			path = r.URL.Path
		}
		HTTPRequests.WithLabelValues(r.Method, path, strconv.Itoa(sw.code)).Inc()
		HTTPDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}
