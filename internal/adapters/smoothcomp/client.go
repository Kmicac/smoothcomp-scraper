package smoothcomp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"go.uber.org/zap"
)

const providerName = "smoothcomp"

type Client struct {
	baseURL    string
	userAgent  string
	delay      time.Duration
	httpClient *http.Client
	logger     *zap.Logger
}

func NewClient(cfg platformconfig.SmoothcompConfig, logger *zap.Logger) *Client {
	return &Client{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		userAgent: cfg.UserAgent,
		delay:     cfg.RequestDelay,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: logger,
	}
}

func (c *Client) Fetch(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, []byte, error) {
	resp, payload, err := c.fetchRaw(ctx, method, rawURL, contentType, body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, coreerrors.New(coreerrors.CategoryExternal, coreerrors.CodeUnexpectedStatus, "smoothcomp.fetch", true, fmt.Sprintf("provider returned status %d", resp.StatusCode), nil)
	}
	return resp, payload, nil
}

func (c *Client) FetchOptional(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, []byte, error) {
	return c.fetchRaw(ctx, method, rawURL, contentType, body)
}

func (c *Client) fetchRaw(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, []byte, error) {
	if c.delay > 0 {
		timer := time.NewTimer(c.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-timer.C:
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.fetch", false, "failed to build provider request", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if contentType != "" {
		req.Header.Set("Accept", contentType)
	}
	if method == http.MethodPost {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, coreerrors.New(coreerrors.CategoryExternal, coreerrors.CodeProviderFailed, "smoothcomp.fetch", true, "provider request failed", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, coreerrors.New(coreerrors.CategoryExternal, coreerrors.CodeProviderFailed, "smoothcomp.fetch", true, "failed to read provider response", err)
	}

	return resp, payload, nil
}

func (c *Client) BuildEventsURL(eventType, country string) (string, error) {
	if eventType != "past" && eventType != "upcoming" {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_events_url", false, "event_type must be past or upcoming", nil)
	}
	parsed, err := url.Parse(c.baseURL + "/en/events/" + eventType)
	if err != nil {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_events_url", false, "invalid base url", err)
	}
	query := parsed.Query()
	if country != "" {
		query.Set("countries", strings.ToUpper(strings.TrimSpace(country)))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) BuildParticipantsURL(eventID, eventURL string) (string, error) {
	if eventID == "" {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_participants_url", false, "event_id is required", nil)
	}
	host := "smoothcomp.com"
	if eventURL != "" {
		if parsed, err := url.Parse(eventURL); err == nil && parsed.Host != "" {
			host = parsed.Host
		}
	}
	return fmt.Sprintf("https://%s/en/event/%s/participants", host, eventID), nil
}

func (c *Client) BuildEventURL(eventID, eventURL string) (string, error) {
	if strings.TrimSpace(eventURL) != "" {
		return strings.TrimSpace(eventURL), nil
	}
	if strings.TrimSpace(eventID) == "" {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_event_url", false, "event_id or event_url is required", nil)
	}
	return fmt.Sprintf("%s/en/event/%s", c.baseURL, strings.TrimSpace(eventID)), nil
}

func (c *Client) BuildEventInfoPanelsURL(eventID, eventURL string) (string, error) {
	base, err := c.BuildEventURL(eventID, eventURL)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_event_info_panels_url", false, "invalid event url", err)
	}
	return parsed.Scheme + "://" + parsed.Host + "/en/event/" + strings.TrimSpace(eventID) + "/getInfoPanelsData", nil
}

func (c *Client) BuildEventCMSURL(eventID, eventURL string) (string, error) {
	base, err := c.BuildEventURL(eventID, eventURL)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_event_cms_url", false, "invalid event url", err)
	}
	return parsed.Scheme + "://" + parsed.Host + "/en/event/" + strings.TrimSpace(eventID) + "/getCmsData", nil
}

func (c *Client) BuildProfileURL(profileID, profileURL string) (string, error) {
	if strings.TrimSpace(profileURL) != "" {
		return strings.TrimSpace(profileURL), nil
	}
	if strings.TrimSpace(profileID) == "" {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_profile_url", false, "profile_id or profile_url is required", nil)
	}
	return fmt.Sprintf("%s/en/profile/%s", c.baseURL, strings.TrimSpace(profileID)), nil
}

func (c *Client) BuildProfileEventsURL(profileID string, pageURL string) (string, error) {
	if strings.TrimSpace(pageURL) != "" {
		if strings.HasPrefix(pageURL, "/") {
			return c.baseURL + strings.TrimSpace(pageURL), nil
		}
		return strings.TrimSpace(pageURL), nil
	}
	if strings.TrimSpace(profileID) == "" {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_profile_events_url", false, "profile_id is required", nil)
	}
	return fmt.Sprintf("%s/en/profile/%s/events", c.baseURL, strings.TrimSpace(profileID)), nil
}

func (c *Client) BuildAcademyCatalogURL(country string) (string, error) {
	parsed, err := url.Parse(c.baseURL + "/en/club")
	if err != nil {
		return "", coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "smoothcomp.build_academy_catalog_url", false, "invalid base url", err)
	}
	if strings.TrimSpace(country) != "" {
		query := parsed.Query()
		query.Set("countries", strings.ToUpper(strings.TrimSpace(country)))
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), nil
}

func snapshotForResponse(parserVersion, resourceType, resourceKey, sourceURL string, status int, contentType string, body []byte, metadata map[string]string) job.RawSnapshot {
	sum := sha256.Sum256(body)
	return job.RawSnapshot{
		ID:            job.NewID("snap"),
		ResourceType:  resourceType,
		ResourceKey:   resourceKey,
		SourceURL:     sourceURL,
		ContentType:   contentType,
		StatusCode:    status,
		ParserVersion: parserVersion,
		CapturedAt:    time.Now().UTC(),
		SHA256:        hex.EncodeToString(sum[:]),
		Body:          body,
		Metadata:      metadata,
	}
}
