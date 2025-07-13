package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
	
	"github.com/imranansari/gh-deploy-wf/secrets"
)

// Config holds all configuration for the application
type Config struct {
	// Temporal Configuration
	Temporal TemporalConfig `envPrefix:"TEMPORAL_"`
	
	// GitHub Configuration  
	GitHub GitHubConfig `envPrefix:"GITHUB_"`
	
	// Application Configuration
	App AppConfig `envPrefix:"APP_"`
	
	// Secrets (loaded from files)
	Secrets SecretsConfig
}

type TemporalConfig struct {
	HostPort      string        `env:"HOST" envDefault:"localhost:7233"`
	Namespace     string        `env:"NAMESPACE" envDefault:"default"`
	TaskQueue     string        `env:"TASK_QUEUE" envDefault:"github-deployment-tracker"`
	MaxConcurrent int           `env:"MAX_CONCURRENT" envDefault:"10"`
	WorkerOptions WorkerOptions `envPrefix:"WORKER_"`
}

type WorkerOptions struct {
	MaxConcurrentActivityExecutionSize     int  `env:"MAX_CONCURRENT_ACTIVITY" envDefault:"20"`
	MaxConcurrentWorkflowTaskExecutionSize int  `env:"MAX_CONCURRENT_WORKFLOW" envDefault:"10"`
	EnableLoggingInReplay                  bool `env:"ENABLE_LOGGING_REPLAY" envDefault:"false"`
}

type GitHubConfig struct {
	BaseURL        string          `env:"BASE_URL" envDefault:"https://api.github.com"`
	UploadURL      string          `env:"UPLOAD_URL" envDefault:"https://uploads.github.com"`
	Enterprise     bool            `env:"ENTERPRISE" envDefault:"false"`
	EnterpriseHost string          `env:"ENTERPRISE_HOST"`
	AppID          int64           `env:"APP_ID"`
	InstallationID int64           `env:"INSTALLATION_ID"`
	RateLimit      RateLimitConfig `envPrefix:"RATE_LIMIT_"`
}

type RateLimitConfig struct {
	MaxRetries        int           `env:"MAX_RETRIES" envDefault:"5"`
	InitialBackoff    time.Duration `env:"INITIAL_BACKOFF" envDefault:"1s"`
	MaxBackoff        time.Duration `env:"MAX_BACKOFF" envDefault:"60s"`
	BackoffMultiplier float64       `env:"BACKOFF_MULTIPLIER" envDefault:"2.0"`
}

type AppConfig struct {
	Environment    string `env:"ENVIRONMENT" envDefault:"development"`
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat      string `env:"LOG_FORMAT" envDefault:"json"`
	MetricsEnabled bool   `env:"METRICS_ENABLED" envDefault:"true"`
	MetricsPort    int    `env:"METRICS_PORT" envDefault:"9090"`
	HealthPort     int    `env:"HEALTH_PORT" envDefault:"8080"`
}

type SecretsConfig struct {
	GitHubPrivateKey []byte
}

// Load loads configuration from environment variables and files
func Load() (*Config, error) {
	// Load .env file if exists (for local development)
	if err := godotenv.Load(); err != nil {
		// Ignore error, use environment variables if no .env file
	}

	cfg := &Config{}
	
	// Parse environment variables using caarlos0/env
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}
	
	// Load secrets from files
	if err := loadSecrets(cfg); err != nil {
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}
	
	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Configure GitHub URLs for enterprise
	if cfg.GitHub.Enterprise && cfg.GitHub.EnterpriseHost != "" {
		cfg.GitHub.BaseURL = fmt.Sprintf("https://%s/api/v3", cfg.GitHub.EnterpriseHost)
		cfg.GitHub.UploadURL = fmt.Sprintf("https://%s/api/uploads", cfg.GitHub.EnterpriseHost)
	}
	
	return cfg, nil
}

// loadSecrets loads secrets from files
func loadSecrets(cfg *Config) error {
	// GitHub App private key
	privateKeyPath := secrets.GetSecretPath("GITHUB_PRIVATE_KEY_PATH", "/var/secrets/github/private-key.pem")
	
	privateKey, err := secrets.LoadFromFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load GitHub App private key: %w", err)
	}
	cfg.Secrets.GitHubPrivateKey = privateKey
	
	return nil
}

func validateConfig(cfg *Config) error {
	if cfg.GitHub.AppID == 0 {
		return fmt.Errorf("GitHub App ID is required")
	}
	if cfg.GitHub.InstallationID == 0 {
		return fmt.Errorf("GitHub Installation ID is required")
	}
	if len(cfg.Secrets.GitHubPrivateKey) == 0 {
		return fmt.Errorf("GitHub App private key is required")
	}
	return nil
}