package main

import (
	"os"
	"os/signal"
	"syscall"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/imranansari/gh-deploy-wf/activities"
	"github.com/imranansari/gh-deploy-wf/config"
	githubClient "github.com/imranansari/gh-deploy-wf/github"
	"github.com/imranansari/gh-deploy-wf/logging"
	"github.com/imranansari/gh-deploy-wf/workflows"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}
	
	// Initialize logger
	logging.InitLogger(cfg.App.LogLevel, cfg.App.LogFormat)
	logger := logging.GitHubLogger().With().Str("component", "worker").Logger()
	
	logger.Info().
		Str("environment", cfg.App.Environment).
		Str("temporal_host", cfg.Temporal.HostPort).
		Str("task_queue", cfg.Temporal.TaskQueue).
		Str("github_enterprise_url", cfg.GitHub.EnterpriseURL).
		Bool("using_enterprise", cfg.GitHub.EnterpriseURL != "").
		Msg("Starting GitHub Deployment Tracker Worker")
	
	// Create Temporal client
	temporalClient, err := createTemporalClient(cfg.Temporal)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Temporal client")
	}
	defer temporalClient.Close()
	
	// Create GitHub client factory
	githubFactory := githubClient.NewClientFactory(cfg.GitHub, cfg.Secrets.GitHubPrivateKey, logger)
	
	// Test GitHub authentication - we'll test with a known org during first workflow execution
	logger.Info().Msg("GitHub App authentication configured - installation IDs will be resolved dynamically per organization")
	
	// Create worker
	w := worker.New(temporalClient, cfg.Temporal.TaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:     cfg.Temporal.WorkerOptions.MaxConcurrentActivityExecutionSize,
		MaxConcurrentWorkflowTaskExecutionSize: cfg.Temporal.WorkerOptions.MaxConcurrentWorkflowTaskExecutionSize,
		EnableLoggingInReplay:                  cfg.Temporal.WorkerOptions.EnableLoggingInReplay,
	})
	
	// Register workflows
	w.RegisterWorkflow(workflows.GitHubDeploymentWorkflow)
	w.RegisterWorkflow(workflows.UpdateDeploymentWorkflow)
	
	// Register activities
	githubActivities := activities.NewGitHubActivities(githubFactory)
	w.RegisterActivity(githubActivities.CreateGitHubDeployment)
	w.RegisterActivity(githubActivities.UpdateGitHubDeploymentStatus)
	w.RegisterActivity(githubActivities.FindGitHubDeployment)
	
	// Run worker
	logger.Info().Msg("Starting Temporal worker")
	
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
			logger.Fatal().Err(err).Msg("Worker error")
		}
	case sig := <-sigChan:
		logger.Info().Str("signal", sig.String()).Msg("Received termination signal")
		w.Stop()
	}
	
	logger.Info().Msg("Worker stopped gracefully")
}

func createTemporalClient(cfg config.TemporalConfig) (client.Client, error) {
	options := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	}
	
	return client.Dial(options)
}