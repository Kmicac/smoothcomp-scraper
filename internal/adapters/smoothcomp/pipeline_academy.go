package smoothcomp

import (
	"context"
	"net/http"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

type AcademyCatalogPipeline struct {
	client *Client
}

func NewAcademyCatalogPipeline(client *Client) *AcademyCatalogPipeline {
	return &AcademyCatalogPipeline{client: client}
}

func (p *AcademyCatalogPipeline) Name() job.Pipeline { return job.PipelineSmoothcompAcademyCatalog }
func (p *AcademyCatalogPipeline) Provider() string   { return providerName }
func (p *AcademyCatalogPipeline) ParserVersion() string {
	return parserVersionAcademyCatalog
}
func (p *AcademyCatalogPipeline) NormalizationVersion() string {
	return normalizationVersion
}

func (p *AcademyCatalogPipeline) Fetch(ctx context.Context, request job.Request) ([]job.RawSnapshot, error) {
	catalogURL, err := p.client.BuildAcademyCatalogURL(request.Country)
	if err != nil {
		return nil, err
	}
	resp, body, err := p.client.Fetch(ctx, http.MethodGet, catalogURL, "text/html,application/xhtml+xml", nil)
	if err != nil {
		return nil, err
	}

	catalogSnapshot := snapshotForResponse(
		p.ParserVersion(),
		"academy_catalog_html",
		firstNonEmpty(request.Country, "all"),
		catalogURL,
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		body,
		map[string]string{"country": request.Country},
	)
	snapshots := []job.RawSnapshot{catalogSnapshot}

	items, parseErr := parseAcademyCatalogHTML(p.client.baseURL, body, catalogSnapshot.ID)
	if parseErr != nil {
		return nil, parseErr
	}
	for _, item := range items {
		detailResp, detailBody, fetchErr := p.client.FetchOptional(ctx, http.MethodGet, item.SourceURL, "text/html,application/xhtml+xml", nil)
		if fetchErr != nil {
			continue
		}
		snapshots = append(snapshots, snapshotForResponse(
			p.ParserVersion(),
			"academy_detail_html",
			item.SourceID,
			item.SourceURL,
			detailResp.StatusCode,
			detailResp.Header.Get("Content-Type"),
			detailBody,
			map[string]string{
				"country": request.Country,
				"name":    item.Name,
			},
		))
	}

	return snapshots, nil
}

func (p *AcademyCatalogPipeline) Normalize(_ context.Context, request job.Request, snapshots []job.RawSnapshot) (contract.Envelope, error) {
	if len(snapshots) == 0 {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.academy_catalog.normalize", true, "no snapshots received", nil)
	}
	var catalogSnapshot *job.RawSnapshot
	detailSnapshots := map[string]job.RawSnapshot{}
	for i := range snapshots {
		switch snapshots[i].ResourceType {
		case "academy_catalog_html":
			catalogSnapshot = &snapshots[i]
		case "academy_detail_html":
			detailSnapshots[snapshots[i].ResourceKey] = snapshots[i]
		}
	}
	if catalogSnapshot == nil {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.academy_catalog.normalize", true, "missing academy catalog html snapshot", nil)
	}

	items, err := parseAcademyCatalogHTML(p.client.baseURL, catalogSnapshot.Body, catalogSnapshot.ID)
	if err != nil {
		return contract.Envelope{}, err
	}

	organizations := make([]contract.Organization, 0, len(items))
	warnings := make([]contract.Warning, 0)
	for _, item := range items {
		org := contract.Organization{
			SourceID:        item.SourceID,
			Name:            firstNonEmpty(item.Name, item.SourceID),
			Kind:            "academy",
			CountryCode:     request.Country,
			RawReferenceIDs: []string{catalogSnapshot.ID},
			Attributes:      map[string]string{},
		}

		if snapshot, ok := detailSnapshots[item.SourceID]; ok {
			org.RawReferenceIDs = append(org.RawReferenceIDs, snapshot.ID)
			if snapshot.StatusCode >= 200 && snapshot.StatusCode < 300 {
				if parsed, parseErr := parseAcademyDetailHTML(snapshot.Body, item.SourceURL, item.SourceID, request.Country, snapshot.ID); parseErr == nil {
					org = mergeOrganization(org, parsed)
				} else {
					warnings = append(warnings, contract.Warning{
						Code:             "academy_detail_parse_failed",
						Message:          parseErr.Error(),
						SubjectType:      "organization",
						SubjectID:        item.SourceID,
						SourceSnapshotID: snapshot.ID,
					})
				}
			} else {
				warnings = append(warnings, contract.Warning{
					Code:             "academy_detail_unavailable",
					Message:          "academy detail endpoint returned non-success status",
					SubjectType:      "organization",
					SubjectID:        item.SourceID,
					SourceSnapshotID: snapshot.ID,
				})
			}
		} else {
			warnings = append(warnings, contract.Warning{
				Code:             "academy_detail_missing",
				Message:          "academy detail snapshot was not captured",
				SubjectType:      "organization",
				SubjectID:        item.SourceID,
				SourceSnapshotID: catalogSnapshot.ID,
			})
		}
		organizations = append(organizations, org)
	}

	return contract.Envelope{
		ContractVersion:      contract.CurrentContractVersion,
		Provider:             providerName,
		Pipeline:             string(job.PipelineSmoothcompAcademyCatalog),
		CorrelationID:        request.CorrelationID,
		ParserVersion:        p.ParserVersion(),
		NormalizationVersion: p.NormalizationVersion(),
		GeneratedAt:          time.Now().UTC(),
		Scope: contract.Scope{
			Country: request.Country,
		},
		Organizations: organizations,
		Warnings:      warnings,
		Metadata: map[string]string{
			"primary_snapshot_id": catalogSnapshot.ID,
		},
	}, nil
}

func (p *AcademyCatalogPipeline) Publish(_ context.Context, _ job.Request, normalized contract.Envelope) (contract.Envelope, error) {
	return normalized, nil
}

func mergeOrganization(base contract.Organization, override contract.Organization) contract.Organization {
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.Country != "" {
		base.Country = override.Country
	}
	if override.CountryCode != "" {
		base.CountryCode = override.CountryCode
	}
	if override.Slug != "" {
		base.Slug = override.Slug
	}
	if override.ImageURL != "" {
		base.ImageURL = override.ImageURL
	}
	if override.Description != "" {
		base.Description = override.Description
	}
	if override.WebsiteURL != "" {
		base.WebsiteURL = override.WebsiteURL
	}
	base.Attributes = mergeStringMap(base.Attributes, override.Attributes)
	base.RawReferenceIDs = append(filterEmpty(base.RawReferenceIDs...), filterEmpty(override.RawReferenceIDs...)...)
	return base
}
