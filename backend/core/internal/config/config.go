package config

type Config struct {
	ServiceName string `envconfig:"SERVICE_NAME" required:"true"`
	Version     string `envconfig:"VERSION" required:"true"`
	Debug       bool   `envconfig:"DEBUG" required:"true"`
	Environment string `envconfig:"ENVIRONMENT" required:"true"`

	ServerHTTPAddr   string   `envconfig:"SERVER_HTTP_ADDR" required:"true"`
	HealthServerAddr string   `envconfig:"HEALTH_SERVER_ADDR" required:"true"`
	AllowedOrigins   []string `envconfig:"ALLOWED_ORIGINS" required:"true"`

	PostgreSQLDSN string `envconfig:"POSTGRESQL_DSN" required:"true"`

	ChunkerHost       string `envconfig:"CHUNKER_SERVER_GRPC_HOST" required:"true"`
	ChunkerWithSecure bool   `envconfig:"CHUNKER_SERVER_GRPC_SECURE" required:"true"`

	AiAPIKey string `envconfig:"AI_API_KEY" required:"true"`
	AiHost   string `envconfig:"AI_HOST" required:"true"`
}
