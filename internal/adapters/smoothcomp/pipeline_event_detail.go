package smoothcomp

import (
	"context"
	"net/http"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

type EventDetailPipeline struct {
	client *Client
}

func NewEventDetailPipeline(client *Client) *EventDetailPipeline {
	return &EventDetailPipeline{client: client}
}

func (p *EventDetailPipeline) Name() job.Pipeline { return job.PipelineSmoothcompEventDetail }
func (p *EventDetailPipeline) Provider() string   { return providerName }
func (p *EventDetailPipeline) ParserVersion() string {
	return parserVersionEventDetail
}
func (p *EventDetailPipeline) NormalizationVersion() string {
	return normalizationVersion
}

func (p *EventDetailPipeline) Fetch(ctx context.Context, request job.Request) ([]job.RawSnapshot, error) {
	eventURL, err := p.client.BuildEventURL(request.EventID, request.EventURL)
	if err != nil {
		return nil, err
	}
	eventID := firstNonEmpty(request.EventID, extractIDFromURL(eventURL))
	resp, body, err := p.client.Fetch(ctx, http.MethodGet, eventURL, "text/html,application/xhtml+xml", nil)
	if err != nil {
		return nil, err
	}

	snapshots := []job.RawSnapshot{snapshotForResponse(
		p.ParserVersion(),
		"event_detail_html",
		eventID,
		eventURL,
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		body,
		map[string]string{"event_id": eventID},
	)}

	if eventID != "" {
		if url, err := p.client.BuildEventInfoPanelsURL(eventID, eventURL); err == nil {
			if optionalResp, optionalBody, fetchErr := p.client.FetchOptional(ctx, http.MethodGet, url, "application/json", nil); fetchErr == nil {
				snapshots = append(snapshots, snapshotForResponse(
					p.ParserVersion(),
					"event_info_panels_json",
					eventID,
					url,
					optionalResp.StatusCode,
					optionalResp.Header.Get("Content-Type"),
					optionalBody,
					nil,
				))
			}
		}
		if url, err := p.client.BuildEventCMSURL(eventID, eventURL); err == nil {
			if optionalResp, optionalBody, fetchErr := p.client.FetchOptional(ctx, http.MethodGet, url, "application/json", nil); fetchErr == nil {
				snapshots = append(snapshots, snapshotForResponse(
					p.ParserVersion(),
					"event_cms_json",
					eventID,
					url,
					optionalResp.StatusCode,
					optionalResp.Header.Get("Content-Type"),
					optionalBody,
					nil,
				))
			}
		}
	}

	return snapshots, nil
}

func (p *EventDetailPipeline) Normalize(_ context.Context, request job.Request, snapshots []job.RawSnapshot) (contract.Envelope, error) {
	if len(snapshots) == 0 {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.event_detail.normalize", true, "no snapshots received", nil)
	}

	var htmlSnapshot *job.RawSnapshot
	var infoPanelsSnapshot *job.RawSnapshot
	var cmsSnapshot *job.RawSnapshot
	for i := range snapshots {
		switch snapshots[i].ResourceType {
		case "event_detail_html":
			htmlSnapshot = &snapshots[i]
		case "event_info_panels_json":
			infoPanelsSnapshot = &snapshots[i]
		case "event_cms_json":
			cmsSnapshot = &snapshots[i]
		}
	}
	if htmlSnapshot == nil {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.event_detail.normalize", true, "missing event detail html snapshot", nil)
	}

	parsed, err := parseEventDetailHTML(htmlSnapshot.Body, request.EventID, request.EventURL, htmlSnapshot.ID)
	if err != nil {
		return contract.Envelope{}, err
	}

	warnings := make([]contract.Warning, 0)
	if infoPanelsSnapshot != nil {
		if infoPanelsSnapshot.StatusCode >= 200 && infoPanelsSnapshot.StatusCode < 300 {
			if panels, parseErr := parseEventInfoPanelsJSON(infoPanelsSnapshot.Body); parseErr == nil {
				mergeEventPanels(&parsed.Event, panels)
			} else {
				warnings = append(warnings, contract.Warning{
					Code:             "event_info_panels_parse_failed",
					Message:          parseErr.Error(),
					SubjectType:      "event",
					SubjectID:        parsed.Event.SourceID,
					SourceSnapshotID: infoPanelsSnapshot.ID,
				})
			}
		} else {
			warnings = append(warnings, contract.Warning{
				Code:             "event_info_panels_unavailable",
				Message:          "event info panels endpoint returned non-success status",
				SubjectType:      "event",
				SubjectID:        parsed.Event.SourceID,
				SourceSnapshotID: infoPanelsSnapshot.ID,
			})
		}
	}

	if cmsSnapshot != nil {
		if cmsSnapshot.StatusCode >= 200 && cmsSnapshot.StatusCode < 300 {
			if cms, parseErr := parseEventCMSJSON(cmsSnapshot.Body); parseErr == nil {
				parsed.Event.Attributes = mergeStringMap(parsed.Event.Attributes, cms.Attributes)
			} else {
				warnings = append(warnings, contract.Warning{
					Code:             "event_cms_parse_failed",
					Message:          parseErr.Error(),
					SubjectType:      "event",
					SubjectID:        parsed.Event.SourceID,
					SourceSnapshotID: cmsSnapshot.ID,
				})
			}
		} else {
			warnings = append(warnings, contract.Warning{
				Code:             "event_cms_unavailable",
				Message:          "event cms endpoint returned non-success status",
				SubjectType:      "event",
				SubjectID:        parsed.Event.SourceID,
				SourceSnapshotID: cmsSnapshot.ID,
			})
		}
	}

	return contract.Envelope{
		ContractVersion:      contract.CurrentContractVersion,
		Provider:             providerName,
		Pipeline:             string(job.PipelineSmoothcompEventDetail),
		CorrelationID:        request.CorrelationID,
		ParserVersion:        p.ParserVersion(),
		NormalizationVersion: p.NormalizationVersion(),
		GeneratedAt:          time.Now().UTC(),
		Scope: contract.Scope{
			EventID: parsed.Event.SourceID,
		},
		Events:   []contract.Event{parsed.Event},
		Warnings: warnings,
		Metadata: map[string]string{
			"primary_snapshot_id": htmlSnapshot.ID,
		},
	}, nil
}

func (p *EventDetailPipeline) Publish(_ context.Context, _ job.Request, normalized contract.Envelope) (contract.Envelope, error) {
	return normalized, nil
}

func mergeEventPanels(event *contract.Event, panels eventPanelsData) {
	if event == nil {
		return
	}
	if event.VenueName == "" {
		event.VenueName = panels.VenueName
	}
	if event.City == "" {
		event.City = panels.City
	}
	if event.Country == "" {
		event.Country = panels.Country
	}
	if event.VenueAddress == "" {
		event.VenueAddress = panels.VenueAddress
	}
	if event.OrganizerName == "" {
		event.OrganizerName = panels.OrganizerName
	}
	event.Attributes = mergeStringMap(event.Attributes, panels.Attributes)
}
