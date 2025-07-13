# Setup Guide

## Prerequisites

1. **Go 1.21+** installed
2. **Temporal** running locally
3. **GitHub App** access (streamcommander)

## GitHub App Configuration

### Using the Existing App

The `streamcommander` GitHub App is already configured:
- **App ID**: 319033
- **Private Key**: `.private/streamcommander.2025-07-12.private-key.pem`

### Getting the Installation ID

1. Go to your repository on GitHub
2. Settings → GitHub Apps → streamcommander → Configure
3. The URL will contain the installation ID: `https://github.com/settings/installations/{INSTALLATION_ID}`

Or use the GitHub CLI:
```bash
gh api /app/installations --jq '.[].id'
```

## Environment Setup

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Update `.env` with your values:
```env
# Temporal Configuration
TEMPORAL_HOST=localhost:7233
TASK_QUEUE=github-deployments

# GitHub App Configuration
GITHUB_APP_ID=319033
GITHUB_INSTALLATION_ID=<your-installation-id>
GITHUB_PRIVATE_KEY_PATH=./.private/streamcommander.2025-07-12.private-key.pem

# Optional: For testing specific repos
TEST_REPO_OWNER=<your-github-username>
TEST_REPO_NAME=<your-test-repo>
```

## Running Temporal Locally

```bash
# Install Temporal CLI if needed
brew install temporal

# Start Temporal server
temporal server start-dev
```

The Temporal Web UI will be available at: http://localhost:8233

## Quick Test

After setup, you can verify everything works:

1. Start the worker (after implementing)
2. Run the test client
3. Check GitHub repository for deployment
4. Check Temporal UI for workflow execution