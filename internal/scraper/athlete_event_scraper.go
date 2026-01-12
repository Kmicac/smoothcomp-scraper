package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SmoothCompAPIResponse representa la respuesta de la API de participantes
type SmoothCompAPIResponse struct {
	Participants []Participant `json:"participants"`
	Categories   []Category    `json:"categories"`
}

// Participant representa un grupo de participantes por categoría
type Participant struct {
	BracketID     int64          `json:"bracket_id"`
	EntryID       int64          `json:"entry_id"`
	EventID       int64          `json:"event_id"`
	ID            int64          `json:"id"`
	Name          string         `json:"name"` // e.g., "Men / Adults / Beginner / -60 kg"
	Registrations []Registration `json:"registrations"`
}

// Registration representa la inscripción de un atleta individual
type Registration struct {
	ID              int64         `json:"id"`
	Approved        int           `json:"approved"`
	Status          string        `json:"status"`
	ClubID          int64         `json:"club_id"`
	AffiliationID   int64         `json:"affiliation_id"`
	SeedPosition    *int          `json:"seed_position"`
	AffiliationName string        `json:"affiliationName"`
	Age             int           `json:"age"`
	Birth           string        `json:"birth"`
	BracketSeed     *int          `json:"bracket_seed"`
	Categories      []RegCategory `json:"categories"`
	ClubName        string        `json:"clubName"`
	CountryCode     string        `json:"cn"`
	Country         string        `json:"country"`
	EventGroupID    int64         `json:"event_group_id"`
	FirstName       string        `json:"firstname"`
	Gender          string        `json:"gender"`
	LastName        string        `json:"lastname"`
	MiddleName      string        `json:"middle_name"`
	ProfileImage    string        `json:"profile_image"`
	ProfileImageID  int64         `json:"profile_image_id"`
	PublicNote      string        `json:"public_note"`
	ScoringAthlete  int           `json:"scoring_athlete"`
	Status2         string        `json:"status2"`
	TeamName        *string       `json:"teamName"`
	Trashed         int           `json:"trashed"`
	UserID          int64         `json:"user_id"`
}

// RegCategory representa una categoría del atleta (peso, edad, etc.)
type RegCategory struct {
	EventRegistrationID int64    `json:"event_registration_id"`
	CategoryValueID     int64    `json:"category_value_id"`
	EventCategoryID     int64    `json:"event_category_id"`
	SortOrder           int      `json:"sort_order"`
	EstimatedWeight     *float64 `json:"estimated_weight"`
	WeightMeasured      *string  `json:"weight_measured"` // String porque puede venir como "60.90"
}

// FlexibleFloat permite parsear numeros o strings numericas (o null).
type FlexibleFloat struct {
	Value *float64
}

func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		f.Value = nil
		return nil
	}

	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		f.Value = &number
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" {
			f.Value = nil
			return nil
		}
		parsed, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return fmt.Errorf("invalid float string: %q", str)
		}
		f.Value = &parsed
		return nil
	}

	return fmt.Errorf("invalid float value: %s", string(data))
}

// Category representa las categorías del evento
type Category struct {
	EventCategoryID    int64    `json:"event_category_id"`
	CategoryName       string   `json:"category_name"`
	Datatype           string   `json:"datatype"`
	DatatypeWeightUnit string   `json:"datatype_weight_unit"`
	ID                 int64    `json:"id"`
	Name               string   `json:"name"`
	WeightMaximum      *FlexibleFloat `json:"weight_maximum"`
	WeightMinimum      *FlexibleFloat `json:"weight_minimum"`
}

// AthleteEventData representa los datos de un atleta extraídos del evento
type AthleteEventData struct {
	SmoothCompID    string
	FirstName       string
	LastName        string
	MiddleName      string
	FullName        string
	Country         string
	CountryCode     string
	BirthYear       int
	Age             int
	AcademyName     string
	AffiliationName string
	ProfileURL      string
	ImageURL        string
	Division        string
	AgeCategory     string
	Rank            string
	WeightClass     string
	ActualWeight    float64
	Seed            int
	Ranking         int
	Gender          string
}

// ScrapeEventAthletes extrae todos los atletas de un evento usando la API de SmoothComp
func (s *Scraper) ScrapeEventAthletes(eventID string, eventName string, eventURL string) error {
	logger.Info("Iniciando scraping de atletas del evento via API",
		zap.String("event_id", eventID),
		zap.String("event_name", eventName),
		zap.String("event_url", eventURL))

	subdomain := "smoothcomp.com"
	if eventURL != "" {
		subdomain = ExtractSubdomainFromURL(eventURL)
	} else {
		subdomain = s.DetectEventSubdomain(eventID)
	}

	apiURL := BuildAPIURL(subdomain, eventID)

	logger.Debug("API URL", zap.String("url", apiURL), zap.String("subdomain", subdomain))

	// Crear cliente HTTP con timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Crear request
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	// Headers importantes
	req.Header.Set("User-Agent", s.config.Scraper.UserAgent)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/json")

	// Hacer el request
	logger.Debug("Realizando request a API")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error haciendo request: %w", err)
	}
	defer resp.Body.Close()

	// Verificar status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API retornó status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Leer body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error leyendo response: %w", err)
	}

	logger.Debug("Response recibido", zap.Int("bytes", len(bodyBytes)))

	// Parsear JSON
	var apiResponse SmoothCompAPIResponse
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return fmt.Errorf("error parseando JSON: %w", err)
	}

	logger.Info("API response parseado",
		zap.Int("categories", len(apiResponse.Participants)),
		zap.Int("category_definitions", len(apiResponse.Categories)))

	// Procesar todos los atletas
	var athletes []AthleteEventData
	totalRegistrations := 0

	for _, participant := range apiResponse.Participants {
		// participant.Name contiene: "Men / Adults / Beginner / -60 kg"
		division, ageCategory, rank, weightClass := parseCategory(participant.Name)

		logger.Debug("Procesando categoría",
			zap.String("category", participant.Name),
			zap.Int("athletes", len(participant.Registrations)))

		for _, reg := range participant.Registrations {
			totalRegistrations++

			// Convertir Registration a AthleteEventData
			athlete := AthleteEventData{
				SmoothCompID:    strconv.FormatInt(reg.UserID, 10), // UserID es el ID único del atleta
				FirstName:       reg.FirstName,
				LastName:        reg.LastName,
				MiddleName:      reg.MiddleName,
				Country:         reg.Country,
				CountryCode:     strings.ToUpper(reg.CountryCode),
				Age:             reg.Age,
				AcademyName:     reg.ClubName,
				AffiliationName: reg.AffiliationName,
				ImageURL:        reg.ProfileImage,
				Division:        division,
				AgeCategory:     ageCategory,
				Rank:            rank,
				WeightClass:     weightClass,
				Gender:          reg.Gender,
			}

			// Construir nombre completo
			nameParts := []string{reg.FirstName}
			if reg.MiddleName != "" {
				nameParts = append(nameParts, reg.MiddleName)
			}
			nameParts = append(nameParts, reg.LastName)
			athlete.FullName = strings.Join(nameParts, " ")

			// Construir profile URL
			athlete.ProfileURL = fmt.Sprintf("https://smoothcomp.com/en/profile/%d", reg.UserID)

			// Parsear año de nacimiento
			if reg.Birth != "" {
				if year, err := strconv.Atoi(reg.Birth); err == nil {
					athlete.BirthYear = year
				}
			}

			// Extraer seed position
			if reg.SeedPosition != nil {
				athlete.Seed = *reg.SeedPosition
			}

			// Extraer peso medido de las categorías
			for _, cat := range reg.Categories {
				if cat.WeightMeasured != nil && *cat.WeightMeasured != "" {
					if weight, err := strconv.ParseFloat(*cat.WeightMeasured, 64); err == nil {
						athlete.ActualWeight = weight
						break
					}
				}
			}

			// Solo agregar si tenemos los datos mínimos requeridos
			if athlete.SmoothCompID != "" && athlete.FullName != "" {
				athletes = append(athletes, athlete)
			}
		}
	}

	logger.Info("Atletas extraídos de la API",
		zap.Int("total_registrations", totalRegistrations),
		zap.Int("valid_athletes", len(athletes)))

	// Guardar atletas en la base de datos
	logger.Info("Guardando atletas en la base de datos", zap.Int("total", len(athletes)))

	savedCount := 0
	for _, athlete := range athletes {
		if err := s.saveAthleteFromEvent(athlete, eventID, eventName); err != nil {
			logger.Error("Error guardando atleta",
				zap.String("name", athlete.FullName),
				zap.Error(err))
		} else {
			savedCount++
		}
	}

	logger.Info("Scraping de evento completado",
		zap.String("event_id", eventID),
		zap.Int("saved", savedCount),
		zap.Int("total", len(athletes)))

	return nil
}

// parseCategory extrae división, categoría de edad, rank y peso de la categoría
// Ejemplo: "Men / Adults / Beginner / -60 kg"
func parseCategory(category string) (division, ageCategory, rank, weightClass string) {
	parts := strings.Split(category, "/")
	if len(parts) >= 4 {
		division = strings.TrimSpace(parts[0])    // Men, Women, Boys, Girls
		ageCategory = strings.TrimSpace(parts[1]) // Adults, Masters, Age ranges
		rank = strings.TrimSpace(parts[2])        // Beginner, Intermediate, Advanced
		weightClass = strings.TrimSpace(parts[3]) // -60 kg, -65 kg, etc
	}
	return
}

// saveAthleteFromEvent guarda un atleta y su inscripción al evento en la base de datos usando GORM
func (s *Scraper) saveAthleteFromEvent(data AthleteEventData, eventID string, eventName string) error {
	db := config.GetDB()

	// Usar transacción
	err := db.Transaction(func(tx *gorm.DB) error {
		// 1. Buscar o crear el atleta
		var athlete models.Athlete

		result := tx.Where("external_id = ?", data.SmoothCompID).First(&athlete)

		if result.Error == gorm.ErrRecordNotFound {
			// Atleta no existe, crear nuevo

			// Buscar academy_external_id si existe
			var academy models.Academy
			if data.AcademyName != "" {
				tx.Where("name = ?", data.AcademyName).First(&academy)
			}

			athlete = models.Athlete{
				ExternalID:        data.SmoothCompID,
				FirstName:         data.FirstName,
				LastName:          data.LastName,
				FullName:          data.FullName,
				CountryCode:       data.CountryCode,
				Nationality:       data.Country,
				BirthYear:         data.BirthYear,
				Age:               data.Age,
				ProfileURL:        data.ProfileURL,
				ImageURL:          data.ImageURL,
				AvatarURL:         data.ImageURL,
				AffiliationName:   data.AffiliationName,
				AcademyExternalID: academy.ExternalID,
				Gender:            data.Gender,
				ScrapedAt:         time.Now(),
			}

			if err := tx.Create(&athlete).Error; err != nil {
				return fmt.Errorf("error creando atleta: %w", err)
			}

			logger.Debug("Atleta creado", zap.String("name", athlete.FullName))

		} else if result.Error != nil {
			return fmt.Errorf("error buscando atleta: %w", result.Error)
		} else {
			// Atleta existe, actualizar datos

			// Buscar academy_external_id si existe
			var academy models.Academy
			if data.AcademyName != "" {
				tx.Where("name = ?", data.AcademyName).First(&academy)
				athlete.AcademyExternalID = academy.ExternalID
			}

			athlete.FirstName = data.FirstName
			athlete.LastName = data.LastName
			athlete.FullName = data.FullName
			athlete.CountryCode = data.CountryCode
			athlete.Nationality = data.Country
			athlete.BirthYear = data.BirthYear
			athlete.Age = data.Age
			athlete.ProfileURL = data.ProfileURL
			athlete.ImageURL = data.ImageURL
			athlete.AvatarURL = data.ImageURL
			athlete.AffiliationName = data.AffiliationName
			athlete.Gender = data.Gender
			athlete.ScrapedAt = time.Now()

			if err := tx.Save(&athlete).Error; err != nil {
				return fmt.Errorf("error actualizando atleta: %w", err)
			}

			logger.Debug("Atleta actualizado", zap.String("name", athlete.FullName))
		}

		// 2. Insertar o actualizar la inscripción al evento
		registration := models.EventRegistration{
			AthleteID:        uint(athlete.ID),
			EventID:          eventID,
			EventName:        eventName,
			Division:         data.Division,
			AgeCategory:      data.AgeCategory,
			Rank:             data.Rank,
			WeightClass:      data.WeightClass,
			ActualWeight:     data.ActualWeight,
			Seed:             data.Seed,
			Ranking:          data.Ranking,
			RegistrationDate: time.Now(),
		}

		// Buscar si ya existe la inscripción
		var existingReg models.EventRegistration
		result = tx.Where(
			"athlete_id = ? AND event_id = ? AND division = ? AND age_category = ? AND rank = ? AND weight_class = ?",
			athlete.ID, eventID, data.Division, data.AgeCategory, data.Rank, data.WeightClass,
		).First(&existingReg)

		if result.Error == gorm.ErrRecordNotFound {
			// No existe, crear nueva
			if err := tx.Create(&registration).Error; err != nil {
				return fmt.Errorf("error creando inscripción: %w", err)
			}
			logger.Debug("Inscripción creada", zap.String("athlete", athlete.FullName))
		} else {
			// Ya existe, actualizar
			registration.ID = existingReg.ID
			if err := tx.Save(&registration).Error; err != nil {
				return fmt.Errorf("error actualizando inscripción: %w", err)
			}
			logger.Debug("Inscripción actualizada", zap.String("athlete", athlete.FullName))
		}

		return nil
	})

	return err
}
