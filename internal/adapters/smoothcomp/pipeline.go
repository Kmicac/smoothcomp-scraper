package smoothcomp

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

const (
	parserVersionEventCatalog      = "smoothcomp.event_catalog.html.v1"
	parserVersionEventParticipants = "smoothcomp.event_participants.json.v1"
	parserVersionEventDetail       = "smoothcomp.event_detail.v1"
	parserVersionAthleteProfile    = "smoothcomp.athlete_profile.v1"
	parserVersionAcademyCatalog    = "smoothcomp.academy_catalog.v1"
	normalizationVersion           = "technical-normalization.v1"
)

type EventCatalogPipeline struct {
	client *Client
}

func NewEventCatalogPipeline(client *Client) *EventCatalogPipeline {
	return &EventCatalogPipeline{client: client}
}

func (p *EventCatalogPipeline) Name() job.Pipeline { return job.PipelineSmoothcompEventCatalog }
func (p *EventCatalogPipeline) Provider() string   { return providerName }
func (p *EventCatalogPipeline) ParserVersion() string {
	return parserVersionEventCatalog
}
func (p *EventCatalogPipeline) NormalizationVersion() string {
	return normalizationVersion
}

func (p *EventCatalogPipeline) Fetch(ctx context.Context, request job.Request) ([]job.RawSnapshot, error) {
	eventsURL, err := p.client.BuildEventsURL(strings.ToLower(request.EventType), request.Country)
	if err != nil {
		return nil, err
	}
	resp, body, err := p.client.Fetch(ctx, http.MethodGet, eventsURL, "text/html,application/xhtml+xml", nil)
	if err != nil {
		return nil, err
	}
	return []job.RawSnapshot{snapshotForResponse(
		p.ParserVersion(),
		"event_catalog_html",
		request.Country+"-"+strings.ToLower(request.EventType),
		eventsURL,
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		body,
		map[string]string{
			"country":    request.Country,
			"event_type": strings.ToLower(request.EventType),
		},
	)}, nil
}

func (p *EventCatalogPipeline) Normalize(_ context.Context, request job.Request, snapshots []job.RawSnapshot) (contract.Envelope, error) {
	if len(snapshots) == 0 {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.event_catalog.normalize", true, "no snapshots received", nil)
	}
	events, err := parseEventCatalogHTML(p.client.baseURL, strings.ToLower(request.EventType), snapshots[0].Body, snapshots[0].ID)
	if err != nil {
		return contract.Envelope{}, err
	}
	return contract.Envelope{
		ContractVersion:      contract.CurrentContractVersion,
		Provider:             providerName,
		Pipeline:             string(job.PipelineSmoothcompEventCatalog),
		CorrelationID:        request.CorrelationID,
		ParserVersion:        p.ParserVersion(),
		NormalizationVersion: p.NormalizationVersion(),
		GeneratedAt:          time.Now().UTC(),
		Scope: contract.Scope{
			Country:   request.Country,
			EventType: strings.ToLower(request.EventType),
		},
		Events: events,
		Metadata: map[string]string{
			"snapshot_id": snapshots[0].ID,
		},
	}, nil
}

func (p *EventCatalogPipeline) Publish(_ context.Context, _ job.Request, normalized contract.Envelope) (contract.Envelope, error) {
	return normalized, nil
}

type EventParticipantsPipeline struct {
	client *Client
}

func NewEventParticipantsPipeline(client *Client) *EventParticipantsPipeline {
	return &EventParticipantsPipeline{client: client}
}

func (p *EventParticipantsPipeline) Name() job.Pipeline {
	return job.PipelineSmoothcompEventParticipants
}
func (p *EventParticipantsPipeline) Provider() string { return providerName }
func (p *EventParticipantsPipeline) ParserVersion() string {
	return parserVersionEventParticipants
}
func (p *EventParticipantsPipeline) NormalizationVersion() string {
	return normalizationVersion
}

func (p *EventParticipantsPipeline) Fetch(ctx context.Context, request job.Request) ([]job.RawSnapshot, error) {
	participantsURL, err := p.client.BuildParticipantsURL(request.EventID, request.EventURL)
	if err != nil {
		return nil, err
	}
	resp, body, err := p.client.Fetch(ctx, http.MethodPost, participantsURL, "application/json, text/javascript, */*", nil)
	if err != nil {
		return nil, err
	}
	return []job.RawSnapshot{snapshotForResponse(
		p.ParserVersion(),
		"event_participants_json",
		request.EventID,
		participantsURL,
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		body,
		map[string]string{
			"event_id": request.EventID,
		},
	)}, nil
}

func (p *EventParticipantsPipeline) Normalize(_ context.Context, request job.Request, snapshots []job.RawSnapshot) (contract.Envelope, error) {
	if len(snapshots) == 0 {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.event_participants.normalize", true, "no snapshots received", nil)
	}
	event, organizations, people, registrations, err := parseParticipantsJSON(snapshots[0].Body, request.EventID, request.EventName, request.EventURL, snapshots[0].ID)
	if err != nil {
		return contract.Envelope{}, err
	}
	return contract.Envelope{
		ContractVersion:      contract.CurrentContractVersion,
		Provider:             providerName,
		Pipeline:             string(job.PipelineSmoothcompEventParticipants),
		CorrelationID:        request.CorrelationID,
		ParserVersion:        p.ParserVersion(),
		NormalizationVersion: p.NormalizationVersion(),
		GeneratedAt:          time.Now().UTC(),
		Scope: contract.Scope{
			EventID: request.EventID,
		},
		Events:        []contract.Event{event},
		Organizations: organizations,
		People:        people,
		Registrations: registrations,
		Metadata: map[string]string{
			"snapshot_id": snapshots[0].ID,
		},
	}, nil
}

func (p *EventParticipantsPipeline) Publish(_ context.Context, _ job.Request, normalized contract.Envelope) (contract.Envelope, error) {
	return normalized, nil
}
