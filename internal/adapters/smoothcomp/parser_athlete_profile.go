package smoothcomp

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
)

type athleteProfileParsed struct {
	Person       contract.Person
	Organization *contract.Organization
	Warnings     []contract.Warning
}

type athleteProfileEventsParsed struct {
	Events       []contract.Event
	Matches      []contract.Match
	MatchSummary *contract.MatchSummary
	Attributes   map[string]string
	EventCount   int
	Warnings     []contract.Warning
}

type athleteProfileData struct {
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

type profileEventsResponse struct {
	Data        []profileEvent `json:"data"`
	NextPageURL *string        `json:"next_page_url"`
}

type profileEvent struct {
	EventID       int64                      `json:"event_id"`
	EventName     string                     `json:"event_name"`
	EventURL      string                     `json:"event_url"`
	StartDate     string                     `json:"start_date"`
	EventStatus   string                     `json:"event_status"`
	IsUpcoming    bool                       `json:"is_upcoming"`
	Registrations []profileEventRegistration `json:"registrations"`
}

type profileEventRegistration struct {
	ID          int64               `json:"id"`
	Name        string              `json:"name"`
	Division    string              `json:"division"`
	AgeCategory string              `json:"age_category"`
	Rank        string              `json:"rank"`
	WeightClass string              `json:"weight_class"`
	BracketName string              `json:"bracket_name"`
	Matches     []profileEventMatch `json:"matches"`
}

type profileEventMatch struct {
	ID                int64  `json:"id"`
	MatchID           int64  `json:"match_id"`
	BracketMatchID    int64  `json:"bracket_match_id"`
	MatchURL          string `json:"match_url"`
	IsWinner          bool   `json:"is_winner"`
	Outcome           string `json:"outcome"`
	Result            string `json:"result"`
	Score             string `json:"score"`
	Method            string `json:"method"`
	ResultMethod      string `json:"result_method"`
	WinType           string `json:"win_type"`
	RoundName         string `json:"round_name"`
	Placement         string `json:"placement"`
	OpponentUserID    int64  `json:"opponent_user_id"`
	OpponentID        int64  `json:"opponent_id"`
	OpponentName      string `json:"opponent_name"`
	OpponentFirstName string `json:"opponent_first_name"`
	OpponentLastName  string `json:"opponent_last_name"`
	OpponentCountry   string `json:"opponent_country"`
}

type labelValue struct {
	Label string
	Value string
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

func parseAthleteProfileHTML(body []byte, profileID, profileURL, snapshotID string) (athleteProfileParsed, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return athleteProfileParsed{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_athlete_profile_html", true, "failed to parse athlete profile html", err)
	}

	fullName := firstNonEmpty(
		strings.TrimSpace(doc.Find("h1").First().Text()),
		strings.TrimSpace(doc.Find(".profile-name").First().Text()),
		strings.TrimSpace(doc.Find(".competitor-name").First().Text()),
		strings.TrimSpace(doc.Find(".page-title").First().Text()),
	)
	givenName, familyName := splitName(fullName)
	if profileID == "" && profileURL != "" {
		profileID = extractIDFromURL(profileURL)
	}

	person := contract.Person{
		SourceID:        firstNonEmpty(profileID, "profile:"+slugify(fullName)),
		GivenName:       givenName,
		FamilyName:      familyName,
		FullName:        fullName,
		ProfileURL:      profileURL,
		ImageURL:        firstNonEmpty(attrOrEmpty(doc.Find("meta[property='og:image']").First(), "content"), attrOrEmpty(doc.Find("img.profile-avatar, .profile-avatar img, .avatar img").First(), "src")),
		RawReferenceIDs: []string{snapshotID},
		Attributes:      map[string]string{},
	}

	profileData := parseAthleteProfileStats(doc)
	if profileData.BeltRank != nil {
		person.BeltRank = *profileData.BeltRank
	}

	if ageText := firstNonEmpty(extractLabelValue(doc, "age"), strings.TrimSpace(doc.Find(".profile-age").First().Text())); ageText != "" {
		if age, ok := parseIntFromString(ageText); ok {
			person.Age = &age
		}
	}
	if birthText := extractLabelValue(doc, "birth"); birthText != "" {
		if birthYear, ok := parseIntFromString(birthText); ok {
			person.BirthYear = &birthYear
		}
	}

	country := firstNonEmpty(
		extractLabelValue(doc, "country"),
		extractLabelValue(doc, "nationality"),
		strings.TrimSpace(doc.Find(".flag-icon").First().AttrOr("title", "")),
	)
	person.Country = country
	person.CountryCode = strings.ToUpper(firstNonEmpty(
		extractLabelValue(doc, "country code"),
		attrOrEmpty(doc.Find(".flag-icon").First(), "data-country"),
	))

	orgName := firstNonEmpty(
		extractLabelValue(doc, "academy"),
		extractLabelValue(doc, "club"),
		extractLabelValue(doc, "affiliation"),
		strings.TrimSpace(doc.Find(".club-name").First().Text()),
	)
	var organization *contract.Organization
	if orgName != "" {
		orgID := "academy_name:" + slugify(orgName)
		person.OrganizationSourceID = orgID
		organization = &contract.Organization{
			SourceID:        orgID,
			Name:            orgName,
			Kind:            "academy",
			Country:         country,
			CountryCode:     person.CountryCode,
			RawReferenceIDs: []string{snapshotID},
		}
	}

	if profileData.TotalWins != nil {
		person.Attributes["total_wins"] = strconv.Itoa(*profileData.TotalWins)
	}
	if profileData.TotalLosses != nil {
		person.Attributes["total_losses"] = strconv.Itoa(*profileData.TotalLosses)
	}
	if profileData.WinsBySubmission != nil {
		person.Attributes["wins_by_submission"] = strconv.Itoa(*profileData.WinsBySubmission)
	}
	if profileData.WinsByPoints != nil {
		person.Attributes["wins_by_points"] = strconv.Itoa(*profileData.WinsByPoints)
	}
	if profileData.WinsByDecision != nil {
		person.Attributes["wins_by_decision"] = strconv.Itoa(*profileData.WinsByDecision)
	}
	if profileData.WinsByDQ != nil {
		person.Attributes["wins_by_dq"] = strconv.Itoa(*profileData.WinsByDQ)
	}
	if profileData.LossesBySubmission != nil {
		person.Attributes["losses_by_submission"] = strconv.Itoa(*profileData.LossesBySubmission)
	}
	if profileData.LossesByPoints != nil {
		person.Attributes["losses_by_points"] = strconv.Itoa(*profileData.LossesByPoints)
	}
	if profileData.LossesByDecision != nil {
		person.Attributes["losses_by_decision"] = strconv.Itoa(*profileData.LossesByDecision)
	}
	if profileData.LossesByDQ != nil {
		person.Attributes["losses_by_dq"] = strconv.Itoa(*profileData.LossesByDQ)
	}

	return athleteProfileParsed{
		Person:       person,
		Organization: organization,
	}, nil
}

func parseAthleteProfileEventsJSON(body []byte, athleteSourceID string, snapshotID string) (athleteProfileEventsParsed, *string, error) {
	var payload profileEventsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return athleteProfileEventsParsed{}, nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_athlete_profile_events_json", true, "failed to decode athlete profile events json", err)
	}

	stats := profileStats{}
	events := make([]contract.Event, 0)
	matches := make([]contract.Match, 0)
	warnings := make([]contract.Warning, 0)
	seenEvents := map[string]struct{}{}
	matchCounter := 0
	matchesWithKnownMethod := 0
	matchesWithoutMethod := 0

	for _, event := range payload.Data {
		eventID := ""
		if event.EventID > 0 {
			eventID = strconv.FormatInt(event.EventID, 10)
		}
		eventURL := strings.TrimSpace(event.EventURL)
		if eventID == "" && eventURL != "" {
			eventID = extractIDFromURL(eventURL)
		}
		if eventID == "" {
			continue
		}
		normalizedEvent := contract.Event{
			SourceID:        eventID,
			SourceURL:       eventURL,
			Name:            strings.TrimSpace(event.EventName),
			Status:          normalizeProfileEventStatus(event),
			StartsAt:        strings.TrimSpace(event.StartDate),
			RawReferenceIDs: []string{snapshotID},
		}
		if _, ok := seenEvents[eventID]; !ok {
			seenEvents[eventID] = struct{}{}
			events = append(events, normalizedEvent)
		}

		for regIndex, registration := range event.Registrations {
			context := matchContext{
				AthleteSourceID: athleteSourceID,
				Event:           normalizedEvent,
				Registration:    registration,
				SnapshotID:      snapshotID,
				RegistrationPos: regIndex,
			}
			for matchIndex, match := range registration.Matches {
				normalizedMatch, matchWarnings := normalizeProfileMatch(match, context, matchIndex)
				if normalizedMatch.SourceID == "" {
					continue
				}
				matchCounter++
				if normalizedMatch.FinishMethod != "" && normalizedMatch.FinishMethod != "unknown" {
					matchesWithKnownMethod++
				} else if normalizedMatch.Outcome == "win" || normalizedMatch.Outcome == "loss" {
					matchesWithoutMethod++
				}
				applyNormalizedMatchStats(&stats, normalizedMatch)
				matches = append(matches, normalizedMatch)
				warnings = append(warnings, matchWarnings...)
			}
		}
	}

	result := athleteProfileEventsParsed{
		Events:     events,
		Matches:    matches,
		EventCount: len(events),
		Attributes: map[string]string{},
		Warnings:   warnings,
	}
	if stats.TotalWins > 0 {
		result.Attributes["competition_total_wins"] = strconv.Itoa(stats.TotalWins)
	}
	if stats.TotalLosses > 0 {
		result.Attributes["competition_total_losses"] = strconv.Itoa(stats.TotalLosses)
	}
	if stats.WinsBySubmission > 0 {
		result.Attributes["competition_wins_by_submission"] = strconv.Itoa(stats.WinsBySubmission)
	}
	if stats.WinsByPoints > 0 {
		result.Attributes["competition_wins_by_points"] = strconv.Itoa(stats.WinsByPoints)
	}
	if stats.WinsByDecision > 0 {
		result.Attributes["competition_wins_by_decision"] = strconv.Itoa(stats.WinsByDecision)
	}
	if stats.WinsByDQ > 0 {
		result.Attributes["competition_wins_by_dq"] = strconv.Itoa(stats.WinsByDQ)
	}
	if stats.LossesBySubmission > 0 {
		result.Attributes["competition_losses_by_submission"] = strconv.Itoa(stats.LossesBySubmission)
	}
	if stats.LossesByPoints > 0 {
		result.Attributes["competition_losses_by_points"] = strconv.Itoa(stats.LossesByPoints)
	}
	if stats.LossesByDecision > 0 {
		result.Attributes["competition_losses_by_decision"] = strconv.Itoa(stats.LossesByDecision)
	}
	if stats.LossesByDQ > 0 {
		result.Attributes["competition_losses_by_dq"] = strconv.Itoa(stats.LossesByDQ)
	}
	if matchCounter > 0 {
		confidence := "high"
		if matchesWithoutMethod > 0 {
			confidence = "medium"
		}
		result.MatchSummary = &contract.MatchSummary{
			AthleteSourceID:    athleteSourceID,
			Scope:              "profile_events",
			Confidence:         confidence,
			TotalMatches:       matchCounter,
			TotalWins:          stats.TotalWins,
			TotalLosses:        stats.TotalLosses,
			WinsBySubmission:   stats.WinsBySubmission,
			WinsByPoints:       stats.WinsByPoints,
			WinsByDecision:     stats.WinsByDecision,
			WinsByDQ:           stats.WinsByDQ,
			LossesBySubmission: stats.LossesBySubmission,
			LossesByPoints:     stats.LossesByPoints,
			LossesByDecision:   stats.LossesByDecision,
			LossesByDQ:         stats.LossesByDQ,
			RawReferenceIDs:    []string{snapshotID},
			Attributes: map[string]string{
				"matches_with_known_method":   strconv.Itoa(matchesWithKnownMethod),
				"matches_with_missing_method": strconv.Itoa(matchesWithoutMethod),
				"events_scanned":              strconv.Itoa(len(events)),
			},
		}
	}

	return result, payload.NextPageURL, nil
}

func parseAthleteProfileStats(doc *goquery.Document) athleteProfileData {
	data := athleteProfileData{}
	if belt := extractProfileBeltRank(doc); belt != "" {
		data.BeltRank = &belt
	}

	applyLegendStats(doc, ".fights_wins_legend li", true, &data)
	applyLegendStats(doc, ".fights_losses_legend li", false, &data)

	items := collectProfileLabelValues(doc)
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
			applyProfileStat(&data, label, parsed)
		}
	}

	applyFightStats(doc, &data)
	fillTotalsFromBreakdown(&data)
	return data
}

func collectProfileLabelValues(doc *goquery.Document) []labelValue {
	items := make([]labelValue, 0, 64)
	addItem := func(label, value string) {
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		if label == "" || value == "" {
			return
		}
		items = append(items, labelValue{Label: label, Value: value})
	}

	doc.Find("dl").Each(func(_ int, selection *goquery.Selection) {
		selection.Find("dt").Each(func(_ int, dt *goquery.Selection) {
			label := dt.Text()
			valueSelection := dt.Next()
			if !valueSelection.Is("dd") {
				valueSelection = dt.NextAll().Filter("dd").First()
			}
			addItem(label, valueSelection.Text())
		})
	})

	doc.Find(".stat-item, .stats-item, .stat, .profile-stat").Each(func(_ int, selection *goquery.Selection) {
		addItem(
			selection.Find(".stat-label, .label, .title").First().Text(),
			selection.Find(".stat-value, .value, .count").First().Text(),
		)
	})

	doc.Find("table tr").Each(func(_ int, tr *goquery.Selection) {
		addItem(tr.Find("th").First().Text(), tr.Find("td").First().Text())
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

func extractProfileBeltRank(doc *goquery.Document) string {
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

func applyLegendStats(doc *goquery.Document, selector string, isWin bool, data *athleteProfileData) {
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
			switch {
			case strings.Contains(label, "submission") && data.WinsBySubmission == nil:
				data.WinsBySubmission = &value
			case strings.Contains(label, "decision") && data.WinsByDecision == nil:
				data.WinsByDecision = &value
			case strings.Contains(label, "points") && data.WinsByPoints == nil:
				data.WinsByPoints = &value
			case (strings.Contains(label, "dq") || strings.Contains(label, "disqualification")) && data.WinsByDQ == nil:
				data.WinsByDQ = &value
			}
			return
		}
		switch {
		case strings.Contains(label, "submission") && data.LossesBySubmission == nil:
			data.LossesBySubmission = &value
		case strings.Contains(label, "decision") && data.LossesByDecision == nil:
			data.LossesByDecision = &value
		case strings.Contains(label, "points") && data.LossesByPoints == nil:
			data.LossesByPoints = &value
		case (strings.Contains(label, "dq") || strings.Contains(label, "disqualification")) && data.LossesByDQ == nil:
			data.LossesByDQ = &value
		}
	})
}

func applyFightStats(doc *goquery.Document, data *athleteProfileData) {
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
		} else {
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

func fillTotalsFromBreakdown(data *athleteProfileData) {
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
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(label))), " ")
}

func parseIntFromString(value string) (int, bool) {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(value)
	if match == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.ReplaceAll(match, ",", ""))
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func applyProfileStat(data *athleteProfileData, label string, value int) {
	label = strings.ToLower(label)
	if strings.Contains(label, "win") {
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
	if strings.Contains(label, "loss") {
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

func extractLabelValue(doc *goquery.Document, contains string) string {
	contains = strings.ToLower(strings.TrimSpace(contains))
	for _, item := range collectProfileLabelValues(doc) {
		if strings.Contains(strings.ToLower(strings.TrimSpace(item.Label)), contains) {
			return strings.TrimSpace(item.Value)
		}
	}
	return ""
}

func splitName(fullName string) (string, string) {
	parts := filterEmpty(strings.Fields(fullName)...)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func applyNormalizedMatchStats(stats *profileStats, match contract.Match) {
	if stats == nil {
		return
	}
	if match.Outcome == "walkover" || match.Outcome == "bye" || match.Outcome == "draw" || match.Outcome == "" {
		return
	}
	if match.Outcome == "win" {
		stats.TotalWins++
		switch match.FinishMethod {
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
	if match.Outcome != "loss" {
		return
	}
	stats.TotalLosses++
	switch match.FinishMethod {
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

func classifyFinishMethod(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case raw == "", raw == "win", raw == "loss":
		return ""
	case strings.Contains(raw, "submission"), strings.Contains(raw, "tap"):
		return "submission"
	case strings.Contains(raw, "decision"):
		return "decision"
	case strings.Contains(raw, "points"), strings.Contains(raw, "advantage"):
		return "points"
	case strings.Contains(raw, "dq") || strings.Contains(raw, "disqualification"):
		return "dq"
	case strings.Contains(raw, "walkover") || strings.Contains(raw, "wo"):
		return "walkover"
	case strings.Contains(raw, "bye"):
		return "bye"
	default:
		return "unknown"
	}
}

type matchContext struct {
	AthleteSourceID string
	Event           contract.Event
	Registration    profileEventRegistration
	SnapshotID      string
	RegistrationPos int
}

func normalizeProfileMatch(match profileEventMatch, context matchContext, matchIndex int) (contract.Match, []contract.Warning) {
	sourceID := firstNonEmpty(
		int64String(match.MatchID),
		int64String(match.BracketMatchID),
		int64String(match.ID),
		syntheticMatchID(context.AthleteSourceID, context.Event.SourceID, context.Registration.ID, matchIndex),
	)

	opponentName := firstNonEmpty(
		strings.TrimSpace(match.OpponentName),
		strings.TrimSpace(strings.Join(filterEmpty(match.OpponentFirstName, match.OpponentLastName), " ")),
	)
	opponentSourceID := ""
	if match.OpponentUserID > 0 {
		opponentSourceID = "athlete:" + strconv.FormatInt(match.OpponentUserID, 10)
	} else if match.OpponentID > 0 {
		opponentSourceID = "athlete:" + strconv.FormatInt(match.OpponentID, 10)
	}

	rawResult := firstNonEmpty(strings.TrimSpace(match.Result), strings.TrimSpace(match.Outcome))
	outcome := normalizeMatchOutcome(match, rawResult)
	method := classifyFinishMethod(firstNonEmpty(match.ResultMethod, match.Method, match.WinType, rawResult))
	if outcome == "walkover" || outcome == "bye" {
		method = outcome
	}
	if method == "unknown" && (outcome == "win" || outcome == "loss") {
		method = ""
	}

	normalized := contract.Match{
		SourceID:         sourceID,
		SourceURL:        strings.TrimSpace(match.MatchURL),
		EventSourceID:    context.Event.SourceID,
		EventName:        context.Event.Name,
		AthleteSourceID:  context.AthleteSourceID,
		OpponentSourceID: opponentSourceID,
		OpponentName:     opponentName,
		OpponentCountry:  strings.TrimSpace(match.OpponentCountry),
		Division:         firstNonEmpty(context.Registration.Division, context.Registration.Name),
		AgeCategory:      strings.TrimSpace(context.Registration.AgeCategory),
		Rank:             strings.TrimSpace(context.Registration.Rank),
		WeightClass:      strings.TrimSpace(context.Registration.WeightClass),
		Outcome:          outcome,
		FinishMethod:     method,
		ResultText:       rawResult,
		ScoreText:        strings.TrimSpace(match.Score),
		BracketLabel:     firstNonEmpty(strings.TrimSpace(context.Registration.BracketName), strings.TrimSpace(context.Registration.Name)),
		RoundLabel:       strings.TrimSpace(match.RoundName),
		Placement:        strings.TrimSpace(match.Placement),
		Confidence:       matchConfidence(method, opponentName, context.Registration),
		StartsAt:         context.Event.StartsAt,
		RawReferenceIDs:  []string{context.SnapshotID},
		Attributes:       map[string]string{},
	}
	if context.Event.SourceURL != "" {
		normalized.Attributes["event_url"] = context.Event.SourceURL
	}

	warnings := make([]contract.Warning, 0, 2)
	if opponentName == "" {
		warnings = append(warnings, contract.Warning{
			Code:             "match_opponent_hidden",
			Message:          "match was visible but opponent identity was not exposed",
			SubjectType:      "match",
			SubjectID:        normalized.SourceID,
			SourceSnapshotID: context.SnapshotID,
		})
	}
	if normalized.FinishMethod == "" && (normalized.Outcome == "win" || normalized.Outcome == "loss") {
		warnings = append(warnings, contract.Warning{
			Code:             "match_finish_method_missing",
			Message:          "match outcome was visible but finish method was not exposed",
			SubjectType:      "match",
			SubjectID:        normalized.SourceID,
			SourceSnapshotID: context.SnapshotID,
		})
	}
	if normalized.Division == "" && normalized.BracketLabel == "" {
		warnings = append(warnings, contract.Warning{
			Code:             "match_context_partial",
			Message:          "match was visible without division or bracket context",
			SubjectType:      "match",
			SubjectID:        normalized.SourceID,
			SourceSnapshotID: context.SnapshotID,
		})
	}

	return normalized, warnings
}

func normalizeMatchOutcome(match profileEventMatch, raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(raw, "bye"):
		return "bye"
	case strings.Contains(raw, "walkover") || raw == "wo":
		return "walkover"
	case strings.Contains(raw, "draw"):
		return "draw"
	case match.IsWinner:
		return "win"
	case raw != "":
		return "loss"
	default:
		return ""
	}
}

func matchConfidence(method string, opponentName string, registration profileEventRegistration) string {
	switch {
	case method != "" && method != "unknown" && opponentName != "" && firstNonEmpty(registration.Division, registration.BracketName, registration.Name) != "":
		return "high"
	case opponentName != "" || method != "":
		return "medium"
	default:
		return "low"
	}
}

func syntheticMatchID(athleteSourceID, eventSourceID string, registrationID int64, matchIndex int) string {
	parts := []string{athleteSourceID}
	if eventSourceID != "" {
		parts = append(parts, eventSourceID)
	}
	if registrationID > 0 {
		parts = append(parts, strconv.FormatInt(registrationID, 10))
	}
	parts = append(parts, strconv.Itoa(matchIndex))
	return "match:" + slugify(strings.Join(parts, "-"))
}

func int64String(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func normalizeProfileEventStatus(event profileEvent) string {
	if strings.TrimSpace(event.EventStatus) != "" {
		return strings.ToLower(strings.TrimSpace(event.EventStatus))
	}
	if event.IsUpcoming {
		return "scheduled"
	}
	if strings.TrimSpace(event.StartDate) != "" {
		return "completed"
	}
	return ""
}
