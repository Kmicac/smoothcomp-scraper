package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AthleteProfileData struct {
	BeltRank           *string
	TotalWins          *int
	WinsBySubmission   *int
	WinsByPoints       *int
	WinsByDecision     *int
	WinsByDQ           *int
	TotalLosses        *int
	LossesBySubmission *int
	LossesByPoints     *int
	LossesByDecision   *int
	LossesByDQ         *int
}

type labelValue struct {
	Label string
	Value string
}

type profileEventsResponse struct {
	Data        []profileEvent `json:"data"`
	NextPageURL *string        `json:"next_page_url"`
}

type profileEvent struct {
	Registrations []profileEventRegistration `json:"registrations"`
}

type profileEventRegistration struct {
	Matches []profileEventMatch `json:"matches"`
}

type profileEventMatch struct {
	IsWinner bool   `json:"is_winner"`
	Outcome  string `json:"outcome"`
}

type profileStats struct {
	TotalWins          int
	TotalLosses        int
	WinsBySubmission   int
	WinsByPoints       int
	WinsByDecision     int
	WinsByDQ           int
	LossesBySubmission int
	LossesByPoints     int
	LossesByDecision   int
	LossesByDQ         int
}

// ScrapeAthleteProfile obtiene el perfil del atleta y actualiza sus estadisticas en la BD.
func (s *Scraper) ScrapeAthleteProfile(externalID string, profileURL string) error {
	if profileURL == "" {
		if externalID == "" {
			return fmt.Errorf("athlete_id or profile_url is required")
		}
		profileURL = fmt.Sprintf("https://smoothcomp.com/en/profile/%s", externalID)
	}

	if externalID == "" {
		externalID = ExtractIDFromURL(profileURL)
	}
	if externalID == "" {
		return fmt.Errorf("failed to resolve athlete id from profile url")
	}

	logger.Info("Scraping athlete profile",
		zap.String("athlete_id", externalID),
		zap.String("profile_url", profileURL))

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return fmt.Errorf("error creating profile request: %w", err)
	}
	req.Header.Set("User-Agent", s.config.Scraper.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error fetching profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("profile returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("error parsing profile html: %w", err)
	}

	data := parseAthleteProfile(doc)
	if stats, err := s.fetchProfileEventStats(externalID); err != nil {
		logger.Warn("Failed to fetch profile event stats", zap.Error(err))
	} else {
		data = mergeProfileStatsFromEvents(data, stats)
	}

	return s.updateAthleteProfile(externalID, data)
}

// ScrapeAthleteProfiles procesa perfiles en lote para completar campos faltantes.
func (s *Scraper) ScrapeAthleteProfiles(limit int, offset int, onlyMissing bool) (int, error) {
	db := config.GetDB()
	query := db.Model(&models.Athlete{}).Order("id ASC")

	if onlyMissing {
		query = query.Where("belt_rank = '' OR belt_rank IS NULL OR (total_wins = 0 AND total_losses = 0)")
	}

	if offset > 0 {
		query = query.Offset(offset)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var athletes []models.Athlete
	if err := query.Find(&athletes).Error; err != nil {
		return 0, fmt.Errorf("error loading athletes: %w", err)
	}

	if len(athletes) == 0 {
		logger.Info("No athletes found for profile scraping")
		return 0, nil
	}

	delay := time.Duration(s.config.Scraper.RequestDelayMs) * time.Millisecond
	scraped := 0

	for i, athlete := range athletes {
		if athlete.ExternalID == "" && athlete.ProfileURL == "" {
			logger.Warn("Skipping athlete without profile reference",
				zap.Int("athlete_id", athlete.ID))
			continue
		}

		if err := s.ScrapeAthleteProfile(athlete.ExternalID, athlete.ProfileURL); err != nil {
			logger.Error("Failed to scrape athlete profile",
				zap.String("athlete_id", athlete.ExternalID),
				zap.Error(err))
		} else {
			scraped++
		}

		if delay > 0 && i < len(athletes)-1 {
			time.Sleep(delay)
		}
	}

	logger.Info("Athlete profile batch completed",
		zap.Int("selected", len(athletes)),
		zap.Int("scraped", scraped))

	return scraped, nil
}

func parseAthleteProfile(doc *goquery.Document) AthleteProfileData {
	data := AthleteProfileData{}
	if belt := extractBeltRank(doc); belt != "" {
		data.BeltRank = &belt
	}

	applyLegendStats(doc, ".fights_wins_legend li", true, &data)
	applyLegendStats(doc, ".fights_losses_legend li", false, &data)

	items := collectLabelValues(doc)

	for _, item := range items {
		label := normalizeLabel(item.Label)
		if label == "" {
			continue
		}
		value := strings.TrimSpace(item.Value)
		if value == "" {
			continue
		}

		if data.BeltRank == nil && strings.Contains(label, "belt") {
			valueCopy := value
			data.BeltRank = &valueCopy
			continue
		}

		if parsed, ok := parseIntFromString(value); ok {
			applyStat(&data, label, parsed)
		}
	}

	applyFightStats(doc, &data)
	fillTotalsFromBreakdown(&data)
	return data
}

func mergeProfileStatsFromEvents(data AthleteProfileData, stats profileStats) AthleteProfileData {
	if stats.TotalWins > 0 {
		data.TotalWins = &stats.TotalWins
	}
	if stats.TotalLosses > 0 {
		data.TotalLosses = &stats.TotalLosses
	}
	if stats.WinsBySubmission > 0 {
		data.WinsBySubmission = &stats.WinsBySubmission
	}
	if stats.WinsByPoints > 0 {
		data.WinsByPoints = &stats.WinsByPoints
	}
	if stats.WinsByDecision > 0 {
		data.WinsByDecision = &stats.WinsByDecision
	}
	if stats.WinsByDQ > 0 {
		data.WinsByDQ = &stats.WinsByDQ
	}
	if stats.LossesBySubmission > 0 {
		data.LossesBySubmission = &stats.LossesBySubmission
	}
	if stats.LossesByPoints > 0 {
		data.LossesByPoints = &stats.LossesByPoints
	}
	if stats.LossesByDecision > 0 {
		data.LossesByDecision = &stats.LossesByDecision
	}
	if stats.LossesByDQ > 0 {
		data.LossesByDQ = &stats.LossesByDQ
	}
	return data
}

func (s *Scraper) fetchProfileEventStats(externalID string) (profileStats, error) {
	stats := profileStats{}
	if externalID == "" {
		return stats, fmt.Errorf("athlete_id is required")
	}

	client := &http.Client{Timeout: 20 * time.Second}
	url := fmt.Sprintf("https://smoothcomp.com/en/profile/%s/events", externalID)

	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return stats, fmt.Errorf("error creating events request: %w", err)
		}
		req.Header.Set("User-Agent", s.config.Scraper.UserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return stats, fmt.Errorf("error fetching events: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return stats, fmt.Errorf("events endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

		var payload profileEventsResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return stats, fmt.Errorf("error decoding events response: %w", err)
		}
		resp.Body.Close()

		for _, event := range payload.Data {
			for _, reg := range event.Registrations {
				for _, match := range reg.Matches {
					applyEventMatchStats(&stats, match)
				}
			}
		}

		if payload.NextPageURL == nil || *payload.NextPageURL == "" {
			break
		}
		next := *payload.NextPageURL
		if strings.HasPrefix(next, "/") {
			url = "https://smoothcomp.com" + next
		} else {
			url = next
		}
	}

	return stats, nil
}

func applyEventMatchStats(stats *profileStats, match profileEventMatch) {
	outcome := strings.ToLower(strings.TrimSpace(match.Outcome))
	if strings.Contains(outcome, "bye") || strings.Contains(outcome, "walkover") {
		return
	}

	method := classifyOutcome(outcome)

	if match.IsWinner {
		stats.TotalWins++
		switch method {
		case "submission":
			stats.WinsBySubmission++
		case "decision":
			stats.WinsByDecision++
		case "points":
			stats.WinsByPoints++
		case "dq":
			stats.WinsByDQ++
		}
		return
	}

	stats.TotalLosses++
	switch method {
	case "submission":
		stats.LossesBySubmission++
	case "decision":
		stats.LossesByDecision++
	case "points":
		stats.LossesByPoints++
	case "dq":
		stats.LossesByDQ++
	}
}

func classifyOutcome(outcome string) string {
	switch {
	case strings.Contains(outcome, "submission"):
		return "submission"
	case strings.Contains(outcome, "decision"):
		return "decision"
	case strings.Contains(outcome, "points"):
		return "points"
	case strings.Contains(outcome, "dq") || strings.Contains(outcome, "disqualification"):
		return "dq"
	default:
		return ""
	}
}

func collectLabelValues(doc *goquery.Document) []labelValue {
	items := make([]labelValue, 0, 64)

	addItem := func(label, value string) {
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		if label == "" || value == "" {
			return
		}
		items = append(items, labelValue{Label: label, Value: value})
	}

	doc.Find("dl").Each(func(_ int, s *goquery.Selection) {
		s.Find("dt").Each(func(_ int, dt *goquery.Selection) {
			label := dt.Text()
			valueSel := dt.Next()
			if !valueSel.Is("dd") {
				valueSel = dt.NextAll().Filter("dd").First()
			}
			value := valueSel.Text()
			addItem(label, value)
		})
	})

	doc.Find(".stat-item, .stats-item, .stat, .profile-stat").Each(func(_ int, s *goquery.Selection) {
		label := s.Find(".stat-label, .label, .title").First().Text()
		value := s.Find(".stat-value, .value, .count").First().Text()
		addItem(label, value)
	})

	doc.Find("table tr").Each(func(_ int, tr *goquery.Selection) {
		label := tr.Find("th").First().Text()
		value := tr.Find("td").First().Text()
		addItem(label, value)
	})

	doc.Find("li").Each(func(_ int, li *goquery.Selection) {
		text := strings.TrimSpace(li.Text())
		if !strings.Contains(text, ":") {
			return
		}
		parts := strings.SplitN(text, ":", 2)
		if len(parts) != 2 {
			return
		}
		addItem(parts[0], parts[1])
	})

	return items
}

func extractBeltRank(doc *goquery.Document) string {
	text := strings.TrimSpace(doc.Find(".well-skillevel strong.font-size-md").First().Text())
	if text == "" {
		text = strings.TrimSpace(doc.Find(".well-skillevel .font-size-md").First().Text())
	}
	if text == "" {
		return ""
	}

	lower := strings.ToLower(text)
	re := regexp.MustCompile(`\b(white|blue|purple|brown|black)\s+belt\b`)
	match := re.FindStringSubmatch(lower)
	if len(match) < 2 {
		return strings.TrimSpace(text)
	}

	return strings.Title(match[1]) + " belt"
}

func applyLegendStats(doc *goquery.Document, selector string, isWin bool, data *AthleteProfileData) {
	doc.Find(selector).Each(func(_ int, li *goquery.Selection) {
		totalText := li.Find(".total").First().Text()
		if totalText == "" {
			totalText = li.Find("strong").First().Text()
		}
		value, ok := parseIntFromString(totalText)
		if !ok {
			return
		}

		label := strings.ToLower(strings.TrimSpace(li.Find(".type").First().Text()))
		if label == "" {
			label = strings.ToLower(strings.TrimSpace(li.Text()))
		}

		if isWin {
			if strings.Contains(label, "submission") && data.WinsBySubmission == nil {
				data.WinsBySubmission = &value
				return
			}
			if strings.Contains(label, "decision") && data.WinsByDecision == nil {
				data.WinsByDecision = &value
				return
			}
			if strings.Contains(label, "points") && data.WinsByPoints == nil {
				data.WinsByPoints = &value
				return
			}
			if (strings.Contains(label, "dq") || strings.Contains(label, "disqualification")) && data.WinsByDQ == nil {
				data.WinsByDQ = &value
				return
			}
		} else {
			if strings.Contains(label, "submission") && data.LossesBySubmission == nil {
				data.LossesBySubmission = &value
				return
			}
			if strings.Contains(label, "decision") && data.LossesByDecision == nil {
				data.LossesByDecision = &value
				return
			}
			if strings.Contains(label, "points") && data.LossesByPoints == nil {
				data.LossesByPoints = &value
				return
			}
			if (strings.Contains(label, "dq") || strings.Contains(label, "disqualification")) && data.LossesByDQ == nil {
				data.LossesByDQ = &value
				return
			}
		}
	})
}

func applyFightStats(doc *goquery.Document, data *AthleteProfileData) {
	var wins, losses int
	var winsBySubmission, winsByDecision, winsByPoints, winsByDQ int
	var lossesBySubmission, lossesByDecision, lossesByPoints, lossesByDQ int

	doc.Find("li").Each(func(_ int, li *goquery.Selection) {
		label := li.Find(".label-success, .label-danger").First()
		if label.Length() == 0 {
			return
		}

		labelText := strings.ToUpper(strings.TrimSpace(label.Text()))
		isWin := label.HasClass("label-success") || strings.Contains(labelText, "WIN")
		isLoss := label.HasClass("label-danger") || strings.Contains(labelText, "LOSS")
		if !isWin && !isLoss {
			return
		}

		text := strings.ToLower(strings.TrimSpace(li.Text()))
		if isWin {
			wins++
		} else if isLoss {
			losses++
		}

		switch {
		case strings.Contains(text, "submission"):
			if isWin {
				winsBySubmission++
			} else {
				lossesBySubmission++
			}
		case strings.Contains(text, "decision"):
			if isWin {
				winsByDecision++
			} else {
				lossesByDecision++
			}
		case strings.Contains(text, "points"):
			if isWin {
				winsByPoints++
			} else {
				lossesByPoints++
			}
		case strings.Contains(text, "dq") || strings.Contains(text, "disqualification"):
			if isWin {
				winsByDQ++
			} else {
				lossesByDQ++
			}
		}
	})

	if data.TotalWins == nil && wins > 0 {
		data.TotalWins = &wins
	}
	if data.TotalLosses == nil && losses > 0 {
		data.TotalLosses = &losses
	}
	if data.WinsBySubmission == nil && winsBySubmission > 0 {
		data.WinsBySubmission = &winsBySubmission
	}
	if data.WinsByDecision == nil && winsByDecision > 0 {
		data.WinsByDecision = &winsByDecision
	}
	if data.WinsByPoints == nil && winsByPoints > 0 {
		data.WinsByPoints = &winsByPoints
	}
	if data.WinsByDQ == nil && winsByDQ > 0 {
		data.WinsByDQ = &winsByDQ
	}
	if data.LossesBySubmission == nil && lossesBySubmission > 0 {
		data.LossesBySubmission = &lossesBySubmission
	}
	if data.LossesByDecision == nil && lossesByDecision > 0 {
		data.LossesByDecision = &lossesByDecision
	}
	if data.LossesByPoints == nil && lossesByPoints > 0 {
		data.LossesByPoints = &lossesByPoints
	}
	if data.LossesByDQ == nil && lossesByDQ > 0 {
		data.LossesByDQ = &lossesByDQ
	}
}

func fillTotalsFromBreakdown(data *AthleteProfileData) {
	if data.TotalWins == nil || (data.TotalWins != nil && *data.TotalWins == 0) {
		total := 0
		if data.WinsBySubmission != nil {
			total += *data.WinsBySubmission
		}
		if data.WinsByDecision != nil {
			total += *data.WinsByDecision
		}
		if data.WinsByPoints != nil {
			total += *data.WinsByPoints
		}
		if data.WinsByDQ != nil {
			total += *data.WinsByDQ
		}
		if total > 0 {
			data.TotalWins = &total
		}
	}

	if data.TotalLosses == nil || (data.TotalLosses != nil && *data.TotalLosses == 0) {
		total := 0
		if data.LossesBySubmission != nil {
			total += *data.LossesBySubmission
		}
		if data.LossesByDecision != nil {
			total += *data.LossesByDecision
		}
		if data.LossesByPoints != nil {
			total += *data.LossesByPoints
		}
		if data.LossesByDQ != nil {
			total += *data.LossesByDQ
		}
		if total > 0 {
			data.TotalLosses = &total
		}
	}
}

func normalizeLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.Join(strings.Fields(label), " ")
	return label
}

func parseIntFromString(value string) (int, bool) {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(value)
	if match == "" {
		return 0, false
	}
	match = strings.ReplaceAll(match, ",", "")
	parsed, err := strconv.Atoi(match)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func applyStat(data *AthleteProfileData, label string, value int) {
	label = strings.ToLower(label)
	isWin := strings.Contains(label, "win")
	isLoss := strings.Contains(label, "loss")

	if isWin {
		switch {
		case strings.Contains(label, "submission"):
			if data.WinsBySubmission == nil {
				data.WinsBySubmission = &value
			}
		case strings.Contains(label, "points"):
			if data.WinsByPoints == nil {
				data.WinsByPoints = &value
			}
		case strings.Contains(label, "decision"):
			if data.WinsByDecision == nil {
				data.WinsByDecision = &value
			}
		case strings.Contains(label, "dq") || strings.Contains(label, "disqualification"):
			if data.WinsByDQ == nil {
				data.WinsByDQ = &value
			}
		default:
			if data.TotalWins == nil {
				data.TotalWins = &value
			}
		}
		return
	}

	if isLoss {
		switch {
		case strings.Contains(label, "submission"):
			if data.LossesBySubmission == nil {
				data.LossesBySubmission = &value
			}
		case strings.Contains(label, "points"):
			if data.LossesByPoints == nil {
				data.LossesByPoints = &value
			}
		case strings.Contains(label, "decision"):
			if data.LossesByDecision == nil {
				data.LossesByDecision = &value
			}
		case strings.Contains(label, "dq") || strings.Contains(label, "disqualification"):
			if data.LossesByDQ == nil {
				data.LossesByDQ = &value
			}
		default:
			if data.TotalLosses == nil {
				data.TotalLosses = &value
			}
		}
	}
}

func (s *Scraper) updateAthleteProfile(externalID string, data AthleteProfileData) error {
	db := config.GetDB()
	var athlete models.Athlete

	if err := db.Where("external_id = ?", externalID).First(&athlete).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("athlete not found: %s", externalID)
		}
		return fmt.Errorf("error loading athlete: %w", err)
	}

	updates := map[string]interface{}{}
	if data.BeltRank != nil && *data.BeltRank != "" {
		updates["belt_rank"] = *data.BeltRank
	}
	if data.TotalWins != nil {
		updates["total_wins"] = *data.TotalWins
	}
	if data.WinsBySubmission != nil {
		updates["wins_by_submission"] = *data.WinsBySubmission
	}
	if data.WinsByPoints != nil {
		updates["wins_by_points"] = *data.WinsByPoints
	}
	if data.WinsByDecision != nil {
		updates["wins_by_decision"] = *data.WinsByDecision
	}
	if data.WinsByDQ != nil {
		updates["wins_by_dq"] = *data.WinsByDQ
	}
	if data.TotalLosses != nil {
		updates["total_losses"] = *data.TotalLosses
	}
	if data.LossesBySubmission != nil {
		updates["losses_by_submission"] = *data.LossesBySubmission
	}
	if data.LossesByPoints != nil {
		updates["losses_by_points"] = *data.LossesByPoints
	}
	if data.LossesByDecision != nil {
		updates["losses_by_decision"] = *data.LossesByDecision
	}
	if data.LossesByDQ != nil {
		updates["losses_by_dq"] = *data.LossesByDQ
	}

	if len(updates) == 0 {
		logger.Info("No profile fields found", zap.String("athlete_id", externalID))
		return nil
	}

	updates["scraped_at"] = time.Now()

	if err := db.Model(&athlete).Updates(updates).Error; err != nil {
		return fmt.Errorf("error updating athlete profile: %w", err)
	}

	logger.Info("Athlete profile updated",
		zap.String("athlete_id", externalID),
		zap.Int("fields", len(updates)-1))

	return nil
}
