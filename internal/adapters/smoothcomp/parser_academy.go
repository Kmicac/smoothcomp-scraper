package smoothcomp

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
)

type academyListingItem struct {
	SourceID   string
	Name       string
	SourceURL  string
	SnapshotID string
}

func parseAcademyCatalogHTML(baseURL string, body []byte, snapshotID string) ([]academyListingItem, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_academy_catalog_html", true, "failed to parse academy catalog html", err)
	}

	seen := map[string]struct{}{}
	items := make([]academyListingItem, 0)
	doc.Find("a[href*='/club/']").Each(func(_ int, selection *goquery.Selection) {
		href, _ := selection.Attr("href")
		sourceURL := absoluteURL(baseURL, href)
		if sourceURL == "" || !strings.Contains(sourceURL, "/en/club/") || strings.Contains(sourceURL, "/finder") {
			return
		}
		sourceID := extractClubIDFromURL(sourceURL)
		if sourceID == "" {
			return
		}
		if _, ok := seen[sourceID]; ok {
			return
		}
		seen[sourceID] = struct{}{}
		name := firstNonEmpty(strings.TrimSpace(selection.Text()), strings.TrimSpace(selection.Find("img").First().AttrOr("alt", "")))
		items = append(items, academyListingItem{
			SourceID:   sourceID,
			Name:       name,
			SourceURL:  sourceURL,
			SnapshotID: snapshotID,
		})
	})

	return items, nil
}

func parseAcademyDetailHTML(body []byte, sourceURL, sourceID, fallbackCountryCode, snapshotID string) (contract.Organization, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return contract.Organization{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_academy_detail_html", true, "failed to parse academy detail html", err)
	}

	org := contract.Organization{
		SourceID:        sourceID,
		Name:            firstNonEmpty(strings.TrimSpace(doc.Find("h1").First().Text()), strings.TrimSpace(doc.Find(".club-name").First().Text())),
		Kind:            "academy",
		Country:         extractAcademyCountry(doc),
		CountryCode:     strings.ToUpper(strings.TrimSpace(fallbackCountryCode)),
		Slug:            slugify(firstNonEmpty(strings.TrimSpace(doc.Find("h1").First().Text()), strings.TrimSpace(doc.Find(".club-name").First().Text()))),
		ImageURL:        firstNonEmpty(absAttr(doc.Find("img.club-logo").First(), "src", sourceURL), absAttr(doc.Find("img.club-cover").First(), "src", sourceURL)),
		Description:     strings.TrimSpace(doc.Find(".club-bio, .club-description").First().Text()),
		WebsiteURL:      strings.TrimSpace(doc.Find("a[href*='http']:not([href*='smoothcomp'])").First().AttrOr("href", "")),
		RawReferenceIDs: []string{snapshotID},
		Attributes:      map[string]string{},
	}

	if org.Name == "" {
		org.Name = sourceID
	}

	doc.Find(".stat-item, .stats-item").Each(func(_ int, selection *goquery.Selection) {
		label := strings.ToLower(strings.TrimSpace(selection.Find(".stat-label, .label").First().Text()))
		value := strings.TrimSpace(selection.Find(".stat-value, .value").First().Text())
		if label == "" || value == "" {
			return
		}
		if parsed, err := strconv.Atoi(strings.ReplaceAll(value, ",", "")); err == nil {
			value = strconv.Itoa(parsed)
		}
		switch {
		case strings.Contains(label, "wins"):
			org.Attributes["total_wins"] = value
		case strings.Contains(label, "losses"):
			org.Attributes["total_losses"] = value
		case strings.Contains(label, "athletes") || strings.Contains(label, "members"):
			org.Attributes["athlete_count"] = value
		case strings.Contains(label, "gold"):
			org.Attributes["gold_medals"] = value
		case strings.Contains(label, "silver"):
			org.Attributes["silver_medals"] = value
		case strings.Contains(label, "bronze"):
			org.Attributes["bronze_medals"] = value
		}
	})

	if instagram := strings.TrimSpace(doc.Find("a[href*='instagram.com']").First().AttrOr("href", "")); instagram != "" {
		org.Attributes["instagram_url"] = instagram
	}
	if facebook := strings.TrimSpace(doc.Find("a[href*='facebook.com']").First().AttrOr("href", "")); facebook != "" {
		org.Attributes["facebook_url"] = facebook
	}

	return org, nil
}

func absoluteURL(baseURL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func absAttr(selection *goquery.Selection, attr string, baseURL string) string {
	if selection.Length() == 0 {
		return ""
	}
	return absoluteURL(baseURL, selection.AttrOr(attr, ""))
}

func extractClubIDFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := filterEmpty(strings.Split(strings.Trim(parsed.Path, "/"), "/")...)
	for index := 0; index < len(parts)-1; index++ {
		if parts[index] == "club" {
			return parts[index+1]
		}
	}
	return extractIDFromURL(rawURL)
}

func extractAcademyCountry(doc *goquery.Document) string {
	for _, selector := range []string{
		".club-country",
		".club-location",
		".club-location-text",
		".location",
		"[data-testid='club-country']",
		"[data-testid='club-location']",
	} {
		if country := countryFromLocationText(doc.Find(selector).First().Text()); country != "" {
			return country
		}
	}

	locationText := ""
	doc.Find("div, li, p, span").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		text := normalizeVisibleText(selection.Text())
		if text == "" {
			return true
		}
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "country:") && !strings.Contains(lower, "location:") {
			return true
		}
		locationText = text
		return false
	})

	return countryFromLocationText(locationText)
}

func countryFromLocationText(raw string) string {
	raw = normalizeVisibleText(raw)
	if raw == "" {
		return ""
	}

	lower := strings.ToLower(raw)
	for _, prefix := range []string{"country:", "location:"} {
		if strings.HasPrefix(lower, prefix) {
			raw = normalizeVisibleText(raw[len(prefix):])
			break
		}
	}

	for _, separator := range []string{",", "|", "/", "•"} {
		if strings.Contains(raw, separator) {
			parts := strings.Split(raw, separator)
			raw = normalizeVisibleText(parts[len(parts)-1])
		}
	}
	for _, separator := range []string{" - ", " – ", " — "} {
		if strings.Contains(raw, separator) {
			parts := strings.Split(raw, separator)
			raw = normalizeVisibleText(parts[len(parts)-1])
		}
	}
	if raw == "" || strings.IndexFunc(raw, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
		return ""
	}
	return raw
}
