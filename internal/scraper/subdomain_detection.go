package scraper

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
)

// DetectEventSubdomain detecta el subdominio correcto para un evento
// Algunos eventos están en subdominios específicos (adcc.smoothcomp.com, ibjjf.smoothcomp.com)
// mientras que otros están en el dominio principal (smoothcomp.com)
func (s *Scraper) DetectEventSubdomain(eventID string) string {
	// Lista de subdominios comunes para probar
	subdomains := []string{
		"",          // smoothcomp.com (sin subdominio)
		"adcc",      // adcc.smoothcomp.com
		"ibjjf",     // ibjjf.smoothcomp.com
		"uaejjf",    // uaejjf.smoothcomp.com
		"ajp",       // ajp.smoothcomp.com
		"sjjif",     // sjjif.smoothcomp.com
		"newbreed",  // newbreed.smoothcomp.com
		"grappling", // grappling.smoothcomp.com
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// No seguir redirects automáticamente
			return http.ErrUseLastResponse
		},
	}

	logger.Info("Detectando subdominio del evento", zap.String("event_id", eventID))

	for _, subdomain := range subdomains {
		var baseURL string
		if subdomain == "" {
			baseURL = "smoothcomp.com"
		} else {
			baseURL = fmt.Sprintf("%s.smoothcomp.com", subdomain)
		}

		// Intentar hacer HEAD request a la página del evento
		eventURL := fmt.Sprintf("https://%s/en/event/%s", baseURL, eventID)

		req, err := http.NewRequest("HEAD", eventURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", s.config.Scraper.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			logger.Debug("Subdominio falló",
				zap.String("subdomain", baseURL),
				zap.Error(err))
			continue
		}
		resp.Body.Close()

		// Si recibimos 200 OK, este es el subdominio correcto
		if resp.StatusCode == http.StatusOK {
			logger.Info("Subdominio detectado",
				zap.String("subdomain", baseURL),
				zap.String("event_url", eventURL))
			return baseURL
		}

		// Si recibimos 301/302 y nos redirigen al mismo dominio con https, también es válido
		if resp.StatusCode == http.StatusMovedPermanently ||
			resp.StatusCode == http.StatusFound {
			location := resp.Header.Get("Location")
			// Verificar si el redirect es al mismo dominio
			if location != "" && containsSubstring(location, baseURL) {
				logger.Info("Subdominio detectado via redirect",
					zap.String("subdomain", baseURL),
					zap.String("redirect", location))
				return baseURL
			}
		}

		logger.Debug("Subdominio no válido",
			zap.String("subdomain", baseURL),
			zap.Int("status", resp.StatusCode))
	}

	// Si no encontramos ningún subdominio válido, usar el dominio principal
	logger.Warn("No se detectó subdominio específico, usando smoothcomp.com",
		zap.String("event_id", eventID))
	return "smoothcomp.com"
}

// ExtractSubdomainFromURL extrae el subdominio de una URL de evento
// Si el usuario ya proporcionó la URL completa, podemos extraer el subdominio directamente
func ExtractSubdomainFromURL(eventURL string) string {
	// Regex para extraer el subdominio
	// Ejemplos:
	// https://adcc.smoothcomp.com/en/event/25258 -> adcc.smoothcomp.com
	// https://smoothcomp.com/en/event/25258 -> smoothcomp.com
	re := regexp.MustCompile(`https?://([a-z0-9\-]+\.)?smoothcomp\.com`)
	matches := re.FindStringSubmatch(eventURL)

	if len(matches) > 0 {
		// matches[0] = "https://adcc.smoothcomp.com"
		// Extraer solo el dominio (sin https://)
		domainRe := regexp.MustCompile(`https?://(.+)`)
		domainMatches := domainRe.FindStringSubmatch(matches[0])
		if len(domainMatches) > 1 {
			return domainMatches[1] // "adcc.smoothcomp.com" o "smoothcomp.com"
		}
	}

	return "smoothcomp.com"
}

// BuildAPIURL construye la URL completa de la API usando el subdominio
func BuildAPIURL(subdomain string, eventID string) string {
	return fmt.Sprintf("https://%s/en/event/%s/participants", subdomain, eventID)
}

// containsSubstring verifica si str contiene substr (helper function)
func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr ||
		len(str) > len(substr) && (str[:len(substr)+1] == substr+"/" ||
			str[len(str)-len(substr):] == substr))
}

// ScrapeEventAthletesWithSubdomainDetection es una versión mejorada que detecta el subdominio
func (s *Scraper) ScrapeEventAthletesWithSubdomainDetection(eventID string, eventName string, eventURL string) error {
	var subdomain string

	// Opción 1: Si tenemos la URL del evento, extraer el subdominio
	if eventURL != "" {
		subdomain = ExtractSubdomainFromURL(eventURL)
		logger.Info("Subdominio extraído de URL",
			zap.String("subdomain", subdomain),
			zap.String("event_url", eventURL))
	} else {
		// Opción 2: Detectar automáticamente probando diferentes subdominios
		subdomain = s.DetectEventSubdomain(eventID)
	}

	// Construir la URL de la API con el subdominio correcto
	apiURL := BuildAPIURL(subdomain, eventID)

	logger.Info("Iniciando scraping de atletas del evento via API",
		zap.String("event_id", eventID),
		zap.String("event_name", eventName),
		zap.String("api_url", apiURL),
		zap.String("subdomain", subdomain))

	// Aquí va el resto del código de ScrapeEventAthletes...
	// (El mismo código del archivo anterior, pero usando apiURL en lugar de construir la URL)

	return fmt.Errorf("implementación completa en athlete_event_scraper_v2.go")
}

// TestSubdomainDetection es una función de utilidad para testing
func (s *Scraper) TestSubdomainDetection(eventID string) {
	logger.Info("=== TEST: Detección de Subdominio ===")

	subdomain := s.DetectEventSubdomain(eventID)
	apiURL := BuildAPIURL(subdomain, eventID)

	logger.Info("Resultado del test",
		zap.String("event_id", eventID),
		zap.String("detected_subdomain", subdomain),
		zap.String("api_url", apiURL))

	// Intentar hacer un request de prueba
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", apiURL, nil)
	req.Header.Set("User-Agent", s.config.Scraper.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Request de prueba falló", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	logger.Info("Request de prueba exitoso",
		zap.Int("status", resp.StatusCode),
		zap.String("content_type", resp.Header.Get("Content-Type")))
}
