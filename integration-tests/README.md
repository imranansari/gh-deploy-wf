# Integration Tests

This folder contains test scripts used to validate the GitHub Deployments API integration during development.

## Test Scripts

### Authentication Tests
- `get-installation/` - Helper to find GitHub App installation ID
- `debug-auth/` - Debug GitHub App authentication issues
- `simple-test/` - Basic authentication and repository access test

### Deployment Tests  
- `test-deployment/` - Basic deployment creation and status updates
- `test-github/` - Original GitHub API test (with app-level calls)
- `test-dev-deployment/` - Simulate development environment deployments
- `test-pr-preview/` - Simulate PR preview environment deployments (recommended)

### PR Integration Tests
- `test-pr-deployment/` - Multi-environment PR deployment simulation
- `test-failed-deployment/` - Simulate failed deployment scenarios
- `create-simple-pr/` - Create test PRs via API
- `create-test-pr/` - Create test PRs with more complex commits

## Usage

These scripts were used to:
1. Validate GitHub App authentication works
2. Test deployment creation for specific commits/PRs
3. Verify deployment status updates appear on PRs
4. Understand how GitHub displays deployment information

## Key Findings

- GitHub App (streamcommander, ID: 319033) authentication works correctly
- Environment name "pr-preview" is clearer than "development" for PR deployments
- Deployments marked as `transient` are appropriate for PR environments
- Deployment statuses appear correctly on PR pages with proper visual indicators

## Environment Variables Required

```env
GITHUB_APP_ID=319033
GITHUB_INSTALLATION_ID=36477471
GITHUB_PRIVATE_KEY_PATH=./.private/streamcommander.2025-07-12.private-key.pem
TEST_REPO_OWNER=imranansari
TEST_REPO_NAME=gh-deploy-test
```