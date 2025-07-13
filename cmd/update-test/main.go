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
	logger := logging.GitHubLogger().With().Str("component", "update-test").Logger()

	logger.Info().Msg("Starting test for deployment update workflow")

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Temporal client")
	}
	defer temporalClient.Close()

	// Test scenarios for different deployment states
	testScenarios := []struct {
		name        string
		state       string
		description string
		logURL      string
		envURL      string
	}{
		{
			name:        "Build Started",
			state:       "pending",
			description: "CI build started for PR deployment",
			logURL:      "https://ci.harness.io/builds/test-123",
		},
		{
			name:        "Deployment In Progress",
			state:       "in_progress",
			description: "Deploying application to pr-preview environment",
			logURL:      "https://ci.harness.io/builds/test-123",
		},
		{
			name:        "Deployment Success",
			state:       "success",
			description: "Successfully deployed to pr-preview environment",
			logURL:      "https://ci.harness.io/builds/test-123",
			envURL:      "https://pr-test.preview.example.com",
		},
	}

	// Base deployment information (same deployment, different states)
	baseInput := workflows.DeploymentUpdateInput{
		GithubOwner: "imranansari",
		GithubRepo:  "gh-deploy-test",
		CommitSHA:   "3be551363b02cf1d6151ce904bfbe424599c1156", // Use existing commit
		Environment: config.EnvironmentPRPreview,
	}

	// Run each test scenario
	for i, scenario := range testScenarios {
		logger.Info().
			Str("scenario", scenario.name).
			Str("state", scenario.state).
			Msg("Running test scenario")

		// Prepare workflow input
		workflowInput := baseInput
		workflowInput.State = scenario.state
		workflowInput.Description = scenario.description
		workflowInput.LogURL = scenario.logURL
		workflowInput.EnvironmentURL = scenario.envURL

		// Start workflow execution
		workflowOptions := client.StartWorkflowOptions{
			ID:        "test-update-" + scenario.state + "-" + time.Now().Format("20060102-150405"),
			TaskQueue: cfg.Temporal.TaskQueue,
		}

		workflowRun, err := temporalClient.ExecuteWorkflow(
			context.Background(),
			workflowOptions,
			workflows.UpdateDeploymentWorkflow,
			workflowInput,
		)
		if err != nil {
			logger.Error().
				Err(err).
				Str("scenario", scenario.name).
				Msg("Failed to start update workflow")
			continue
		}

		logger.Info().
			Str("workflow_id", workflowRun.GetID()).
			Str("run_id", workflowRun.GetRunID()).
			Str("scenario", scenario.name).
			Msg("Update workflow started")

		// Wait for workflow completion
		err = workflowRun.Get(context.Background(), nil)
		if err != nil {
			logger.Error().
				Err(err).
				Str("scenario", scenario.name).
				Msg("Update workflow failed")
		} else {
			logger.Info().
				Str("scenario", scenario.name).
				Str("state", scenario.state).
				Msg("✓ Update workflow completed successfully")
		}

		// Small delay between scenarios to avoid rate limiting
		if i < len(testScenarios)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	logger.Info().Msg("=== Update Test Results ===")
	logger.Info().Msgf("✓ Tested %d deployment state updates", len(testScenarios))
	logger.Info().Msgf("✓ View GitHub Deployments: https://github.com/%s/%s/deployments",
		baseInput.GithubOwner, baseInput.GithubRepo)
	logger.Info().Msgf("✓ View Temporal Workflows: http://localhost:8233/namespaces/%s/workflows",
		cfg.Temporal.Namespace)

	logger.Info().Msg("🎉 Deployment update tests completed!")
}
