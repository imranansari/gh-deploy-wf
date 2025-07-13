package main

import (
	"context"
	"log"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/imranansari/gh-deploy-wf/config"
	"github.com/imranansari/gh-deploy-wf/logging"
	"github.com/imranansari/gh-deploy-wf/workflows"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logging.InitLogger(cfg.App.LogLevel, cfg.App.LogFormat)
	logger := logging.GitHubLogger().With().Str("component", "test-client").Logger()

	logger.Info().Msg("Starting test client for GitHub deployment workflow")

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Temporal client")
	}
	defer temporalClient.Close()

	// Hardcoded test parameters
	workflowInput := workflows.DeploymentWorkflowInput{
		// GitHub Repository Information
		GithubOwner: "imranansari",
		GithubRepo:  "gh-deploy-test",
		CommitSHA:   "3be551363b02cf1d6151ce904bfbe424599c1156",

		// Deployment Configuration
		Environment: "pr-preview",
		Description: "Test deployment from Temporal workflow",
		IsTransient: true, // Preview environments are transient

		// External System Integration (simulated Harness)
		HarnessPipelineID:  "test-pipeline-123",
		HarnessExecutionID: "test-execution-456",

		// Deployment Metadata
		LogURL:         "https://ci.example.com/builds/test-123",
		EnvironmentURL: "https://pr-test.preview.example.com",

		// Custom payload
		Payload: map[string]string{
			"triggered_by": "test-client",
			"test_mode":    "true",
			"branch":       "main",
		},
	}

	logger.Info().
		Str("github_owner", workflowInput.GithubOwner).
		Str("github_repo", workflowInput.GithubRepo).
		Str("environment", workflowInput.Environment).
		Msg("Starting deployment workflow with test parameters")

	// Start workflow execution
	workflowOptions := client.StartWorkflowOptions{
		ID:        "test-deployment-" + time.Now().Format("20060102-150405"),
		TaskQueue: cfg.Temporal.TaskQueue,
	}

	workflowRun, err := temporalClient.ExecuteWorkflow(
		context.Background(),
		workflowOptions,
		workflows.GitHubDeploymentWorkflow,
		workflowInput,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start workflow")
	}

	logger.Info().
		Str("workflow_id", workflowRun.GetID()).
		Str("run_id", workflowRun.GetRunID()).
		Msg("Workflow started successfully")

	// Wait for workflow completion
	logger.Info().Msg("Waiting for workflow to complete...")

	var result workflows.DeploymentWorkflowResult
	err = workflowRun.Get(context.Background(), &result)
	if err != nil {
		logger.Error().Err(err).Msg("Workflow execution failed")
		return
	}

	// Display results
	logger.Info().
		Int64("deployment_id", result.DeploymentID).
		Str("final_status", result.FinalStatus).
		Str("environment", result.Environment).
		Str("duration", result.TotalDuration).
		Int("status_updates", result.StatusUpdates).
		Msg("Workflow completed successfully")

	if result.EnvironmentURL != "" {
		logger.Info().
			Str("environment_url", result.EnvironmentURL).
			Msg("Deployment environment is ready")
	}

	// Print URLs for easy access
	logger.Info().Msg("=== Test Results ===")
	logger.Info().Msgf("✓ Workflow ID: %s", workflowRun.GetID())
	logger.Info().Msgf("✓ Deployment ID: %d", result.DeploymentID)
	logger.Info().Msgf("✓ Final Status: %s", result.FinalStatus)
	logger.Info().Msgf("✓ View GitHub Deployment: https://github.com/%s/%s/deployments",
		workflowInput.GithubOwner, workflowInput.GithubRepo)
	logger.Info().Msgf("✓ View Temporal Workflow: http://localhost:8233/namespaces/%s/workflows/%s/%s",
		cfg.Temporal.Namespace, workflowRun.GetID(), workflowRun.GetRunID())

	if result.FinalStatus == "success" {
		logger.Info().Msg("🎉 Test deployment completed successfully!")
	} else {
		logger.Warn().Msgf("⚠️ Test deployment finished with status: %s", result.FinalStatus)
	}
}
