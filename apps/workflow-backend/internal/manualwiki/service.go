package manualwiki

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type ImportManualRequest = registry.SiteManualImportRequest
type PreviewContextRequest = registry.SiteManualWikiSearchQuery

type ImportManualResponse struct {
	Source registry.SiteManualSource      `json:"source"`
	Pages  []registry.SiteManualWikiPage  `json:"pages"`
	Chunks []registry.SiteManualWikiChunk `json:"chunks"`
}

type Service struct {
	repo     registry.SiteManualRepository
	compiler Compiler
}

func NewService(repo registry.SiteManualRepository) *Service {
	return &Service{repo: repo, compiler: FallbackCompiler{}}
}

func NewServiceWithCompiler(repo registry.SiteManualRepository, compiler Compiler) *Service {
	if compiler == nil {
		compiler = FallbackCompiler{}
	}
	return &Service{repo: repo, compiler: compiler}
}

func (service *Service) ImportManual(ctx context.Context, request ImportManualRequest) (ImportManualResponse, error) {
	if service.repo == nil {
		return ImportManualResponse{}, errors.New("site manual repository is not configured")
	}
	request.ProjectID = defaultProjectID(request.ProjectID)
	request.Site = strings.TrimSpace(request.Site)
	request.Module = strings.TrimSpace(request.Module)
	request.Content = strings.TrimSpace(request.Content)
	if request.SourceType == "" {
		request.SourceType = registry.SiteManualSourceText
	}
	if err := registry.ValidateSiteManualImport(request); err != nil {
		return ImportManualResponse{}, err
	}
	hash := contentHash(request.Content)
	if existing, err := service.repo.FindSiteManualSourceByHash(ctx, registry.SiteManualSourceHashQuery{
		ProjectID:   request.ProjectID,
		Site:        request.Site,
		Module:      request.Module,
		ContentHash: hash,
	}); err == nil {
		wiki, _ := service.repo.GetSiteManualWikiForSource(ctx, existing.ID)
		return ImportManualResponse{Source: existing, Pages: wiki.Pages, Chunks: wiki.Chunks}, nil
	}
	source := registry.SiteManualSource{
		ProjectID:   request.ProjectID,
		Site:        request.Site,
		Module:      request.Module,
		Title:       strings.TrimSpace(request.Title),
		SourceType:  request.SourceType,
		ContentHash: hash,
		RawContent:  request.Content,
		Metadata:    request.Metadata,
		Status:      registry.StatusActive,
		CreatedAt:   time.Now().UTC(),
	}
	if source.Title == "" {
		source.Title = "Site manual"
	}
	if err := service.repo.SaveSiteManualSource(ctx, source); err != nil {
		return ImportManualResponse{}, err
	}
	saved, err := service.repo.FindSiteManualSourceByHash(ctx, registry.SiteManualSourceHashQuery{
		ProjectID:   source.ProjectID,
		Site:        source.Site,
		Module:      source.Module,
		ContentHash: source.ContentHash,
	})
	if err != nil {
		return ImportManualResponse{}, err
	}
	pages, chunks, err := service.compiler.Compile(saved)
	if err != nil {
		return ImportManualResponse{}, err
	}
	if err := service.repo.SaveSiteManualWiki(ctx, pages, chunks); err != nil {
		return ImportManualResponse{}, err
	}
	return ImportManualResponse{Source: saved, Pages: pages, Chunks: chunks}, nil
}

func (service *Service) Rebuild(ctx context.Context, sourceID string) (ImportManualResponse, error) {
	source, err := service.repo.GetSiteManualSource(ctx, sourceID)
	if err != nil {
		return ImportManualResponse{}, err
	}
	pages, chunks, err := service.compiler.Compile(source)
	if err != nil {
		return ImportManualResponse{}, err
	}
	if err := service.repo.SaveSiteManualWiki(ctx, pages, chunks); err != nil {
		return ImportManualResponse{}, err
	}
	return ImportManualResponse{Source: source, Pages: pages, Chunks: chunks}, nil
}

func (service *Service) PreviewContext(ctx context.Context, request PreviewContextRequest) (registry.SiteManualPreviewContext, error) {
	if service.repo == nil {
		return registry.SiteManualPreviewContext{}, errors.New("site manual repository is not configured")
	}
	request.ProjectID = defaultProjectID(request.ProjectID)
	request.Limit = 3
	request.IncludeFiltered = true
	raw, err := service.repo.SearchSiteManualWikiChunks(ctx, request)
	if err != nil {
		return registry.SiteManualPreviewContext{}, err
	}
	preview := registry.SiteManualPreviewContext{}
	for _, match := range raw {
		if match.Score < 0 {
			preview.Filtered = append(preview.Filtered, registry.SiteManualFilteredReason{ChunkID: match.Chunk.ID, Reason: match.Reason})
			continue
		}
		preview.Matches = append(preview.Matches, match)
	}
	if len(preview.Matches) > 3 {
		preview.Matches = preview.Matches[:3]
	}
	preview.Prompt = FormatPrompt(preview.Matches)
	return preview, nil
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func defaultProjectID(projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "default"
	}
	return projectID
}
