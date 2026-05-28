package db

import (
	"context"
	"sort"

	"github.com/page-agent/workflow-backend/internal/migrations"
)

type Executor interface {
	ExecContext(context.Context, string, ...any) error
}

type Migration struct {
	Name string
	SQL  string
}

type MigrationRunner struct {
	migrations []Migration
}

func NewMigrationRunner(migrations []Migration) *MigrationRunner {
	return &MigrationRunner{migrations: append([]Migration(nil), migrations...)}
}

func NewEmbeddedMigrationRunner() (*MigrationRunner, error) {
	migrations, err := EmbeddedMigrations()
	if err != nil {
		return nil, err
	}
	return NewMigrationRunner(migrations), nil
}

func (runner *MigrationRunner) Run(ctx context.Context, executor Executor) error {
	for _, migration := range runner.migrations {
		if err := executor.ExecContext(ctx, migration.SQL); err != nil {
			return err
		}
	}
	return nil
}

func EmbeddedMigrationSQL() []string {
	migrations, err := EmbeddedMigrations()
	if err != nil {
		return nil
	}
	sql := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		sql = append(sql, migration.SQL)
	}
	return sql
}

func EmbeddedMigrations() ([]Migration, error) {
	files, err := migrations.Files()
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	result := make([]Migration, 0, len(files))
	for _, file := range files {
		result = append(result, Migration{Name: file.Name, SQL: file.SQL})
	}
	return result, nil
}
