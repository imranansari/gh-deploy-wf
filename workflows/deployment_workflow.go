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
	DeploymentID   int64  `json:"deployment_id"`
	FinalStatus    string `json:"final_status"`
	Environment    string `json:"environment"`
	EnvironmentURL string `json:"environment_url,omitempty"`
	CompletedAt    string `json:"completed_at"`
	TotalDuration  string `json:"total_duration"`
	StatusUpdates  int    `json:"status_updates"`
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
	
	// Configure activity options with comprehensive retry logging
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
	
	// Get workflow info for structured logging
	workflowInfo := workflow.GetInfo(ctx)
	
	// Log workflow start
	logger.Info("Starting GitHub deployment workflow",
		"workflow_id", workflowInfo.WorkflowExecution.ID,
		"run_id", workflowInfo.WorkflowExecution.RunID,
		"github_owner", input.GithubOwner,
		"github_repo", input.GithubRepo,
		"commit", input.CommitSHA,
		"environment", input.Environment,
		"harness_execution_id", input.HarnessExecutionID,
		"harness_pipeline_id", input.HarnessPipelineID,
		"is_transient", input.IsTransient)
	
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
		logger.Error("Failed to create GitHub deployment",
			"error", err,
			"github_owner", input.GithubOwner,
			"github_repo", input.GithubRepo,
			"commit", input.CommitSHA,
			"environment", input.Environment,
			"workflow_id", workflowInfo.WorkflowExecution.ID)
		return nil, fmt.Errorf("failed to create deployment for %s/%s@%s in %s environment: %w", 
			input.GithubOwner, input.GithubRepo, input.CommitSHA, input.Environment, err)
	}
	
	result.DeploymentID = deploymentResult.DeploymentID
	logger.Info("GitHub deployment created",
		"deployment_id", deploymentResult.DeploymentID,
		"deployment_url", deploymentResult.URL,
		"github_owner", input.GithubOwner,
		"github_repo", input.GithubRepo,
		"commit", input.CommitSHA,
		"environment", deploymentResult.Environment)
	
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
		logger.Error("Failed to update initial deployment status",
			"error", err,
			"deployment_id", deploymentResult.DeploymentID,
			"target_status", initialStatus,
			"github_owner", input.GithubOwner,
			"github_repo", input.GithubRepo,
			"workflow_id", workflowInfo.WorkflowExecution.ID,
			"activity_retry_count", workflow.GetInfo(ctx).Attempt)
		// Continue anyway - deployment was created
	} else {
		result.StatusUpdates++
		logger.Info("Updated deployment to initial status",
			"status", initialStatus,
			"deployment_id", deploymentResult.DeploymentID,
			"github_owner", input.GithubOwner,
			"github_repo", input.GithubRepo,
			"workflow_id", workflowInfo.WorkflowExecution.ID)
	}
	
	// 3. For MVP, immediately mark as success
	// In full implementation, this would wait for signals or external updates
	
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
		logger.Error("Failed to update final deployment status",
			"error", err,
			"deployment_id", deploymentResult.DeploymentID,
			"target_status", "success",
			"github_owner", input.GithubOwner,
			"github_repo", input.GithubRepo,
			"workflow_id", workflowInfo.WorkflowExecution.ID,
			"activity_retry_count", workflow.GetInfo(ctx).Attempt)
		result.FinalStatus = "error"
	} else {
		result.StatusUpdates++
		result.FinalStatus = "success"
		result.EnvironmentURL = input.EnvironmentURL
		logger.Info("Updated deployment to success status",
			"deployment_id", deploymentResult.DeploymentID,
			"environment_url", input.EnvironmentURL,
			"github_owner", input.GithubOwner,
			"github_repo", input.GithubRepo,
			"workflow_id", workflowInfo.WorkflowExecution.ID)
	}
	
	// Calculate final metrics
	endTime := workflow.Now(ctx)
	result.CompletedAt = endTime.Format(time.RFC3339)
	result.TotalDuration = endTime.Sub(startTime).String()
	
	// Ensure all required fields are set
	if result.DeploymentID == 0 {
		logger.Error("DeploymentID is 0 - this should not happen")
		return nil, fmt.Errorf("deployment ID was not set properly")
	}
	
	if result.FinalStatus == "" {
		logger.Error("FinalStatus is empty - this should not happen") 
		result.FinalStatus = "unknown"
	}
	
	logger.Info("Deployment workflow completed",
		"workflow_id", workflowInfo.WorkflowExecution.ID,
		"deployment_id", result.DeploymentID,
		"final_status", result.FinalStatus,
		"duration", result.TotalDuration,
		"status_updates", result.StatusUpdates,
		"github_owner", input.GithubOwner,
		"github_repo", input.GithubRepo,
		"commit", input.CommitSHA,
		"environment", input.Environment,
		"harness_execution_id", input.HarnessExecutionID)
	
	logger.Info("Returning workflow result",
		"result_deployment_id", result.DeploymentID,
		"result_final_status", result.FinalStatus,
		"result_environment", result.Environment,
		"result_status_updates", result.StatusUpdates)
	
	return result, nil
}

// DeploymentUpdateInput represents input for updating an existing deployment's status
type DeploymentUpdateInput struct {
	// GitHub Repository Information (to find deployment)
	GithubOwner string `json:"github_owner"`
	GithubRepo  string `json:"github_repo"`
	CommitSHA   string `json:"commit_sha"`
	Environment string `json:"environment"`
	
	// Status Update Information
	State          string `json:"state"`
	Description    string `json:"description"`
	LogURL         string `json:"log_url,omitempty"`
	EnvironmentURL string `json:"environment_url,omitempty"`
}

// DeploymentUpdateResult represents the result of a deployment status update
type DeploymentUpdateResult struct {
	DeploymentID   int64  `json:"deployment_id"`
	UpdatedStatus  string `json:"updated_status"`
	Environment    string `json:"environment"`
	UpdatedAt      string `json:"updated_at"`
	LogURL         string `json:"log_url,omitempty"`
	EnvironmentURL string `json:"environment_url,omitempty"`
}

// UpdateDeploymentWorkflow updates an existing GitHub deployment status based on cloud events
func UpdateDeploymentWorkflow(ctx workflow.Context, input DeploymentUpdateInput) (*DeploymentUpdateResult, error) {
	// Create workflow-specific logger
	logger := workflow.GetLogger(ctx)
	
	// Configure activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute, // Shorter timeout for updates
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
	
	// Get workflow info for structured logging
	workflowInfo := workflow.GetInfo(ctx)
	
	// Log workflow start
	logger.Info("Starting deployment status update workflow",
		"workflow_id", workflowInfo.WorkflowExecution.ID,
		"run_id", workflowInfo.WorkflowExecution.RunID,
		"github_owner", input.GithubOwner,
		"github_repo", input.GithubRepo,
		"commit", input.CommitSHA,
		"environment", input.Environment,
		"new_state", input.State,
		"log_url", input.LogURL,
		"environment_url", input.EnvironmentURL)
	
	// 1. Find the existing deployment
	logger.Info("Finding existing GitHub deployment")
	
	findInput := activities.FindDeploymentInput{
		GithubOwner: input.GithubOwner,
		GithubRepo:  input.GithubRepo,
		CommitSHA:   input.CommitSHA,
		Environment: input.Environment,
	}
	
	var deploymentID int64
	findActivity := workflow.ExecuteActivity(ctx, "FindGitHubDeployment", findInput)
	
	if err := findActivity.Get(ctx, &deploymentID); err != nil {
		logger.Error("Failed to find GitHub deployment",
			"error", err,
			"github_owner", input.GithubOwner,
			"github_repo", input.GithubRepo,
			"commit", input.CommitSHA,
			"environment", input.Environment,
			"workflow_id", workflowInfo.WorkflowExecution.ID)
		return nil, fmt.Errorf("failed to find deployment for %s/%s@%s in %s environment: %w", 
			input.GithubOwner, input.GithubRepo, input.CommitSHA, input.Environment, err)
	}
	
	logger.Info("Found GitHub deployment",
		"deployment_id", deploymentID,
		"github_owner", input.GithubOwner,
		"github_repo", input.GithubRepo,
		"commit", input.CommitSHA,
		"environment", input.Environment)
	
	// 2. Update deployment status
	updateInput := activities.UpdateDeploymentStatusInput{
		GithubOwner:    input.GithubOwner,
		GithubRepo:     input.GithubRepo,
		DeploymentID:   deploymentID,
		State:          input.State,
		Description:    input.Description,
		LogURL:         input.LogURL,
		EnvironmentURL: input.EnvironmentURL,
	}
	
	updateActivity := workflow.ExecuteActivity(ctx, "UpdateGitHubDeploymentStatus", updateInput)
	
	if err := updateActivity.Get(ctx, nil); err != nil {
		logger.Error("Failed to update deployment status",
			"error", err,
			"deployment_id", deploymentID,
			"target_state", input.State,
			"github_owner", input.GithubOwner,
			"github_repo", input.GithubRepo,
			"workflow_id", workflowInfo.WorkflowExecution.ID,
			"activity_retry_count", workflow.GetInfo(ctx).Attempt)
		return nil, fmt.Errorf("failed to update deployment %d status to %s for %s/%s: %w", 
			deploymentID, input.State, input.GithubOwner, input.GithubRepo, err)
	}
	
	// Create result object
	result := &DeploymentUpdateResult{
		DeploymentID:   deploymentID,
		UpdatedStatus:  input.State,
		Environment:    input.Environment,
		UpdatedAt:      workflow.Now(ctx).Format(time.RFC3339),
		LogURL:         input.LogURL,
		EnvironmentURL: input.EnvironmentURL,
	}
	
	logger.Info("Successfully updated deployment status",
		"workflow_id", workflowInfo.WorkflowExecution.ID,
		"deployment_id", deploymentID,
		"new_state", input.State,
		"github_owner", input.GithubOwner,
		"github_repo", input.GithubRepo,
		"commit", input.CommitSHA,
		"environment", input.Environment,
		"log_url", input.LogURL,
		"environment_url", input.EnvironmentURL)
	
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