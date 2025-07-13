package activities

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