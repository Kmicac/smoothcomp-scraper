package models

import "time"

// Academy represents a BJJ academy/school
type Academy struct {
	ID          int    `json:"id" gorm:"primaryKey"`
	ExternalID  string `json:"external_id" gorm:"uniqueIndex;not null"`
	Name        string `json:"name" gorm:"not null"`
	Slug        string `json:"slug"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	LogoURL     string `json:"logo_url"`
	CoverURL    string `json:"cover_url"`
	Bio         string `json:"bio" gorm:"type:text"`
	Website     string `json:"website"`
	Instagram   string `json:"instagram"`
	Facebook    string `json:"facebook"`

	// Statistics
	TotalWins    int `json:"total_wins"`
	TotalLosses  int `json:"total_losses"`
	AthleteCount int `json:"athlete_count"`
	GoldMedals   int `json:"gold_medals"`
	SilverMedals int `json:"silver_medals"`
	BronzeMedals int `json:"bronze_medals"`

	// Metadata
	ScrapedAt time.Time `json:"scraped_at"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relationships
	Athletes []Athlete `json:"athletes,omitempty" gorm:"foreignKey:AcademyExternalID;references:ExternalID"`
}

// Athlete represents a BJJ athlete/competitor
type Athlete struct {
	ID                int    `json:"id" gorm:"primaryKey"`
	ExternalID        string `json:"external_id" gorm:"uniqueIndex;not null"`
	FirstName         string `json:"first_name" gorm:"not null"`
	LastName          string `json:"last_name" gorm:"not null"`
	FullName          string `json:"full_name"`
	AcademyExternalID string `json:"academy_external_id"`
	Nationality       string `json:"nationality"`
	CountryCode       string `json:"country_code"`
	BeltRank          string `json:"belt_rank"`
	Age               int    `json:"age"`
	ProfileURL        string `json:"profile_url"`
	AvatarURL         string `json:"avatar_url"`

	// Win Statistics
	TotalWins        int `json:"total_wins"`
	WinsBySubmission int `json:"wins_by_submission"`
	WinsByPoints     int `json:"wins_by_points"`
	WinsByDecision   int `json:"wins_by_decision"`
	WinsByDQ         int `json:"wins_by_dq"`

	// Loss Statistics
	TotalLosses        int `json:"total_losses"`
	LossesBySubmission int `json:"losses_by_submission"`
	LossesByPoints     int `json:"losses_by_points"`
	LossesByDecision   int `json:"losses_by_decision"`
	LossesByDQ         int `json:"losses_by_dq"`

	// Metadata
	ScrapedAt time.Time `json:"scraped_at"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relationships
	Academy *Academy `json:"academy,omitempty" gorm:"foreignKey:AcademyExternalID;references:ExternalID"`
}

// ScrapeJob represents a scraping job execution
type ScrapeJob struct {
	ID           int        `json:"id" gorm:"primaryKey"`
	JobType      string     `json:"job_type"` // "academies", "athletes", "all"
	Status       string     `json:"status"`   // "running", "completed", "failed"
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ItemsScraped int        `json:"items_scraped"`
	ErrorMessage string     `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

// ScheduleConfig represents the cron schedule configuration
type ScheduleConfig struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	CronExpr  string    `json:"cron_expr" gorm:"not null"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// API Response structures
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

type StatusResponse struct {
	LastRun         *time.Time `json:"last_run,omitempty"`
	NextRun         *time.Time `json:"next_run,omitempty"`
	IsRunning       bool       `json:"is_running"`
	ScheduleEnabled bool       `json:"schedule_enabled"`
	CronExpression  string     `json:"cron_expression"`
	TotalAcademies  int64      `json:"total_academies"`
	TotalAthletes   int64      `json:"total_athletes"`
}
