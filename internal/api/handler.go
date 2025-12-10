package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"github.com/kmicac/smoothcomp-scraper/internal/scheduler"
	"github.com/kmicac/smoothcomp-scraper/internal/scraper"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
)

type Handler struct {
	config    *config.Config
	scheduler *scheduler.Scheduler
	scraper   *scraper.Scraper
}

func NewHandler(cfg *config.Config, sched *scheduler.Scheduler) *Handler {
	return &Handler{
		config:    cfg,
		scheduler: sched,
		scraper:   scraper.NewScraper(cfg),
	}
}

// HealthCheck returns the health status of the service
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}

	respondJSON(w, http.StatusOK, response)
}

// GetStatus returns the current status of the scraper
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	db := config.GetDB()

	// Get schedule config
	var scheduleConfig models.ScheduleConfig
	db.First(&scheduleConfig)

	// Get total counts
	var totalAcademies, totalAthletes int64
	db.Model(&models.Academy{}).Count(&totalAcademies)
	db.Model(&models.Athlete{}).Count(&totalAthletes)

	// Get last completed job
	var lastJob models.ScrapeJob
	db.Where("status = ?", "completed").Order("completed_at DESC").First(&lastJob)

	var lastRun *time.Time
	if lastJob.ID != 0 {
		lastRun = lastJob.CompletedAt
	}

	// Get next run from scheduler
	nextRun := h.scheduler.GetNextRun()

	response := models.StatusResponse{
		LastRun:         lastRun,
		NextRun:         nextRun,
		IsRunning:       h.scheduler.IsRunning(),
		ScheduleEnabled: scheduleConfig.Enabled,
		CronExpression:  scheduleConfig.CronExpr,
		TotalAcademies:  totalAcademies,
		TotalAthletes:   totalAthletes,
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Status retrieved successfully",
		Data:    response,
	})
}

// ScrapeAcademies triggers manual academy scraping
func (h *Handler) ScrapeAcademies(w http.ResponseWriter, r *http.Request) {
	logger.Info("Manual academy scraping triggered")

	go func() {
		if err := h.scraper.ScrapeAcademies(); err != nil {
			logger.Error("Failed to scrape academies", zap.Error(err))
		}
	}()

	respondJSON(w, http.StatusAccepted, models.APIResponse{
		Success: true,
		Message: "Academy scraping started",
	})
}

// ScrapeAthletes triggers manual athlete scraping
func (h *Handler) ScrapeAthletes(w http.ResponseWriter, r *http.Request) {
	logger.Info("Manual athlete scraping triggered")

	go func() {
		if err := h.scraper.ScrapeAthletes(); err != nil {
			logger.Error("Failed to scrape athletes", zap.Error(err))
		}
	}()

	respondJSON(w, http.StatusAccepted, models.APIResponse{
		Success: true,
		Message: "Athlete scraping started",
	})
}

// ScrapeAll triggers scraping of both academies and athletes
func (h *Handler) ScrapeAll(w http.ResponseWriter, r *http.Request) {
	logger.Info("Manual full scraping triggered")

	go func() {
		if err := h.scraper.ScrapeAll(); err != nil {
			logger.Error("Failed to scrape all", zap.Error(err))
		}
	}()

	respondJSON(w, http.StatusAccepted, models.APIResponse{
		Success: true,
		Message: "Full scraping started",
	})
}

// GetAcademies returns all academies with pagination
func (h *Handler) GetAcademies(w http.ResponseWriter, r *http.Request) {
	db := config.GetDB()

	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	country := r.URL.Query().Get("country")

	offset := (page - 1) * limit

	// Build query
	query := db.Model(&models.Academy{})
	if country != "" {
		query = query.Where("country_code = ?", country)
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get paginated results
	var academies []models.Academy
	query.Offset(offset).Limit(limit).Order("total_wins DESC").Find(&academies)

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Academies retrieved successfully",
		Data: map[string]interface{}{
			"academies": academies,
			"page":      page,
			"limit":     limit,
			"total":     total,
		},
	})
}

// GetAcademyByID returns a specific academy
func (h *Handler) GetAcademyByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	db := config.GetDB()
	var academy models.Academy

	if err := db.Where("external_id = ?", id).Preload("Athletes").First(&academy).Error; err != nil {
		respondJSON(w, http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Academy not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Academy retrieved successfully",
		Data:    academy,
	})
}

// GetAthletes returns all athletes with pagination
func (h *Handler) GetAthletes(w http.ResponseWriter, r *http.Request) {
	db := config.GetDB()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	country := r.URL.Query().Get("country")
	academyID := r.URL.Query().Get("academy_id")

	offset := (page - 1) * limit

	query := db.Model(&models.Athlete{})
	if country != "" {
		query = query.Where("country_code = ?", country)
	}
	if academyID != "" {
		query = query.Where("academy_external_id = ?", academyID)
	}

	var total int64
	query.Count(&total)

	var athletes []models.Athlete
	query.Offset(offset).Limit(limit).Preload("Academy").Order("total_wins DESC").Find(&athletes)

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Athletes retrieved successfully",
		Data: map[string]interface{}{
			"athletes": athletes,
			"page":     page,
			"limit":    limit,
			"total":    total,
		},
	})
}

// GetAthleteByID returns a specific athlete
func (h *Handler) GetAthleteByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	db := config.GetDB()
	var athlete models.Athlete

	if err := db.Where("external_id = ?", id).Preload("Academy").First(&athlete).Error; err != nil {
		respondJSON(w, http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Athlete not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Athlete retrieved successfully",
		Data:    athlete,
	})
}

// GetScheduleConfig returns the current schedule configuration
func (h *Handler) GetScheduleConfig(w http.ResponseWriter, r *http.Request) {
	db := config.GetDB()
	var scheduleConfig models.ScheduleConfig
	db.First(&scheduleConfig)

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Schedule config retrieved successfully",
		Data:    scheduleConfig,
	})
}

// UpdateScheduleConfig updates the schedule configuration
func (h *Handler) UpdateScheduleConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CronExpr string `json:"cron_expr"`
		Enabled  bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	db := config.GetDB()
	var scheduleConfig models.ScheduleConfig
	db.First(&scheduleConfig)

	scheduleConfig.CronExpr = input.CronExpr
	scheduleConfig.Enabled = input.Enabled

	if err := db.Save(&scheduleConfig).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to update schedule config",
		})
		return
	}

	// Restart scheduler with new config
	h.scheduler.UpdateSchedule(input.CronExpr)

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Schedule config updated successfully",
		Data:    scheduleConfig,
	})
}

// GetJobs returns scraping job history
func (h *Handler) GetJobs(w http.ResponseWriter, r *http.Request) {
	db := config.GetDB()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	var total int64
	db.Model(&models.ScrapeJob{}).Count(&total)

	var jobs []models.ScrapeJob
	db.Offset(offset).Limit(limit).Order("created_at DESC").Find(&jobs)

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Jobs retrieved successfully",
		Data: map[string]interface{}{
			"jobs":  jobs,
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// GetJobByID returns a specific job
func (h *Handler) GetJobByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	db := config.GetDB()
	var job models.ScrapeJob

	if err := db.First(&job, id).Error; err != nil {
		respondJSON(w, http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Job not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Job retrieved successfully",
		Data:    job,
	})
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
