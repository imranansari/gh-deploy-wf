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

// CreateDeploymentInput represents input for creating a deployment
type CreateDeploymentInput struct {
	GithubOwner        string            `json:"github_owner"`
	GithubRepo         string            `json:"github_repo"`
	CommitSHA          string            `json:"commit_sha"`
	Environment        string            `json:"environment"`
	Description        string            `json:"description"`
	IsTransient        bool              `json:"is_transient"`
	HarnessExecutionID string            `json:"harness_execution_id"`
	HarnessPipelineID  string            `json:"harness_pipeline_id"`
	Payload            map[string]string `json:"payload"`
}

// CreateDeploymentResult represents the result of creating a deployment
type CreateDeploymentResult struct {
	DeploymentID int64  `json:"deployment_id"`
	URL          string `json:"url"`
	Environment  string `json:"environment"`
}

// FindDeploymentInput represents input for finding a deployment
type FindDeploymentInput struct {
	GithubOwner string `json:"github_owner"`
	GithubRepo  string `json:"github_repo"`
	CommitSHA   string `json:"commit_sha"`
	Environment string `json:"environment"`
}

// UpdateDeploymentStatusInput represents input for updating deployment status
type UpdateDeploymentStatusInput struct {
	GithubOwner    string `json:"github_owner"`
	GithubRepo     string `json:"github_repo"`
	DeploymentID   int64  `json:"deployment_id"`
	State          string `json:"state"`
	Description    string `json:"description"`
	LogURL         string `json:"log_url"`
	EnvironmentURL string `json:"environment_url"`
}

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
		Str("activity_id", activityInfo.ActivityID).
		Int32("attempt", activityInfo.Attempt).
		Msg("Creating GitHub deployment")
	
	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Creating GitHub client")
	
	// Create GitHub client for the organization
	client, err := a.clientFactory.CreateClientForOrg(ctx, input.GithubOwner)
	if err != nil {
		logger.Error().
			Err(err).
			Str("github_owner", input.GithubOwner).
			Str("github_repo", input.GithubRepo).
			Msg("Failed to create GitHub client for organization")
		return nil, fmt.Errorf("failed to create GitHub client for organization %s: %w", input.GithubOwner, err)
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
		ProductionEnvironment: github.Bool(input.Environment == "production"), // TODO: Use config.EnvironmentProduction
		RequiredContexts:      &[]string{}, // Skip status checks for external deployments
		AutoMerge:             github.Bool(false),
		Payload:               payload,
	}
	
	// Record heartbeat before API call
	activity.RecordHeartbeat(ctx, "Calling GitHub API")
	
	// Create deployment
	deployment, response, err := client.Repositories.CreateDeployment(ctx, input.GithubOwner, input.GithubRepo, deploymentRequest)
	if err != nil {
		logger.Error().
			Err(err).
			Str("github_owner", input.GithubOwner).
			Str("github_repo", input.GithubRepo).
			Str("commit", input.CommitSHA).
			Str("environment", input.Environment).
			Int("http_status", response.StatusCode).
			Str("response_status", response.Status).
			Msg("Failed to create GitHub deployment")
		return nil, fmt.Errorf("failed to create deployment for %s/%s@%s in %s environment: %w", 
			input.GithubOwner, input.GithubRepo, input.CommitSHA, input.Environment, err)
	}
	
	result := &CreateDeploymentResult{
		DeploymentID: deployment.GetID(),
		URL:          deployment.GetURL(),
		Environment:  deployment.GetEnvironment(),
	}
	
	logger.Info().
		Int64("deployment_id", result.DeploymentID).
		Str("url", result.URL).
		Str("github_owner", input.GithubOwner).
		Str("github_repo", input.GithubRepo).
		Str("commit", input.CommitSHA).
		Str("environment", input.Environment).
		Str("harness_execution_id", input.HarnessExecutionID).
		Dur("duration", time.Since(time.Now())).  // TODO: Track actual duration
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
		Str("activity_id", activityInfo.ActivityID).
		Int32("attempt", activityInfo.Attempt).
		Str("log_url", input.LogURL).
		Str("environment_url", input.EnvironmentURL).
		Msg("Updating GitHub deployment status")
	
	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Creating GitHub client")
	
	// Create GitHub client for the organization
	client, err := a.clientFactory.CreateClientForOrg(ctx, input.GithubOwner)
	if err != nil {
		logger.Error().
			Err(err).
			Str("github_owner", input.GithubOwner).
			Str("github_repo", input.GithubRepo).
			Int64("deployment_id", input.DeploymentID).
			Msg("Failed to create GitHub client for organization")
		return fmt.Errorf("failed to create GitHub client for organization %s: %w", input.GithubOwner, err)
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
	status, response, err := client.Repositories.CreateDeploymentStatus(ctx, input.GithubOwner, input.GithubRepo, input.DeploymentID, statusRequest)
	if err != nil {
		logger.Error().
			Err(err).
			Str("github_owner", input.GithubOwner).
			Str("github_repo", input.GithubRepo).
			Int64("deployment_id", input.DeploymentID).
			Str("state", input.State).
			Int("http_status", response.StatusCode).
			Str("response_status", response.Status).
			Msg("Failed to update GitHub deployment status")
		return fmt.Errorf("failed to update deployment %d status to %s for %s/%s: %w", 
			input.DeploymentID, input.State, input.GithubOwner, input.GithubRepo, err)
	}
	
	logger.Info().
		Int64("deployment_id", input.DeploymentID).
		Str("state", status.GetState()).
		Str("url", status.GetURL()).
		Str("github_owner", input.GithubOwner).
		Str("github_repo", input.GithubRepo).
		Int64("status_id", status.GetID()).
		Time("updated_at", status.GetUpdatedAt().Time).
		Msg("Successfully updated GitHub deployment status")
	
	return nil
}

// FindGitHubDeployment finds a deployment by repository, commit SHA, and environment
func (a *GitHubActivities) FindGitHubDeployment(ctx context.Context, input FindDeploymentInput) (int64, error) {
	activityInfo := activity.GetInfo(ctx)
	logger := logging.ActivityLogger("FindGitHubDeployment", activityInfo.WorkflowExecution.ID, activityInfo.WorkflowExecution.RunID)
	
	logger.Info().
		Str("github_owner", input.GithubOwner).
		Str("github_repo", input.GithubRepo).
		Str("commit", input.CommitSHA).
		Str("environment", input.Environment).
		Str("activity_id", activityInfo.ActivityID).
		Int32("attempt", activityInfo.Attempt).
		Msg("Finding GitHub deployment")
	
	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Creating GitHub client")
	
	// Create GitHub client for the organization
	client, err := a.clientFactory.CreateClientForOrg(ctx, input.GithubOwner)
	if err != nil {
		logger.Error().
			Err(err).
			Str("github_owner", input.GithubOwner).
			Str("github_repo", input.GithubRepo).
			Msg("Failed to create GitHub client for organization")
		return 0, fmt.Errorf("failed to create GitHub client for organization %s: %w", input.GithubOwner, err)
	}
	
	// List deployments with filters
	deployments, _, err := client.Repositories.ListDeployments(ctx, input.GithubOwner, input.GithubRepo, &github.DeploymentsListOptions{
		SHA:         input.CommitSHA,
		Environment: input.Environment,
		ListOptions: github.ListOptions{
			PerPage: 10, // Only need recent deployments
		},
	})
	if err != nil {
		logger.Error().
			Err(err).
			Str("github_owner", input.GithubOwner).
			Str("github_repo", input.GithubRepo).
			Str("commit", input.CommitSHA).
			Str("environment", input.Environment).
			Msg("Failed to list GitHub deployments")
		return 0, fmt.Errorf("failed to list deployments for %s/%s@%s in %s environment: %w", 
			input.GithubOwner, input.GithubRepo, input.CommitSHA, input.Environment, err)
	}
	
	if len(deployments) == 0 {
		logger.Error().
			Str("github_owner", input.GithubOwner).
			Str("github_repo", input.GithubRepo).
			Str("commit", input.CommitSHA).
			Str("environment", input.Environment).
			Msg("No deployment found matching criteria")
		return 0, fmt.Errorf("no deployment found for %s/%s@%s in %s environment", 
			input.GithubOwner, input.GithubRepo, input.CommitSHA, input.Environment)
	}
	
	// Return the most recent deployment (first in list)
	deploymentID := deployments[0].GetID()
	
	logger.Info().
		Int64("deployment_id", deploymentID).
		Int("total_found", len(deployments)).
		Str("github_owner", input.GithubOwner).
		Str("github_repo", input.GithubRepo).
		Str("commit", input.CommitSHA).
		Str("environment", input.Environment).
		Time("deployment_created_at", deployments[0].GetCreatedAt().Time).
		Msg("Successfully found GitHub deployment")
	
	return deploymentID, nil
}

// truncateDescription ensures description doesn't exceed GitHub's limit
func truncateDescription(desc string, maxLen int) string {
	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen-3] + "..."
}