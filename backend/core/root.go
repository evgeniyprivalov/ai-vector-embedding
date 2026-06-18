package root

import (
	"embed"
)

// MigationsOptions represents configuration options for database migrations.
type MigationsOptions struct{}

//go:embed migrations
var migrations embed.FS

// GetMigrationsDir returns the directory path containing migration files.
func (mo *MigationsOptions) GetMigrationsDir() string {
	return "migrations"
}

// GetMigrationsFS returns the embedded filesystem containing migration files.
func (mo *MigationsOptions) GetMigrationsFS() embed.FS {
	return migrations
}
