package smoothcomp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
)

type participantsResponse struct {
	Participants []participantCategory `json:"participants"`
}

type participantCategory struct {
	Name          string         `json:"name"`
	Registrations []registration `json:"registrations"`
}

type registration struct {
	ID              int64         `json:"id"`
	ClubID          int64         `json:"club_id"`
	AffiliationID   int64         `json:"affiliation_id"`
	SeedPosition    *int          `json:"seed_position"`
	AffiliationName string        `json:"affiliationName"`
	Age             int           `json:"age"`
	Birth           string        `json:"birth"`
	ClubName        string        `json:"clubName"`
	CountryCode     string        `json:"cn"`
	Country         string        `json:"country"`
	FirstName       string        `json:"firstname"`
	Gender          string        `json:"gender"`
	LastName        string        `json:"lastname"`
	MiddleName      string        `json:"middle_name"`
	ProfileImage    string        `json:"profile_image"`
	Categories      []regCategory `json:"categories"`
	UserID          int64         `json:"user_id"`
}

type regCategory struct {
	WeightMeasured *string `json:"weight_measured"`
}

func parseParticipantsJSON(body []byte, eventID, eventName, eventURL, snapshotID string) (contract.Event, []contract.Organization, []contract.Person, []contract.Registration, error) {
	var payload participantsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return contract.Event{}, nil, nil, nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_participants_json", true, "failed to decode participants payload", err)
	}

	event := contract.Event{
		SourceID:        eventID,
		SourceURL:       eventURL,
		Name:            eventName,
		RawReferenceIDs: []string{snapshotID},
	}

	orgsByID := map[string]contract.Organization{}
	peopleByID := map[string]contract.Person{}
	registrations := make([]contract.Registration, 0)

	for _, category := range payload.Participants {
		division, ageCategory, rank, weightClass := parseCategory(category.Name)
		for _, reg := range category.Registrations {
			personID := fmt.Sprintf("athlete:%d", reg.UserID)
			orgID := organizationID(reg.ClubID, reg.ClubName, reg.AffiliationID, reg.AffiliationName)

			if orgID != "" {
				if _, ok := orgsByID[orgID]; !ok {
					orgsByID[orgID] = contract.Organization{
						SourceID:        orgID,
						Name:            firstNonEmpty(reg.ClubName, reg.AffiliationName),
						Kind:            "team",
						CountryCode:     strings.ToUpper(strings.TrimSpace(reg.CountryCode)),
						RawReferenceIDs: []string{snapshotID},
					}
				}
			}

			person := contract.Person{
				SourceID:             personID,
				GivenName:            strings.TrimSpace(reg.FirstName),
				FamilyName:           strings.TrimSpace(reg.LastName),
				FullName:             strings.TrimSpace(strings.Join(filterEmpty(reg.FirstName, reg.MiddleName, reg.LastName), " ")),
				Country:              strings.TrimSpace(reg.Country),
				CountryCode:          strings.ToUpper(strings.TrimSpace(reg.CountryCode)),
				Gender:               strings.TrimSpace(reg.Gender),
				ProfileURL:           fmt.Sprintf("https://smoothcomp.com/en/profile/%d", reg.UserID),
				ImageURL:             strings.TrimSpace(reg.ProfileImage),
				OrganizationSourceID: orgID,
				RawReferenceIDs:      []string{snapshotID},
				Attributes: map[string]string{
					"affiliation_name": strings.TrimSpace(reg.AffiliationName),
				},
			}
			if reg.Age > 0 {
				age := reg.Age
				person.Age = &age
				person.Attributes["age"] = strconv.Itoa(reg.Age)
			}
			if reg.Birth != "" {
				if year, err := strconv.Atoi(reg.Birth); err == nil {
					person.BirthYear = &year
				}
			}
			peopleByID[personID] = person

			registrationID := fmt.Sprintf("registration:%d", reg.ID)
			record := contract.Registration{
				SourceID:             registrationID,
				EventSourceID:        eventID,
				PersonSourceID:       personID,
				OrganizationSourceID: orgID,
				Division:             division,
				AgeCategory:          ageCategory,
				Rank:                 rank,
				WeightClass:          weightClass,
				RawReferenceIDs:      []string{snapshotID},
			}
			if reg.SeedPosition != nil {
				record.Seed = reg.SeedPosition
			}
			if weight := measuredWeight(reg.Categories); weight != nil {
				record.ActualWeight = weight
			}
			registrations = append(registrations, record)
		}
	}

	organizations := make([]contract.Organization, 0, len(orgsByID))
	for _, item := range orgsByID {
		organizations = append(organizations, item)
	}
	people := make([]contract.Person, 0, len(peopleByID))
	for _, item := range peopleByID {
		people = append(people, item)
	}

	return event, organizations, people, registrations, nil
}

func parseCategory(category string) (string, string, string, string) {
	parts := strings.Split(category, "/")
	if len(parts) < 4 {
		return "", "", "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), strings.TrimSpace(parts[3])
}

func measuredWeight(categories []regCategory) *float64 {
	for _, category := range categories {
		if category.WeightMeasured == nil || strings.TrimSpace(*category.WeightMeasured) == "" {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(*category.WeightMeasured), 64)
		if err == nil {
			return &value
		}
	}
	return nil
}

func organizationID(clubID int64, clubName string, affiliationID int64, affiliationName string) string {
	switch {
	case clubID > 0:
		return fmt.Sprintf("club:%d", clubID)
	case affiliationID > 0:
		return fmt.Sprintf("affiliation:%d", affiliationID)
	case strings.TrimSpace(clubName) != "":
		return "club_name:" + slugify(clubName)
	case strings.TrimSpace(affiliationName) != "":
		return "affiliation_name:" + slugify(affiliationName)
	default:
		return ""
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "/", "-")
	filtered := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			filtered = append(filtered, r)
		}
	}
	return string(filtered)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func filterEmpty(values ...string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, strings.TrimSpace(value))
		}
	}
	return items
}
