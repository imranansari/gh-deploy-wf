package config

import (
	"fmt"
	"os"
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
	// GitHub App ID (same for both GitHub.com and Enterprise)
	AppID          int64           `env:"APP_ID"`
	
	// Enterprise GitHub Configuration
	// Set GITHUB_ENTERPRISE_URL to use Enterprise GitHub
	// Leave empty to use GitHub.com (temporary - remove when switching to enterprise-only)
	EnterpriseURL  string          `env:"ENTERPRISE_URL"`
	
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
	
	return cfg, nil
}

// loadSecrets loads secrets from files
func loadSecrets(cfg *Config) error {
	// Get secrets base path
	secretsPath := getEnv("SECRETS_PATH", ".private")
	
	// GitHub App private key (streamcommander)
	privateKeyPath := fmt.Sprintf("%s/streamcommander.2025-07-12.private-key.pem", secretsPath)
	
	privateKey, err := secrets.LoadFromFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load GitHub App private key: %w", err)
	}
	cfg.Secrets.GitHubPrivateKey = privateKey
	
	return nil
}

// Helper function for simple environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func validateConfig(cfg *Config) error {
	if cfg.GitHub.AppID == 0 {
		return fmt.Errorf("GitHub App ID is required")
	}
	if len(cfg.Secrets.GitHubPrivateKey) == 0 {
		return fmt.Errorf("GitHub App private key is required")
	}
	return nil
}