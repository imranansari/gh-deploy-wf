package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/imranansari/gh-deploy-wf/activities"
)

const (
	WorkflowTimeout = 30 * time.Minute // Simplified timeout for MVP
)

// DeploymentWorkflowInput represents the input for the GitHub deployment workflow
type DeploymentWorkflowInput struct {
	// GitHub Repository Information
	GithubOwner string `json:"github_owner"`
	GithubRepo  string `json:"github_repo"`
	CommitSHA   string `json:"commit_sha"`
	
	// Deployment Configuration
	Environment   string `json:"environment"`
	Description   string `json:"description,omitempty"`
	IsTransient   bool   `json:"is_transient"`
	
	// External System Integration
	HarnessPipelineID  string `json:"harness_pipeline_id,omitempty"`
	HarnessExecutionID string `json:"harness_execution_id,omitempty"`
	
	// Deployment Metadata
	LogURL         string            `json:"log_url,omitempty"`
	EnvironmentURL string            `json:"environment_url,omitempty"`
	Payload        map[string]string `json:"payload,omitempty"`
}

// DeploymentWorkflowResult represents the result of the deployment workflow
type DeploymentWorkflowResult struct {
	DeploymentID   int64     `json:"deployment_id"`
	FinalStatus    string    `json:"final_status"`
	Environment    string    `json:"environment"`
	EnvironmentURL string    `json:"environment_url,omitempty"`
	CompletedAt    time.Time `json:"completed_at"`
	TotalDuration  string    `json:"total_duration"`
	StatusUpdates  int       `json:"status_updates"`
}

// DeploymentStatusUpdate represents a status update for the deployment
type DeploymentStatusUpdate struct {
	Status         string    `json:"status"`
	Description    string    `json:"description,omitempty"`
	LogURL         string    `json:"log_url,omitempty"`
	EnvironmentURL string    `json:"environment_url,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GitHubDeploymentWorkflow orchestrates GitHub deployment creation and status updates
func GitHubDeploymentWorkflow(ctx workflow.Context, input DeploymentWorkflowInput) (*DeploymentWorkflowResult, error) {
	// Create workflow-specific logger
	logger := workflow.GetLogger(ctx)
	
	// Initialize result
	result := &DeploymentWorkflowResult{
		StatusUpdates: 0,
		Environment:   input.Environment,
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
			MaximumAttempts:        3,
			NonRetryableErrorTypes: []string{"ValidationError", "AuthenticationError"},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)
	
	// Log workflow start
	logger.Info("Starting GitHub deployment workflow",
		"github_owner", input.GithubOwner,
		"github_repo", input.GithubRepo,
		"commit", input.CommitSHA,
		"environment", input.Environment,
		"harness_execution_id", input.HarnessExecutionID)
	
	// 1. Create GitHub deployment
	logger.Info("Creating GitHub deployment")
	
	createInput := activities.CreateDeploymentInput{
		GithubOwner:        input.GithubOwner,
		GithubRepo:         input.GithubRepo,
		CommitSHA:          input.CommitSHA,
		Environment:        input.Environment,
		Description:        input.Description,
		IsTransient:        input.IsTransient,
		HarnessExecutionID: input.HarnessExecutionID,
		HarnessPipelineID:  input.HarnessPipelineID,
		Payload:            input.Payload,
	}
	
	var deploymentResult *activities.CreateDeploymentResult
	createDeploymentActivity := workflow.ExecuteActivity(ctx, "CreateGitHubDeployment", createInput)
	
	if err := createDeploymentActivity.Get(ctx, &deploymentResult); err != nil {
		logger.Error("Failed to create GitHub deployment", "error", err)
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}
	
	result.DeploymentID = deploymentResult.DeploymentID
	logger.Info("GitHub deployment created", "deployment_id", deploymentResult.DeploymentID)
	
	// 2. Update to initial status (queued/in_progress)
	initialStatus := "queued"
	if input.LogURL != "" {
		initialStatus = "in_progress"
	}
	
	updateInput := activities.UpdateDeploymentStatusInput{
		GithubOwner:    input.GithubOwner,
		GithubRepo:     input.GithubRepo,
		DeploymentID:   deploymentResult.DeploymentID,
		State:          initialStatus,
		Description:    getInitialStatusDescription(initialStatus, input.Environment),
		LogURL:         input.LogURL,
		EnvironmentURL: "", // No environment URL yet
	}
	
	updateStatusActivity := workflow.ExecuteActivity(ctx, "UpdateGitHubDeploymentStatus", updateInput)
	
	if err := updateStatusActivity.Get(ctx, nil); err != nil {
		logger.Error("Failed to update initial deployment status", "error", err)
		// Continue anyway - deployment was created
	} else {
		result.StatusUpdates++
		logger.Info("Updated deployment to initial status", "status", initialStatus)
	}
	
	// 3. For MVP, immediately mark as success
	// In full implementation, this would wait for signals or external updates
	
	// Small delay to simulate deployment process
	workflow.Sleep(ctx, 2*time.Second)
	
	// Update to success status
	finalUpdateInput := activities.UpdateDeploymentStatusInput{
		GithubOwner:    input.GithubOwner,
		GithubRepo:     input.GithubRepo,
		DeploymentID:   deploymentResult.DeploymentID,
		State:          "success",
		Description:    fmt.Sprintf("Successfully deployed to %s environment", input.Environment),
		LogURL:         input.LogURL,
		EnvironmentURL: input.EnvironmentURL,
	}
	
	finalUpdateActivity := workflow.ExecuteActivity(ctx, "UpdateGitHubDeploymentStatus", finalUpdateInput)
	
	if err := finalUpdateActivity.Get(ctx, nil); err != nil {
		logger.Error("Failed to update final deployment status", "error", err)
		result.FinalStatus = "error"
	} else {
		result.StatusUpdates++
		result.FinalStatus = "success"
		result.EnvironmentURL = input.EnvironmentURL
		logger.Info("Updated deployment to success status")
	}
	
	// Calculate final metrics
	endTime := workflow.Now(ctx)
	result.CompletedAt = endTime
	result.TotalDuration = endTime.Sub(startTime).String()
	
	logger.Info("Deployment workflow completed",
		"deployment_id", result.DeploymentID,
		"final_status", result.FinalStatus,
		"duration", result.TotalDuration,
		"status_updates", result.StatusUpdates)
	
	return result, nil
}

// getInitialStatusDescription returns an appropriate description for the initial status
func getInitialStatusDescription(status, environment string) string {
	switch status {
	case "queued":
		return fmt.Sprintf("Deployment to %s environment queued", environment)
	case "in_progress":
		return fmt.Sprintf("Deploying to %s environment", environment)
	default:
		return fmt.Sprintf("Deployment to %s environment started", environment)
	}
}