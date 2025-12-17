package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AthleteEventData representa los datos de un atleta extraídos del evento
type AthleteEventData struct {
	SmoothCompID    string
	FirstName       string
	LastName        string
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
	EventCardURL    string
}

// ScrapeEventAthletes extrae todos los atletas de un evento específico
func (s *Scraper) ScrapeEventAthletes(eventID string, eventName string) error {
	logger.Info("Iniciando scraping de atletas del evento",
		zap.String("event_id", eventID),
		zap.String("event_name", eventName))

	url := fmt.Sprintf("https://smoothcomp.com/en/event/%s/participants", eventID)

	// Crear un nuevo collector
	c := colly.NewCollector(
		colly.AllowedDomains("smoothcomp.com", "www.smoothcomp.com"),
		colly.UserAgent(s.config.Scraper.UserAgent),
	)

	// Configurar rate limiting
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*smoothcomp.com*",
		Delay:       time.Duration(s.config.Scraper.RequestDelayMs) * time.Millisecond,
		RandomDelay: 1 * time.Second,
	})

	var athletes []AthleteEventData
	var currentCategory string

	// Extraer el nombre de la categoría (heading)
	c.OnHTML("div.participant-group", func(group *colly.HTMLElement) {
		// Extraer la categoría del h2
		categoryText := group.ChildText("h2.group-name")
		currentCategory = categoryText
		logger.Debug("Procesando categoría", zap.String("category", currentCategory))

		// Parsear la categoría
		division, ageCategory, rank, weightClass := parseCategory(categoryText)

		// Procesar cada atleta en esta categoría
		group.ForEach("div.sc-card", func(_ int, card *colly.HTMLElement) {
			athlete := AthleteEventData{
				Division:    division,
				AgeCategory: ageCategory,
				Rank:        rank,
				WeightClass: weightClass,
			}

			// Extraer nombre y profile URL
			profileLink := card.ChildAttr("a.truncate.block", "href")
			athlete.FullName = strings.TrimSpace(card.ChildText("a.truncate.block span"))
			athlete.ProfileURL = profileLink

			// Separar nombre en FirstName y LastName
			nameParts := strings.Fields(athlete.FullName)
			if len(nameParts) > 0 {
				athlete.FirstName = nameParts[0]
				if len(nameParts) > 1 {
					athlete.LastName = strings.Join(nameParts[1:], " ")
				}
			}

			// Extraer SmoothCompID del URL del perfil
			profileRegex := regexp.MustCompile(`/profile/(\d+)`)
			if matches := profileRegex.FindStringSubmatch(profileLink); len(matches) > 1 {
				athlete.SmoothCompID = matches[1]
			}

			// Extraer imagen
			athlete.ImageURL = card.ChildAttr("div.profile-image-wrapper img", "src")

			// Extraer país
			countryClass := card.ChildAttr("span.flag-icon", "class")
			countryRegex := regexp.MustCompile(`flag-icon-([a-z]{2})`)
			if matches := countryRegex.FindStringSubmatch(countryClass); len(matches) > 1 {
				athlete.CountryCode = strings.ToUpper(matches[1])
				athlete.Country = config.GetCountryName(athlete.CountryCode)
			}

			// Extraer academia
			athlete.AcademyName = strings.TrimSpace(card.ChildText("div.sc-card-body-club div.margin-top-xs-8 a"))

			// Extraer afiliación (puede no existir)
			athlete.AffiliationName = strings.TrimSpace(card.ChildText("div.sc-card-body-club div.margin-top-xs-2 a"))

			// Extraer seed/ranking (solo si existe)
			seedText := card.ChildText("div.sc-card-body-seed")
			if seedText != "" {
				athlete.Seed, athlete.Ranking = parseSeedRanking(seedText)
			}

			// Extraer datos de la tabla (en el footer)
			card.ForEach("table.table tbody tr", func(_ int, row *colly.HTMLElement) {
				header := strings.TrimSpace(row.ChildText("th"))
				value := strings.TrimSpace(row.ChildText("td"))

				switch {
				case strings.Contains(header, "Birth"):
					// Extraer año de nacimiento y edad
					// Formato: "1999 (26 years)"
					birthRegex := regexp.MustCompile(`(\d{4})\s*\((\d+)\s*years?\)`)
					if matches := birthRegex.FindStringSubmatch(value); len(matches) > 2 {
						if year, err := strconv.Atoi(matches[1]); err == nil {
							athlete.BirthYear = year
						}
						if age, err := strconv.Atoi(matches[2]); err == nil {
							athlete.Age = age
						}
					}

				case strings.Contains(header, "Weight"):
					// El peso real está en el div con class "muted"
					// Formato: "59.20 kg"
					weightText := row.ChildText("div.muted")
					weightRegex := regexp.MustCompile(`([\d.]+)\s*kg`)
					if matches := weightRegex.FindStringSubmatch(weightText); len(matches) > 1 {
						if weight, err := strconv.ParseFloat(matches[1], 64); err == nil {
							athlete.ActualWeight = weight
						}
					}

				case strings.Contains(header, "Download"):
					// Extraer URL de la credencial del evento
					athlete.EventCardURL = row.ChildAttr("td a", "href")
				}
			})

			// Solo agregar si tenemos los datos mínimos requeridos
			if athlete.SmoothCompID != "" && athlete.FullName != "" {
				athletes = append(athletes, athlete)
			}
		})
	})

	// Error handler
	c.OnError(func(r *colly.Response, err error) {
		logger.Error("Error scrapeando evento", zap.String("url", r.Request.URL.String()), zap.Error(err))
	})

	// Log de requests
	c.OnRequest(func(r *colly.Request) {
		logger.Debug("Visitando URL", zap.String("url", r.URL.String()))
	})

	// Visitar la página
	if err := c.Visit(url); err != nil {
		return fmt.Errorf("error visitando URL: %w", err)
	}

	c.Wait()

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
		division = strings.TrimSpace(parts[0])    // Men, Women
		ageCategory = strings.TrimSpace(parts[1]) // Adults, Masters, Juveniles
		rank = strings.TrimSpace(parts[2])        // Beginner, Intermediate, Advanced
		weightClass = strings.TrimSpace(parts[3]) // -60 kg, -65 kg, etc
	}
	return
}

// parseSeedRanking extrae el seed y ranking del texto
// Ejemplo: "Seed # 1 (Ranked 96)"
func parseSeedRanking(text string) (seed int, ranking int) {
	// Extraer seed
	seedRegex := regexp.MustCompile(`Seed\s*#\s*(\d+)`)
	if matches := seedRegex.FindStringSubmatch(text); len(matches) > 1 {
		seed, _ = strconv.Atoi(matches[1])
	}

	// Extraer ranking
	rankingRegex := regexp.MustCompile(`Ranked\s+(\d+)`)
	if matches := rankingRegex.FindStringSubmatch(text); len(matches) > 1 {
		ranking, _ = strconv.Atoi(matches[1])
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
			EventCardURL:     data.EventCardURL,
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
