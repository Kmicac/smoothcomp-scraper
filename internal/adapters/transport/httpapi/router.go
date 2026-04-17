package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/kmicac/smoothcomp-scraper/internal/application/ingestion"
	"github.com/kmicac/smoothcomp-scraper/internal/application/operations"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"github.com/kmicac/smoothcomp-scraper/internal/platform/httpx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Handler struct {
	ops      *operations.Service
	commands *ingestion.CommandService
}

type Logger interface {
	Info(string, ...zap.Field)
}

func Field(key string, value any) zap.Field {
	return zap.Any(key, value)
}

func NewRouter(
	cfg *platformconfig.Config,
	logger *zap.Logger,
	ops *operations.Service,
	commands *ingestion.CommandService,
) http.Handler {
	handler := &Handler{ops: ops, commands: commands}
	router := mux.NewRouter()

	for _, middleware := range []muxMiddleware{
		correlationMiddleware(cfg.Security.CorrelationHeader),
		loggingMiddleware(logger),
		internalAuthMiddleware(cfg.Security),
	} {
		router.Use(middleware)
	}

	internal := router.PathPrefix("/internal/v1").Subrouter()
	internal.HandleFunc("/health/live", handler.live).Methods(http.MethodGet)
	internal.HandleFunc("/health/ready", handler.ready).Methods(http.MethodGet)
	internal.HandleFunc("/jobs", handler.createJob).Methods(http.MethodPost)
	internal.HandleFunc("/jobs", handler.listJobs).Methods(http.MethodGet)
	internal.HandleFunc("/jobs/{id}", handler.getJob).Methods(http.MethodGet)
	internal.HandleFunc("/publications/latest", handler.latestPublication).Methods(http.MethodGet)

	return router
}

func (h *Handler) live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.ops.Liveness())
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if err := h.ops.Readiness(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Pipeline   string            `json:"pipeline"`
		Country    string            `json:"country,omitempty"`
		EventType  string            `json:"event_type,omitempty"`
		EventID    string            `json:"event_id,omitempty"`
		EventURL   string            `json:"event_url,omitempty"`
		EventName  string            `json:"event_name,omitempty"`
		ProfileID  string            `json:"profile_id,omitempty"`
		ProfileURL string            `json:"profile_url,omitempty"`
		Metadata   map[string]string `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "http.create_job", false, "invalid JSON body", err))
		return
	}

	record, err := h.commands.Enqueue(r.Context(), job.Request{
		Pipeline:      job.Pipeline(request.Pipeline),
		Trigger:       job.TriggerManual,
		CorrelationID: httpx.CorrelationIDFromContext(r.Context()),
		Country:       request.Country,
		EventType:     request.EventType,
		EventID:       request.EventID,
		EventURL:      request.EventURL,
		EventName:     request.EventName,
		ProfileID:     request.ProfileID,
		ProfileURL:    request.ProfileURL,
		Metadata:      request.Metadata,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if coreerrors.IsCode(err, coreerrors.CodeInvalidRequest) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}

	writeJSON(w, http.StatusAccepted, record)
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.ops.ListJobs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	record, err := h.ops.GetJob(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) latestPublication(w http.ResponseWriter, r *http.Request) {
	pipeline := job.Pipeline(r.URL.Query().Get("pipeline"))
	if pipeline == "" {
		writeError(w, http.StatusBadRequest, coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "http.latest_publication", false, "pipeline query parameter is required", nil))
		return
	}
	record, err := h.ops.LatestPublication(r.Context(), pipeline)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	var typed *coreerrors.Error
	if errors.As(err, &typed) {
		writeJSON(w, status, map[string]any{
			"error": map[string]any{
				"category":  typed.Category,
				"code":      typed.Code,
				"message":   typed.Message,
				"retryable": typed.Retryable,
			},
		})
		return
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"category":  coreerrors.CategoryInternal,
			"code":      coreerrors.CodeInternal,
			"message":   err.Error(),
			"retryable": false,
		},
	})
}
