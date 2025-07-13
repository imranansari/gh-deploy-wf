package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v58/github"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	client, err := createGitHubClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx := context.Background()
	owner := os.Getenv("TEST_REPO_OWNER")
	repo := os.Getenv("TEST_REPO_NAME")

	// Create a new file in a new branch directly
	branchName := fmt.Sprintf("test-deployment-%d", time.Now().Unix())
	fileName := fmt.Sprintf("deployment-test-%d.md", time.Now().Unix())
	
	fileContent := fmt.Sprintf(`# Deployment Test File

Created at: %s

This file is used to test deployment status on pull requests.

## Test Scenarios
- Preview environment deployments
- Staging environment deployments
- Production environment deployments

## Expected Behavior
When deployments are created for this PR's commit:
1. Deployment status should appear in the PR
2. Multiple environments should be visible
3. Status updates should be reflected in real-time
`, time.Now().Format(time.RFC3339))

	// Create file (this will also create the branch)
	opts := &github.RepositoryContentFileOptions{
		Message: github.String("Add deployment test file"),
		Content: []byte(fileContent),
		Branch:  github.String(branchName),
	}

	file, _, err := client.Repositories.CreateFile(ctx, owner, repo, fileName, opts)
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}
	log.Printf("✓ Created file: %s on branch: %s\n", fileName, branchName)

	// Create pull request
	pr, _, err := client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: github.String(fmt.Sprintf("Test PR for Deployment Status - %s", time.Now().Format("Jan 2 15:04"))),
		Head:  github.String(branchName),
		Base:  github.String("main"),
		Body:  github.String(`## Testing Deployment Status on PRs

This PR demonstrates how GitHub deployment statuses appear on pull requests.

### What will be tested:
- 🔵 **Preview Environment**: Temporary deployment for this PR
- 🟡 **Staging Environment**: Pre-production deployment
- 🟢 **Production Environment**: (if needed)

### Expected Results:
- Deployment status badges should appear below
- Each environment should have its own status
- Clicking on the status should show deployment details

---
**Note**: This is an automated test PR created by the deployment workflow test script.`),
	})
	if err != nil {
		log.Fatalf("Failed to create PR: %v", err)
	}

	log.Printf("\n✅ Successfully created PR #%d\n", pr.GetNumber())
	log.Printf("  Title: %s\n", pr.GetTitle())
	log.Printf("  Branch: %s → main\n", branchName)
	log.Printf("  Commit: %s\n", file.GetSHA()[:8])
	log.Printf("  URL: %s\n", pr.GetHTMLURL())
	
	log.Printf("\n📋 Next steps:\n")
	log.Printf("1. View the PR: %s\n", pr.GetHTMLURL())
	log.Printf("2. Run deployment test: go run cmd/test-pr-deployment/main.go -pr=%d\n", pr.GetNumber())
	log.Printf("3. Watch the deployment status appear on the PR!\n")
}

func createGitHubClient() (*github.Client, error) {
	appID, err := strconv.ParseInt(os.Getenv("GITHUB_APP_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %v", err)
	}

	installationID, err := strconv.ParseInt(os.Getenv("GITHUB_INSTALLATION_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_INSTALLATION_ID: %v", err)
	}

	privateKey, err := os.ReadFile(os.Getenv("GITHUB_PRIVATE_KEY_PATH"))
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}

	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %v", err)
	}

	return github.NewClient(&http.Client{Transport: itr}), nil
}