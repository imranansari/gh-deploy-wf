# GitHub Deployment Workflow

Temporal-based system for tracking GitHub deployments triggered by Harness pipeline cloud events.

## Overview

This system orchestrates GitHub deployment creation and status updates using Temporal workflows. It integrates with Harness CI/CD pipelines through an event-driven architecture (EDA) where Harness publishes cloud events that trigger deployment status updates.

## Architecture

### Event-Driven Flow

1. **Initial Deployment**: Temporal workflow creates GitHub deployment
2. **Harness Events**: Harness pipeline publishes cloud events for different stages
3. **Status Updates**: Cloud events trigger Temporal workflows to update deployment status

### Deployment States

- `pending` - CI build started
- `in_progress` - Deployment in progress (shows spinning indicator in GitHub)
- `success` - Deployment completed successfully
- `failure` - Deployment failed

## Components

### Workflows

- **GitHubDeploymentWorkflow**: Creates initial GitHub deployment
- **UpdateDeploymentWorkflow**: Updates deployment status based on cloud events

### Activities

- **CreateGitHubDeployment**: Creates deployment in GitHub via API
- **FindGitHubDeployment**: Finds existing deployment by repo/commit/environment
- **UpdateGitHubDeploymentStatus**: Updates deployment status

### Configuration

Environment-based configuration using `caarlos0/env`:
- GitHub App authentication with dynamic installation ID resolution per organization
- Separate Enterprise and GitHub.com client implementations (no accidental cross-connection)
- Temporal connection settings
- File-based secrets for Kubernetes compatibility

## Usage

### Start Worker

```bash
go run worker/main.go
```

### Create Deployment

```bash
go run test/main.go
```

### Test Status Updates

```bash
go run cmd/update-test/main.go
```

## Integration with Harness

Harness pipelines publish cloud events containing:

```json
{
  "github_owner": "owner",
  "github_repo": "repo",
  "commit_sha": "abc123...",
  "environment": "pr-preview", 
  "state": "in_progress",
  "log_url": "https://ci.harness.io/builds/123"
}
```

These events trigger `UpdateDeploymentWorkflow` which:
1. Finds the deployment using repo/commit/environment
2. Updates the GitHub deployment status
3. Provides visual feedback in GitHub UI

## Environment Constants

Predefined environment types prevent typos:

- `config.EnvironmentProduction`
- `config.EnvironmentStaging` 
- `config.EnvironmentPRPreview`
- `config.EnvironmentDevelopment`
- `config.EnvironmentTesting`

## Configuration

Set environment variables or use `.env` file:

```
TEMPORAL_HOST=localhost:7233
TEMPORAL_TASK_QUEUE=github-deployments
GITHUB_APP_ID=319033
GITHUB_ENTERPRISE_URL=  # Set to your Enterprise URL (e.g., https://github.mycompany.com)
SECRETS_PATH=.private
```

## Architecture

See [Architecture Documentation](docs/architecture.md) for:
- C4 Component diagram showing system boundaries
- High-level sequence diagram of complete flow
- Low-level workflow sequence diagrams

## Dependencies

- Temporal Server running on localhost:7233
- GitHub App with deployment permissions
- Go 1.21+