package config

type Config struct {
	AiAPIKey      string `envconfig:"AI_API_KEY" required:"true"`
	PostgreSQLDSN string `envconfig:"POSTGRESQL_DSN" required:"true"`
}
