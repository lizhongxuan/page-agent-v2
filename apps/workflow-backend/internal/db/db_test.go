package db

import (
	"context"
	"strings"
	"testing"
)

func TestEmbeddedMigrationsContainCoreTablesAndLocalArtifactDefault(t *testing.T) {
	sql := strings.Join(EmbeddedMigrationSQL(), "\n")
	for _, expected := range []string{
		"create extension if not exists vector",
		"create table if not exists projects",
		"create table if not exists workflow_candidates",
		"create table if not exists workflow_patch_candidates",
		"storage_backend text not null default 'local'",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected migration SQL to contain %q\n%s", expected, sql)
		}
	}
}

func TestMigrationRunnerExecutesMigrationsInOrder(t *testing.T) {
	runner := NewMigrationRunner([]Migration{
		{Name: "001", SQL: "select 1;"},
		{Name: "002", SQL: "select 2;"},
	})
	exec := &recordingExecutor{}

	if err := runner.Run(context.Background(), exec); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := strings.Join(exec.queries, "|"); got != "select 1;|select 2;" {
		t.Fatalf("unexpected execution order: %s", got)
	}
}

type recordingExecutor struct {
	queries []string
}

func (executor *recordingExecutor) ExecContext(_ context.Context, query string, _ ...any) error {
	executor.queries = append(executor.queries, query)
	return nil
}
