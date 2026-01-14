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
	Gender            string `json:"gender"`
	ProfileURL        string `json:"profile_url"`
	AvatarURL         string `json:"avatar_url"`

	// NUEVOS CAMPOS AGREGADOS
	BirthYear       int    `json:"birth_year"`       // Año de nacimiento
	ImageURL        string `json:"image_url"`        // URL de la imagen del atleta
	AffiliationName string `json:"affiliation_name"` // Afiliación (opcional)

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
	Academy            *Academy            `json:"academy,omitempty" gorm:"foreignKey:AcademyExternalID;references:ExternalID"`
	EventRegistrations []EventRegistration `json:"event_registrations,omitempty" gorm:"foreignKey:AthleteID"`
}

// Event represents a SmoothComp event card
type Event struct {
	ID          int    `json:"id" gorm:"primaryKey"`
	ExternalID  string `json:"external_id" gorm:"index"`
	Name        string `json:"name" gorm:"not null"`
	EventURL    string `json:"event_url" gorm:"uniqueIndex;not null"`
	ImageURL    string `json:"image_url"`
	City        string `json:"city"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	DateText    string `json:"date_text"`
	DaysText    string `json:"days_text"`
	EventType   string `json:"event_type"`
	Section     string `json:"section"`

	ScrapedAt time.Time `json:"scraped_at"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// EventDetail stores extended data scraped from an event page
type EventDetail struct {
	ID                 int       `json:"id" gorm:"primaryKey"`
	EventID            string    `json:"event_id" gorm:"uniqueIndex;not null"`
	EventURL           string    `json:"event_url"`
	Name               string    `json:"name"`
	Description        string    `json:"description" gorm:"type:text"`
	StartDate          string    `json:"start_date"`
	EndDate            string    `json:"end_date"`
	ImageURL           string    `json:"image_url"`
	LocationName       string    `json:"location_name"`
	LocationCity       string    `json:"location_city"`
	LocationCountry    string    `json:"location_country"`
	LocationAddress    string    `json:"location_address"`
	OrganizerName      string    `json:"organizer_name"`
	InfoPanelsJSON     string    `json:"info_panels_json" gorm:"type:text"`
	InfoPageBlocksJSON string    `json:"info_page_blocks_json" gorm:"type:text"`
	ScrapedAt          time.Time `json:"scraped_at"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
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

// EventRegistration representa la inscripción de un atleta en un evento
type EventRegistration struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	AthleteID        uint      `json:"athlete_id" gorm:"not null;index"`
	EventID          string    `json:"event_id" gorm:"not null;index"`
	EventName        string    `json:"event_name" gorm:"not null"`
	Division         string    `json:"division" gorm:"not null"`     // Men/Women
	AgeCategory      string    `json:"age_category" gorm:"not null"` // Adults/Masters/Juveniles
	Rank             string    `json:"rank" gorm:"not null"`         // Beginner/Intermediate/Advanced
	WeightClass      string    `json:"weight_class" gorm:"not null"` // -60 kg, -65 kg
	ActualWeight     float64   `json:"actual_weight"`                // Peso real en el pesaje
	Seed             int       `json:"seed" gorm:"default:0"`        // Seed en el bracket
	Ranking          int       `json:"ranking" gorm:"default:0"`     // Ranking global
	EventCardURL     string    `json:"event_card_url"`
	RegistrationDate time.Time `json:"registration_date"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relación con atleta
	Athlete Athlete `json:"athlete,omitempty" gorm:"foreignKey:AthleteID;constraint:OnDelete:CASCADE"`
}

// TableName especifica el nombre de la tabla
func (EventRegistration) TableName() string {
	return "event_registrations"
}
