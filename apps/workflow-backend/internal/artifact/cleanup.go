package artifact

import (
	"context"
	"errors"
	"io"
	"time"
)

type CleanupResult struct {
	Deleted int
	Errors  []string
}

func CleanupExpired(ctx context.Context, service *Service, now time.Time) (CleanupResult, error) {
	artifacts, err := service.ListAll(ctx)
	if err != nil {
		return CleanupResult{}, err
	}

	result := CleanupResult{}
	for _, artifact := range artifacts {
		if artifact.ExpiresAt == nil || artifact.ExpiresAt.After(now) {
			continue
		}
		if err := service.Delete(ctx, artifact.ID); err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		result.Deleted++
	}
	return result, nil
}

type VerifyReport struct {
	Checked      int
	MissingFiles []string
	SHA256Errors []string
}

func VerifyArtifacts(ctx context.Context, service *Service) (VerifyReport, error) {
	artifacts, err := service.ListAll(ctx)
	if err != nil {
		return VerifyReport{}, err
	}

	report := VerifyReport{}
	for _, artifact := range artifacts {
		report.Checked++
		reader, err := service.store.Get(ctx, artifact)
		if err != nil {
			report.MissingFiles = append(report.MissingFiles, artifact.ID)
			continue
		}
		_, copyErr := io.Copy(io.Discard, reader)
		closeErr := reader.Close()
		if errors.Join(copyErr, closeErr) != nil {
			report.SHA256Errors = append(report.SHA256Errors, artifact.ID)
		}
	}
	return report, nil
}
