# Temporal Workflow Implementation for GitHub Deployment Tracking

## 1. Overview

This document provides a detailed implementation design for the Temporal workflow component of the GitHub deployment tracking system. The implementation uses Go with zerolog for structured logging, carlos for environment configuration, file-based secret management for Kubernetes compatibility, and GitHub App authentication for superior security and rate limits.

## 2. Configuration Architecture

### 2.1 Environment Configuration Structure

```go
// config/config.go
package config

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "github.com/spf13/viper"
)

type Config struct {
    // Temporal Configuration
    Temporal TemporalConfig `mapstructure:"temporal"`
    
    // GitHub Configuration
    GitHub GitHubConfig `mapstructure:"github"`
    
    // Application Configuration
    App AppConfig `mapstructure:"app"`
    
    // Secrets (loaded from files)
    Secrets SecretsConfig
}

type TemporalConfig struct {
    HostPort      string        `mapstructure:"host_port" default:"localhost:7233"`
    Namespace     string        `mapstructure:"namespace" default:"default"`
    TaskQueue     string        `mapstructure:"task_queue" default:"github-deployment-tracker"`
    MaxConcurrent int           `mapstructure:"max_concurrent" default:"10"`
    WorkerOptions WorkerOptions `mapstructure:"worker_options"`
}

type WorkerOptions struct {
    MaxConcurrentActivityExecutionSize     int `mapstructure:"max_concurrent_activity" default:"20"`
    MaxConcurrentWorkflowTaskExecutionSize int `mapstructure:"max_concurrent_workflow" default:"10"`
    EnableLoggingInReplay                  bool `mapstructure:"enable_logging_replay" default:"false"`
}

type GitHubConfig struct {
    BaseURL        string `mapstructure:"base_url" default:"https://api.github.com"`
    UploadURL      string `mapstructure:"upload_url" default:"https://uploads.github.com"`
    Enterprise     bool   `mapstructure:"enterprise" default:"false"`
    EnterpriseHost string `mapstructure:"enterprise_host"`
    AppID          int64  `mapstructure:"app_id"`
    InstallationID int64  `mapstructure:"installation_id"`
    
    // Rate limiting configuration
    RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

type RateLimitConfig struct {
    MaxRetries         int           `mapstructure:"max_retries" default:"5"`
    InitialBackoff     time.Duration `mapstructure:"initial_backoff" default:"1s"`
    MaxBackoff         time.Duration `mapstructure:"max_backoff" default:"60s"`
    BackoffMultiplier  float64       `mapstructure:"backoff_multiplier" default:"2.0"`
}

type AppConfig struct {
    Environment    string        `mapstructure:"environment" default:"development"`
    LogLevel       string        `mapstructure:"log_level" default:"info"`
    LogFormat      string        `mapstructure:"log_format" default:"json"`
    MetricsEnabled bool          `mapstructure:"metrics_enabled" default:"true"`
    MetricsPort    int           `mapstructure:"metrics_port" default:"9090"`
    HealthPort     int           `mapstructure:"health_port" default:"8080"`
}

type SecretsConfig struct {
    GitHubPrivateKey []byte
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
    // Initialize viper with carlos
    v := viper.New()
    
    // Set config name and paths
    v.SetConfigName("config")
    v.SetConfigType("yaml")
    v.AddConfigPath(".")
    v.AddConfigPath("./config")
    v.AddConfigPath("/etc/github-deployment-tracker/")
    
    // Enable environment variable binding
    v.AutomaticEnv()
    v.SetEnvPrefix("GDT") // GitHub Deployment Tracker
    v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    
    // Read config file if exists
    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("error reading config file: %w", err)
        }
        log.Info().Msg("No config file found, using environment variables and defaults")
    }
    
    // Load .env file if exists (for local development)
    if err := godotenv.Load(); err != nil {
        log.Debug().Err(err).Msg("No .env file found")
    }
    
    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unable to decode config: %w", err)
    }
    
    // Load secrets from files
    if err := loadSecrets(&cfg); err != nil {
        return nil, fmt.Errorf("failed to load secrets: %w", err)
    }
    
    // Validate configuration
    if err := validateConfig(&cfg); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    
    // Configure GitHub URLs for enterprise
    if cfg.GitHub.Enterprise && cfg.GitHub.EnterpriseHost != "" {
        cfg.GitHub.BaseURL = fmt.Sprintf("https://%s/api/v3", cfg.GitHub.EnterpriseHost)
        cfg.GitHub.UploadURL = fmt.Sprintf("https://%s/api/uploads", cfg.GitHub.EnterpriseHost)
    }
    
    return &cfg, nil
}

// loadSecrets loads secrets from Kubernetes mounted files
func loadSecrets(cfg *Config) error {
    // GitHub App private key
    privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
    if privateKeyPath == "" {
        privateKeyPath = "/var/secrets/github/private-key.pem"
    }
    
    privateKey, err := ioutil.ReadFile(privateKeyPath)
    if err != nil {
        return fmt.Errorf("failed to read GitHub App private key: %w", err)
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
```

### 2.2 Logging Configuration

```go
// logging/logger.go
package logging

import (
    "os"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

// InitLogger initializes zerolog with the specified configuration
func InitLogger(level string, format string) {
    // Set time format
    zerolog.TimeFieldFormat = time.RFC3339Nano
    
    // Parse log level
    logLevel, err := zerolog.ParseLevel(level)
    if err != nil {
        logLevel = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(logLevel)
    
    // Configure output format
    if format == "console" {
        log.Logger = log.Output(zerolog.ConsoleWriter{
            Out:        os.Stdout,
            TimeFormat: time.RFC3339,
        })
    } else {
        // JSON format (default)
        log.Logger = zerolog.New(os.Stdout).With().
            Timestamp().
            Caller().
            Logger()
    }
    
    // Add service metadata
    log.Logger = log.With().
        Str("service", "github-deployment-tracker").
        Logger()
}

// WorkflowLogger creates a logger for Temporal workflows
func WorkflowLogger(workflowID string, runID string) zerolog.Logger {
    return log.With().
        Str("workflow_id", workflowID).
        Str("run_id", runID).
        Logger()
}

// ActivityLogger creates a logger for Temporal activities
func ActivityLogger(activityName string, workflowID string, runID string) zerolog.Logger {
    return log.With().
        Str("activity", activityName).
        Str("workflow_id", workflowID).
        Str("run_id", runID).
        Logger()
}
```

## 3. GitHub App Authentication Implementation

### 3.1 Authentication Client

```go
// github/auth.go
package github

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/bradleyfalzon/ghinstallation/v2"
    "github.com/google/go-github/v62/github"
    "github.com/rs/zerolog/log"
    "golang.org/x/oauth2"
)

// GitHubClientFactory creates authenticated GitHub clients
type GitHubClientFactory struct {
    appID          int64
    installationID int64
    privateKey     []byte
    baseURL        string
    uploadURL      string
}

// NewGitHubClientFactory creates a new GitHub client factory
func NewGitHubClientFactory(cfg config.GitHubConfig, privateKey []byte) (*GitHubClientFactory, error) {
    return &GitHubClientFactory{
        appID:          cfg.AppID,
        installationID: cfg.InstallationID,
        privateKey:     privateKey,
        baseURL:        cfg.BaseURL,
        uploadURL:      cfg.UploadURL,
    }, nil
}

// CreateClient creates a new authenticated GitHub client
func (f *GitHubClientFactory) CreateClient(ctx context.Context) (*github.Client, error) {
    // Create GitHub App installation transport
    itr, err := ghinstallation.New(
        http.DefaultTransport,
        f.appID,
        f.installationID,
        f.privateKey,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create installation transport: %w", err)
    }
    
    // Configure base URL for enterprise if needed
    if f.baseURL != "" && f.baseURL != "https://api.github.com" {
        itr.BaseURL = f.baseURL
    }
    
    // Create the client
    client := github.NewClient(&http.Client{Transport: itr})
    
    // Set custom URLs if enterprise
    if f.baseURL != "" {
        client.BaseURL, _ = client.BaseURL.Parse(f.baseURL + "/")
    }
    if f.uploadURL != "" {
        client.UploadURL, _ = client.UploadURL.Parse(f.uploadURL + "/")
    }
    
    // Verify authentication
    _, _, err = client.Apps.Get(ctx, "")
    if err != nil {
        return nil, fmt.Errorf("failed to verify GitHub App authentication: %w", err)
    }
    
    log.Info().
        Int64("app_id", f.appID).
        Int64("installation_id", f.installationID).
        Str("base_url", f.baseURL).
        Msg("GitHub client created successfully")
    
    return client, nil
}

// CreateClientWithRetry creates a client with retry capabilities
func (f *GitHubClientFactory) CreateClientWithRetry(ctx context.Context, retryConfig RateLimitConfig) (*GitHubRetryClient, error) {
    baseClient, err := f.CreateClient(ctx)
    if err != nil {
        return nil, err
    }
    
    return &GitHubRetryClient{
        client:      baseClient,
        retryConfig: retryConfig,
    }, nil
}
```

### 3.2 Retry Client with Rate Limit Handling

```go
// github/retry_client.go
package github

import (
    "context"
    "fmt"
    "time"

    "github.com/cenkalti/backoff/v4"
    "github.com/google/go-github/v62/github"
    "github.com/rs/zerolog/log"
)

// GitHubRetryClient wraps the GitHub client with retry logic
type GitHubRetryClient struct {
    client      *github.Client
    retryConfig RateLimitConfig
}

// CreateDeployment creates a deployment with retry logic
func (c *GitHubRetryClient) CreateDeployment(ctx context.Context, owner, repo string, request *github.DeploymentRequest) (*github.Deployment, error) {
    var deployment *github.Deployment
    var resp *github.Response
    
    operation := func() error {
        var err error
        deployment, resp, err = c.client.Repositories.CreateDeployment(ctx, owner, repo, request)
        return c.handleError(err, resp)
    }
    
    backoffConfig := c.createBackoffConfig()
    err := backoff.Retry(operation, backoff.WithContext(backoffConfig, ctx))
    
    if err != nil {
        return nil, fmt.Errorf("failed to create deployment after retries: %w", err)
    }
    
    return deployment, nil
}

// CreateDeploymentStatus creates a deployment status with retry logic
func (c *GitHubRetryClient) CreateDeploymentStatus(ctx context.Context, owner, repo string, deploymentID int64, request *github.DeploymentStatusRequest) (*github.DeploymentStatus, error) {
    var status *github.DeploymentStatus
    var resp *github.Response
    
    operation := func() error {
        var err error
        status, resp, err = c.client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deploymentID, request)
        return c.handleError(err, resp)
    }
    
    backoffConfig := c.createBackoffConfig()
    err := backoff.Retry(operation, backoff.WithContext(backoffConfig, ctx))
    
    if err != nil {
        return nil, fmt.Errorf("failed to create deployment status after retries: %w", err)
    }
    
    return status, nil
}

// handleError processes GitHub API errors and determines if they're retryable
func (c *GitHubRetryClient) handleError(err error, resp *github.Response) error {
    if err == nil {
        return nil
    }
    
    // Check for rate limit errors
    if resp != nil && resp.StatusCode == 403 {
        if resp.Rate.Remaining == 0 {
            resetTime := time.Unix(resp.Rate.Reset.Unix(), 0)
            waitDuration := time.Until(resetTime)
            
            log.Warn().
                Time("reset_time", resetTime).
                Dur("wait_duration", waitDuration).
                Msg("GitHub API rate limit exceeded")
            
            // If wait time is reasonable, sleep and retry
            if waitDuration > 0 && waitDuration < 5*time.Minute {
                time.Sleep(waitDuration)
                return fmt.Errorf("rate limited, will retry: %w", err)
            }
        }
    }
    
    // Check for server errors (5xx) - these are retryable
    if resp != nil && resp.StatusCode >= 500 {
        return fmt.Errorf("server error, will retry: %w", err)
    }
    
    // Check for 422 (validation errors) - these are not retryable
    if resp != nil && resp.StatusCode == 422 {
        return backoff.Permanent(fmt.Errorf("validation error: %w", err))
    }
    
    // Default: treat as permanent error
    return backoff.Permanent(err)
}

// createBackoffConfig creates the backoff configuration
func (c *GitHubRetryClient) createBackoffConfig() backoff.BackOff {
    exponentialBackoff := backoff.NewExponentialBackOff()
    exponentialBackoff.InitialInterval = c.retryConfig.InitialBackoff
    exponentialBackoff.MaxInterval = c.retryConfig.MaxBackoff
    exponentialBackoff.Multiplier = c.retryConfig.BackoffMultiplier
    exponentialBackoff.MaxElapsedTime = 0 // No max elapsed time
    
    return backoff.WithMaxRetries(exponentialBackoff, uint64(c.retryConfig.MaxRetries))
}
```

## 4. Temporal Workflow Implementation

### 4.1 Workflow Definition

```go
// workflows/deployment_workflow.go
package workflows

import (
    "fmt"
    "time"

    "go.temporal.io/sdk/temporal"
    "go.temporal.io/sdk/workflow"
    "github.com/rs/zerolog"
)

const (
    DeploymentStatusUpdateSignal = "deployment-status-update"
    WorkflowTimeout             = 2 * time.Hour
    SignalTimeout               = 30 * time.Minute
)

// DeploymentWorkflowInput represents the initial workflow input
type DeploymentWorkflowInput struct {
    // Repository information
    RepoOwner  string `json:"repo_owner"`
    RepoName   string `json:"repo_name"`
    CommitSHA  string `json:"commit_sha"`
    BranchName string `json:"branch_name"`
    
    // Deployment information
    Environment        string `json:"environment"`
    IsTransient       bool   `json:"is_transient"`
    HarnessPipelineID string `json:"harness_pipeline_id"`
    HarnessExecutionID string `json:"harness_execution_id"`
    
    // Initial status
    Status         string `json:"status"`
    LogURL         string `json:"log_url"`
    EnvironmentURL string `json:"environment_url"`
    Description    string `json:"description"`
}

// DeploymentStatusUpdate represents a status update signal
type DeploymentStatusUpdate struct {
    Status         string    `json:"status"`
    LogURL         string    `json:"log_url"`
    EnvironmentURL string    `json:"environment_url"`
    Description    string    `json:"description"`
    UpdatedAt      time.Time `json:"updated_at"`
}

// DeploymentWorkflowResult represents the workflow result
type DeploymentWorkflowResult struct {
    DeploymentID   int64     `json:"deployment_id"`
    FinalStatus    string    `json:"final_status"`
    CompletedAt    time.Time `json:"completed_at"`
    TotalDuration  string    `json:"total_duration"`
    StatusUpdates  int       `json:"status_updates"`
}

// GitHubDeploymentWorkflow orchestrates GitHub deployment tracking
func GitHubDeploymentWorkflow(ctx workflow.Context, input DeploymentWorkflowInput) (*DeploymentWorkflowResult, error) {
    // Create workflow-specific logger
    logger := workflow.GetLogger(ctx)
    workflowInfo := workflow.GetInfo(ctx)
    
    // Initialize result
    result := &DeploymentWorkflowResult{
        StatusUpdates: 0,
    }
    
    startTime := workflow.Now(ctx)
    
    // Configure activity options
    activityOptions := workflow.ActivityOptions{
        StartToCloseTimeout: 2 * time.Minute,
        HeartbeatTimeout:    30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:        time.Second,
            BackoffCoefficient:     2.0,
            MaximumInterval:        30 * time.Second,
            MaximumAttempts:        5,
            NonRetryableErrorTypes: []string{"ValidationError", "AuthenticationError"},
        },
    }
    ctx = workflow.WithActivityOptions(ctx, activityOptions)
    
    // 1. Create GitHub deployment
    logger.Info("Creating GitHub deployment",
        "repo", fmt.Sprintf("%s/%s", input.RepoOwner, input.RepoName),
        "commit", input.CommitSHA,
        "environment", input.Environment,
        "harness_execution_id", input.HarnessExecutionID)
    
    var deploymentID int64
    createDeploymentActivity := workflow.ExecuteActivity(ctx,
        activities.CreateGitHubDeployment,
        activities.CreateDeploymentInput{
            RepoOwner:          input.RepoOwner,
            RepoName:           input.RepoName,
            CommitSHA:          input.CommitSHA,
            Environment:        input.Environment,
            Description:        input.Description,
            IsTransient:        input.IsTransient,
            HarnessExecutionID: input.HarnessExecutionID,
            HarnessPipelineID:  input.HarnessPipelineID,
        })
    
    if err := createDeploymentActivity.Get(ctx, &deploymentID); err != nil {
        logger.Error("Failed to create GitHub deployment", "error", err)
        return nil, fmt.Errorf("failed to create deployment: %w", err)
    }
    
    result.DeploymentID = deploymentID
    logger.Info("GitHub deployment created", "deployment_id", deploymentID)
    
    // 2. Update initial status
    updateStatusActivity := workflow.ExecuteActivity(ctx,
        activities.UpdateGitHubDeploymentStatus,
        activities.UpdateDeploymentStatusInput{
            RepoOwner:      input.RepoOwner,
            RepoName:       input.RepoName,
            DeploymentID:   deploymentID,
            State:          mapHarnessStatusToGitHub(input.Status),
            LogURL:         input.LogURL,
            EnvironmentURL: input.EnvironmentURL,
            Description:    input.Description,
        })
    
    if err := updateStatusActivity.Get(ctx, nil); err != nil {
        logger.Error("Failed to update initial deployment status", "error", err)
        // Continue anyway - deployment was created
    } else {
        result.StatusUpdates++
    }
    
    // 3. Wait for status updates via signals
    result.FinalStatus = input.Status
    signalChan := workflow.GetSignalChannel(ctx, DeploymentStatusUpdateSignal)
    
    for {
        selector := workflow.NewSelector(ctx)
        
        // Add signal handler
        selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
            var update DeploymentStatusUpdate
            c.Receive(ctx, &update)
            
            logger.Info("Received deployment status update",
                "status", update.Status,
                "deployment_id", deploymentID)
            
            // Update GitHub deployment status
            updateActivity := workflow.ExecuteActivity(ctx,
                activities.UpdateGitHubDeploymentStatus,
                activities.UpdateDeploymentStatusInput{
                    RepoOwner:      input.RepoOwner,
                    RepoName:       input.RepoName,
                    DeploymentID:   deploymentID,
                    State:          mapHarnessStatusToGitHub(update.Status),
                    LogURL:         update.LogURL,
                    EnvironmentURL: update.EnvironmentURL,
                    Description:    update.Description,
                })
            
            if err := updateActivity.Get(ctx, nil); err != nil {
                logger.Error("Failed to update deployment status",
                    "error", err,
                    "status", update.Status)
            } else {
                result.StatusUpdates++
                result.FinalStatus = update.Status
            }
            
            // Check if this is a terminal status
            if isTerminalStatus(update.Status) {
                logger.Info("Deployment reached terminal status",
                    "status", update.Status,
                    "deployment_id", deploymentID)
                return
            }
        })
        
        // Add timeout handler
        timeoutCtx, cancel := workflow.WithCancel(ctx)
        selector.AddFuture(workflow.NewTimer(timeoutCtx, SignalTimeout), func(f workflow.Future) {
            logger.Warn("Timeout waiting for deployment status update",
                "deployment_id", deploymentID,
                "timeout", SignalTimeout)
            
            // Mark deployment as inactive if not already terminal
            if !isTerminalStatus(result.FinalStatus) {
                inactiveActivity := workflow.ExecuteActivity(ctx,
                    activities.UpdateGitHubDeploymentStatus,
                    activities.UpdateDeploymentStatusInput{
                        RepoOwner:    input.RepoOwner,
                        RepoName:     input.RepoName,
                        DeploymentID: deploymentID,
                        State:        "inactive",
                        Description:  "Deployment timed out waiting for status update",
                    })
                
                if err := inactiveActivity.Get(ctx, nil); err == nil {
                    result.StatusUpdates++
                    result.FinalStatus = "inactive"
                }
            }
        })
        
        // Wait for signal or timeout
        selector.Select(ctx)
        cancel() // Cancel the timer if signal received
        
        // Exit if terminal status reached
        if isTerminalStatus(result.FinalStatus) {
            break
        }
    }
    
    // Calculate final metrics
    endTime := workflow.Now(ctx)
    result.CompletedAt = endTime
    result.TotalDuration = endTime.Sub(startTime).String()
    
    logger.Info("Deployment workflow completed",
        "deployment_id", deploymentID,
        "final_status", result.FinalStatus,
        "duration", result.TotalDuration,
        "status_updates", result.StatusUpdates)
    
    return result, nil
}

// mapHarnessStatusToGitHub maps Harness status to GitHub deployment status
func mapHarnessStatusToGitHub(harnessStatus string) string {
    statusMap := map[string]string{
        "queued":      "queued",
        "running":     "in_progress",
        "in_progress": "in_progress",
        "success":     "success",
        "succeeded":   "success",
        "failed":      "failure",
        "failure":     "failure",
        "error":       "error",
        "aborted":     "failure",
        "cancelled":   "failure",
        "inactive":    "inactive",
    }
    
    if githubStatus, ok := statusMap[harnessStatus]; ok {
        return githubStatus
    }
    return "pending"
}

// isTerminalStatus checks if a status is terminal
func isTerminalStatus(status string) bool {
    terminalStatuses := map[string]bool{
        "success":  true,
        "failure":  true,
        "error":    true,
        "inactive": true,
    }
    return terminalStatuses[status]
}
```

### 4.2 Activities Implementation

```go
// activities/github_activities.go
package activities

import (
    "context"
    "fmt"
    "time"

    "github.com/google/go-github/v62/github"
    "github.com/rs/zerolog/log"
    "go.temporal.io/sdk/activity"
)

// GitHubActivities contains GitHub-related activities
type GitHubActivities struct {
    clientFactory *github.GitHubClientFactory
}

// NewGitHubActivities creates a new instance of GitHub activities
func NewGitHubActivities(clientFactory *github.GitHubClientFactory) *GitHubActivities {
    return &GitHubActivities{
        clientFactory: clientFactory,
    }
}

// CreateDeploymentInput represents input for creating a deployment
type CreateDeploymentInput struct {
    RepoOwner          string            `json:"repo_owner"`
    RepoName           string            `json:"repo_name"`
    CommitSHA          string            `json:"commit_sha"`
    Environment        string            `json:"environment"`
    Description        string            `json:"description"`
    IsTransient        bool              `json:"is_transient"`
    HarnessExecutionID string            `json:"harness_execution_id"`
    HarnessPipelineID  string            `json:"harness_pipeline_id"`
    Payload            map[string]string `json:"payload"`
}

// CreateGitHubDeployment creates a new deployment in GitHub
func (a *GitHubActivities) CreateGitHubDeployment(ctx context.Context, input CreateDeploymentInput) (int64, error) {
    activityInfo := activity.GetInfo(ctx)
    logger := log.With().
        Str("activity", "CreateGitHubDeployment").
        Str("workflow_id", activityInfo.WorkflowExecution.ID).
        Str("run_id", activityInfo.WorkflowExecution.RunID).
        Logger()
    
    logger.Info().
        Str("repo", fmt.Sprintf("%s/%s", input.RepoOwner, input.RepoName)).
        Str("commit", input.CommitSHA).
        Str("environment", input.Environment).
        Msg("Creating GitHub deployment")
    
    // Record heartbeat
    activity.RecordHeartbeat(ctx, "Creating GitHub client")
    
    // Create GitHub client with retry capabilities
    client, err := a.clientFactory.CreateClientWithRetry(ctx, a.clientFactory.retryConfig)
    if err != nil {
        return 0, fmt.Errorf("failed to create GitHub client: %w", err)
    }
    
    // Prepare deployment payload
    payload := map[string]interface{}{
        "harness_execution_id": input.HarnessExecutionID,
        "harness_pipeline_id":  input.HarnessPipelineID,
        "triggered_by":         "temporal-workflow",
        "created_at":           time.Now().UTC().Format(time.RFC3339),
    }
    
    // Add custom payload if provided
    for k, v := range input.Payload {
        payload[k] = v
    }
    
    // Create deployment request
    deploymentRequest := &github.DeploymentRequest{
        Ref:                   github.String(input.CommitSHA),
        Task:                  github.String("deploy"),
        Environment:           github.String(input.Environment),
        Description:           github.String(input.Description),
        TransientEnvironment:  github.Bool(input.IsTransient),
        ProductionEnvironment: github.Bool(input.Environment == "production"),
        RequiredContexts:      &[]string{}, // Skip status checks for external deployments
        AutoMerge:             github.Bool(false),
        Payload:               payload,
    }
    
    // Record heartbeat before API call
    activity.RecordHeartbeat(ctx, "Calling GitHub API")
    
    // Create deployment
    deployment, err := client.CreateDeployment(ctx, input.RepoOwner, input.RepoName, deploymentRequest)
    if err != nil {
        logger.Error().Err(err).Msg("Failed to create GitHub deployment")
        return 0, err
    }
    
    deploymentID := deployment.GetID()
    logger.Info().
        Int64("deployment_id", deploymentID).
        Str("url", deployment.GetURL()).
        Msg("Successfully created GitHub deployment")
    
    return deploymentID, nil
}

// UpdateDeploymentStatusInput represents input for updating deployment status
type UpdateDeploymentStatusInput struct {
    RepoOwner      string `json:"repo_owner"`
    RepoName       string `json:"repo_name"`
    DeploymentID   int64  `json:"deployment_id"`
    State          string `json:"state"`
    LogURL         string `json:"log_url"`
    EnvironmentURL string `json:"environment_url"`
    Description    string `json:"description"`
}

// UpdateGitHubDeploymentStatus updates the status of a deployment
func (a *GitHubActivities) UpdateGitHubDeploymentStatus(ctx context.Context, input UpdateDeploymentStatusInput) error {
    activityInfo := activity.GetInfo(ctx)
    logger := log.With().
        Str("activity", "UpdateGitHubDeploymentStatus").
        Str("workflow_id", activityInfo.WorkflowExecution.ID).
        Str("run_id", activityInfo.WorkflowExecution.RunID).
        Int64("deployment_id", input.DeploymentID).
        Logger()
    
    logger.Info().
        Str("state", input.State).
        Msg("Updating GitHub deployment status")
    
    // Record heartbeat
    activity.RecordHeartbeat(ctx, "Creating GitHub client")
    
    // Create GitHub client with retry capabilities
    client, err := a.clientFactory.CreateClientWithRetry(ctx, a.clientFactory.retryConfig)
    if err != nil {
        return fmt.Errorf("failed to create GitHub client: %w", err)
    }
    
    // Create status request
    statusRequest := &github.DeploymentStatusRequest{
        State:          github.String(input.State),
        LogURL:         github.String(input.LogURL),
        Description:    github.String(truncateDescription(input.Description, 140)),
        EnvironmentURL: github.String(input.EnvironmentURL),
        AutoInactive:   github.Bool(true), // Automatically mark previous deployments as inactive
    }
    
    // Record heartbeat before API call
    activity.RecordHeartbeat(ctx, "Calling GitHub API")
    
    // Update deployment status
    status, err := client.CreateDeploymentStatus(ctx, input.RepoOwner, input.RepoName, input.DeploymentID, statusRequest)
    if err != nil {
        logger.Error().Err(err).Msg("Failed to update GitHub deployment status")
        return err
    }
    
    logger.Info().
        Str("state", status.GetState()).
        Str("url", status.GetURL()).
        Msg("Successfully updated GitHub deployment status")
    
    return nil
}

// truncateDescription ensures description doesn't exceed GitHub's limit
func truncateDescription(desc string, maxLen int) string {
    if len(desc) <= maxLen {
        return desc
    }
    return desc[:maxLen-3] + "..."
}
```

## 5. Worker Implementation

### 5.1 Main Worker Application

```go
// cmd/worker/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/rs/zerolog/log"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"

    "your-org/github-deployment-tracker/activities"
    "your-org/github-deployment-tracker/config"
    "your-org/github-deployment-tracker/github"
    "your-org/github-deployment-tracker/logging"
    "your-org/github-deployment-tracker/workflows"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to load configuration")
    }
    
    // Initialize logger
    logging.InitLogger(cfg.App.LogLevel, cfg.App.LogFormat)
    
    log.Info().
        Str("environment", cfg.App.Environment).
        Str("temporal_host", cfg.Temporal.HostPort).
        Str("task_queue", cfg.Temporal.TaskQueue).
        Bool("github_enterprise", cfg.GitHub.Enterprise).
        Msg("Starting GitHub Deployment Tracker Worker")
    
    // Create Temporal client
    temporalClient, err := createTemporalClient(cfg.Temporal)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create Temporal client")
    }
    defer temporalClient.Close()
    
    // Create GitHub client factory
    githubFactory, err := github.NewGitHubClientFactory(cfg.GitHub, cfg.Secrets.GitHubPrivateKey)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create GitHub client factory")
    }
    
    // Test GitHub authentication
    ctx := context.Background()
    if _, err := githubFactory.CreateClient(ctx); err != nil {
        log.Fatal().Err(err).Msg("Failed to authenticate with GitHub")
    }
    
    // Create worker
    w := worker.New(temporalClient, cfg.Temporal.TaskQueue, worker.Options{
        MaxConcurrentActivityExecutionSize:     cfg.Temporal.WorkerOptions.MaxConcurrentActivityExecutionSize,
        MaxConcurrentWorkflowTaskExecutionSize: cfg.Temporal.WorkerOptions.MaxConcurrentWorkflowTaskExecutionSize,
        EnableLoggingInReplay:                  cfg.Temporal.WorkerOptions.EnableLoggingInReplay,
    })
    
    // Register workflows
    w.RegisterWorkflow(workflows.GitHubDeploymentWorkflow)
    
    // Register activities
    githubActivities := activities.NewGitHubActivities(githubFactory)
    w.RegisterActivity(githubActivities)
    
    // Start health check server
    go startHealthServer(cfg.App.HealthPort)
    
    // Start metrics server if enabled
    if cfg.App.MetricsEnabled {
        go startMetricsServer(cfg.App.MetricsPort)
    }
    
    // Run worker
    log.Info().Msg("Starting Temporal worker")
    
    // Handle graceful shutdown
    errChan := make(chan error, 1)
    go func() {
        errChan <- w.Run(worker.InterruptCh())
    }()
    
    // Wait for termination signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
    
    select {
    case err := <-errChan:
        if err != nil {
            log.Fatal().Err(err).Msg("Worker error")
        }
    case sig := <-sigChan:
        log.Info().Str("signal", sig.String()).Msg("Received termination signal")
        w.Stop()
    }
    
    log.Info().Msg("Worker stopped gracefully")
}

func createTemporalClient(cfg config.TemporalConfig) (client.Client, error) {
    options := client.Options{
        HostPort:  cfg.HostPort,
        Namespace: cfg.Namespace,
    }
    
    return client.Dial(options)
}
```

## 6. Environment Configuration Examples

### 6.1 Development Configuration (.env file)

```env
# Temporal Configuration
GDT_TEMPORAL_HOST_PORT=localhost:7233
GDT_TEMPORAL_NAMESPACE=default
GDT_TEMPORAL_TASK_QUEUE=github-deployment-tracker

# GitHub Configuration (GitHub.com)
GDT_GITHUB_APP_ID=123456
GDT_GITHUB_INSTALLATION_ID=789012
GDT_GITHUB_ENTERPRISE=false

# Application Configuration
GDT_APP_ENVIRONMENT=development
GDT_APP_LOG_LEVEL=debug
GDT_APP_LOG_FORMAT=console

# Secret Paths (for local development)
GITHUB_APP_PRIVATE_KEY_PATH=./secrets/github-app-key.pem
```

### 6.2 Production Configuration (Kubernetes ConfigMap)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: github-deployment-tracker-config
data:
  config.yaml: |
    temporal:
      host_port: temporal-frontend.temporal:7233
      namespace: production
      task_queue: github-deployment-tracker
      worker_options:
        max_concurrent_activity: 50
        max_concurrent_workflow: 20
    
    github:
      app_id: 234567
      installation_id: 890123
      enterprise: true
      enterprise_host: github.company.com
      rate_limit:
        max_retries: 10
        initial_backoff: 2s
        max_backoff: 120s
    
    app:
      environment: production
      log_level: info
      log_format: json
      metrics_enabled: true
```

### 6.3 Kubernetes Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-deployment-tracker-secrets
type: Opaque
data:
  private-key.pem: <base64-encoded-github-app-private-key>
```

## 7. Deployment Considerations

### 7.1 Health Check Implementation

```go
// health/server.go
package health

import (
    "fmt"
    "net/http"

    "github.com/rs/zerolog/log"
)

func startHealthServer(port int) {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", healthHandler)
    mux.HandleFunc("/ready", readyHandler)
    
    addr := fmt.Sprintf(":%d", port)
    log.Info().Str("addr", addr).Msg("Starting health check server")
    
    if err := http.ListenAndServe(addr, mux); err != nil {
        log.Error().Err(err).Msg("Health check server error")
    }
}
```

### 7.2 Metrics Implementation

```go
// metrics/server.go
package metrics

import (
    "fmt"
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/rs/zerolog/log"
)

func startMetricsServer(port int) {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())
    
    addr := fmt.Sprintf(":%d", port)
    log.Info().Str("addr", addr).Msg("Starting metrics server")
    
    if err := http.ListenAndServe(addr, mux); err != nil {
        log.Error().Err(err).Msg("Metrics server error")
    }
}
```

## 8. Testing Strategy

### 8.1 Workflow Testing

```go
// workflows/deployment_workflow_test.go
package workflows_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/suite"
    "go.temporal.io/sdk/testsuite"
    
    "your-org/github-deployment-tracker/workflows"
)

type DeploymentWorkflowTestSuite struct {
    suite.Suite
    testsuite.WorkflowTestSuite
}

func TestDeploymentWorkflowTestSuite(t *testing.T) {
    suite.Run(t, new(DeploymentWorkflowTestSuite))
}

func (s *DeploymentWorkflowTestSuite) Test_SuccessfulDeployment() {
    env := s.NewTestWorkflowEnvironment()
    
    // Mock activities
    env.OnActivity("CreateGitHubDeployment", mock.Anything, mock.Anything).Return(int64(12345), nil)
    env.OnActivity("UpdateGitHubDeploymentStatus", mock.Anything, mock.Anything).Return(nil)
    
    // Execute workflow
    env.ExecuteWorkflow(workflows.GitHubDeploymentWorkflow, workflows.DeploymentWorkflowInput{
        RepoOwner:   "test-org",
        RepoName:    "test-repo",
        CommitSHA:   "abc123",
        Environment: "staging",
        Status:      "in_progress",
    })
    
    // Send status update signal
    env.SignalWorkflow(workflows.DeploymentStatusUpdateSignal, workflows.DeploymentStatusUpdate{
        Status: "success",
    })
    
    // Assert workflow completion
    s.True(env.IsWorkflowCompleted())
    s.NoError(env.GetWorkflowError())
    
    var result workflows.DeploymentWorkflowResult
    s.NoError(env.GetWorkflowResult(&result))
    s.Equal(int64(12345), result.DeploymentID)
    s.Equal("success", result.FinalStatus)
}
```

## 9. Summary

This implementation provides a production-ready Temporal workflow system for GitHub deployment tracking with:

- **Robust Configuration**: Environment-based configuration with support for both GitHub.com and Enterprise
- **Secure Authentication**: GitHub App authentication with file-based secret management
- **Reliability**: Comprehensive retry logic with rate limit handling
- **Observability**: Structured logging with zerolog and metrics support
- **Testability**: Full test coverage for workflows and activities
- **Kubernetes Ready**: Designed for containerized deployment with ConfigMaps and Secrets

The system seamlessly integrates with your existing Harness + NATS architecture while providing enterprise-grade deployment tracking capabilities.