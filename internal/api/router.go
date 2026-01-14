package api

import (
	"github.com/gorilla/mux"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/scheduler"
)

// NewRouter creates and configures the HTTP router
func NewRouter(cfg *config.Config, scheduler *scheduler.Scheduler) *mux.Router {
	router := mux.NewRouter()

	// Create handler instance
	handler := NewHandler(cfg, scheduler)

	// API v1 routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Health & Status
	api.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	api.HandleFunc("/status", handler.GetStatus).Methods("GET")

	// Manual scraping triggers
	api.HandleFunc("/scrape/academies", handler.ScrapeAcademies).Methods("POST")
	api.HandleFunc("/scrape/athletes", handler.ScrapeAthletes).Methods("POST")
	api.HandleFunc("/scrape/all", handler.ScrapeAll).Methods("POST")
	api.HandleFunc("/scrape/event/athletes", handler.ScrapeEventAthletes).Methods("POST")
	api.HandleFunc("/scrape/athlete/profile", handler.ScrapeAthleteProfile).Methods("POST")
	api.HandleFunc("/scrape/athletes/enrich", handler.ScrapeAthleteProfiles).Methods("POST")
	api.HandleFunc("/scrape/events/past", handler.ScrapePastEvents).Methods("POST")
	api.HandleFunc("/scrape/events/upcoming", handler.ScrapeUpcomingEvents).Methods("POST")

	// Data retrieval
	api.HandleFunc("/academies", handler.GetAcademies).Methods("GET")
	api.HandleFunc("/academies/{id}", handler.GetAcademyByID).Methods("GET")
	api.HandleFunc("/athletes", handler.GetAthletes).Methods("GET")
	api.HandleFunc("/athletes/{id}", handler.GetAthleteByID).Methods("GET")
	api.HandleFunc("/events", handler.GetEvents).Methods("GET")
	api.HandleFunc("/events/{id}", handler.GetEventByID).Methods("GET")
	api.HandleFunc("/events/{id}/details", handler.GetEventDetails).Methods("GET")

	// Schedule configuration
	api.HandleFunc("/schedule/config", handler.GetScheduleConfig).Methods("GET")
	api.HandleFunc("/schedule/config", handler.UpdateScheduleConfig).Methods("PUT")

	// Jobs history
	api.HandleFunc("/jobs", handler.GetJobs).Methods("GET")
	api.HandleFunc("/jobs/{id}", handler.GetJobByID).Methods("GET")

	// Middleware
	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)

	return router
}
