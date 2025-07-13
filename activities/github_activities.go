package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v58/github"
	"go.temporal.io/sdk/activity"

	githubClient "github.com/imranansari/gh-deploy-wf/github"
	"github.com/imranansari/gh-deploy-wf/logging"
)

// GitHubActivities contains GitHub-related activities
type GitHubActivities struct {
	clientFactory *githubClient.ClientFactory
}

// NewGitHubActivities creates a new instance of GitHub activities
func NewGitHubActivities(clientFactory *githubClient.ClientFactory) *GitHubActivities {
	return &GitHubActivities{
		clientFactory: clientFactory,
	}
}

// CreateGitHubDeployment creates a new deployment in GitHub
func (a *GitHubActivities) CreateGitHubDeployment(ctx context.Context, input CreateDeploymentInput) (*CreateDeploymentResult, error) {
	activityInfo := activity.GetInfo(ctx)
	logger := logging.ActivityLogger("CreateGitHubDeployment", activityInfo.WorkflowExecution.ID, activityInfo.WorkflowExecution.RunID)
	
	logger.Info().
		Str("github_owner", input.GithubOwner).
		Str("github_repo", input.GithubRepo).
		Str("commit", input.CommitSHA).
		Str("environment", input.Environment).
		Msg("Creating GitHub deployment")
	
	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Creating GitHub client")
	
	// Create GitHub client
	client, err := a.clientFactory.CreateClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}
	
	// Prepare deployment payload
	payload := map[string]interface{}{
		"triggered_by": "temporal-workflow",
		"created_at":   time.Now().UTC().Format(time.RFC3339),
	}
	
	// Add Harness metadata if provided
	if input.HarnessExecutionID != "" {
		payload["harness_execution_id"] = input.HarnessExecutionID
	}
	if input.HarnessPipelineID != "" {
		payload["harness_pipeline_id"] = input.HarnessPipelineID
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
	deployment, _, err := client.Repositories.CreateDeployment(ctx, input.GithubOwner, input.GithubRepo, deploymentRequest)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create GitHub deployment")
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}
	
	result := &CreateDeploymentResult{
		DeploymentID: deployment.GetID(),
		URL:          deployment.GetURL(),
		Environment:  deployment.GetEnvironment(),
	}
	
	logger.Info().
		Int64("deployment_id", result.DeploymentID).
		Str("url", result.URL).
		Msg("Successfully created GitHub deployment")
	
	return result, nil
}

// UpdateGitHubDeploymentStatus updates the status of a deployment
func (a *GitHubActivities) UpdateGitHubDeploymentStatus(ctx context.Context, input UpdateDeploymentStatusInput) error {
	activityInfo := activity.GetInfo(ctx)
	logger := logging.ActivityLogger("UpdateGitHubDeploymentStatus", activityInfo.WorkflowExecution.ID, activityInfo.WorkflowExecution.RunID)
	
	logger.Info().
		Str("github_owner", input.GithubOwner).
		Str("github_repo", input.GithubRepo).
		Int64("deployment_id", input.DeploymentID).
		Str("state", input.State).
		Msg("Updating GitHub deployment status")
	
	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Creating GitHub client")
	
	// Create GitHub client
	client, err := a.clientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}
	
	// Create status request
	statusRequest := &github.DeploymentStatusRequest{
		State:          github.String(input.State),
		Description:    github.String(truncateDescription(input.Description, 140)),
		AutoInactive:   github.Bool(true), // Automatically mark previous deployments as inactive
	}
	
	// Add URLs if provided
	if input.LogURL != "" {
		statusRequest.LogURL = github.String(input.LogURL)
	}
	if input.EnvironmentURL != "" {
		statusRequest.EnvironmentURL = github.String(input.EnvironmentURL)
	}
	
	// Record heartbeat before API call
	activity.RecordHeartbeat(ctx, "Calling GitHub API")
	
	// Update deployment status
	status, _, err := client.Repositories.CreateDeploymentStatus(ctx, input.GithubOwner, input.GithubRepo, input.DeploymentID, statusRequest)
	if err != nil {
		logger.Error().Err(err).
			Str("state", input.State).
			Msg("Failed to update GitHub deployment status")
		return fmt.Errorf("failed to update deployment status: %w", err)
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