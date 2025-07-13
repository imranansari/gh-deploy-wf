package workflows

import "time"

// DeploymentWorkflowInput represents the input for the GitHub deployment workflow
type DeploymentWorkflowInput struct {
	// GitHub Repository Information
	GithubOwner string `json:"github_owner"`
	GithubRepo  string `json:"github_repo"`
	CommitSHA   string `json:"commit_sha"`
	BranchName  string `json:"branch_name,omitempty"`
	
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