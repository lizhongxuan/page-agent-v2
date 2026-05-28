package migrations

import "embed"

//go:embed *.sql
var migrationFS embed.FS

type File struct {
	Name string
	SQL  string
}

func Files() ([]File, error) {
	entries, err := migrationFS.ReadDir(".")
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := migrationFS.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, File{Name: entry.Name(), SQL: string(content)})
	}
	return files, nil
}
