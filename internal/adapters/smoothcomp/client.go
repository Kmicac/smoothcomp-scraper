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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, coreerrors.New(coreerrors.CategoryExternal, coreerrors.CodeUnexpectedStatus, "smoothcomp.fetch", true, fmt.Sprintf("provider returned status %d", resp.StatusCode), nil)
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
