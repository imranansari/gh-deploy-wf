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

	// Create GitHub client
	client, err := createGitHubClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx := context.Background()
	owner := os.Getenv("TEST_REPO_OWNER")
	repo := os.Getenv("TEST_REPO_NAME")

	// Get PR #2
	prNum := 2
	pr, _, err := client.PullRequests.Get(ctx, owner, repo, prNum)
	if err != nil {
		log.Fatalf("Failed to get PR: %v", err)
	}

	log.Printf("Creating a failed deployment scenario for PR #%d\n", prNum)

	// Create production deployment that fails
	deployment, err := createDeployment(ctx, client, owner, repo, pr.GetHead().GetSHA(), "production", 
		fmt.Sprintf("Deploy PR #%d to production", prNum))
	if err != nil {
		log.Fatalf("Failed to create deployment: %v", err)
	}
	
	log.Printf("✓ Created production deployment (ID: %d)\n", deployment.GetID())

	// Simulate deployment process
	statuses := []struct {
		state       string
		description string
		delay       time.Duration
	}{
		{"queued", "Production deployment queued", 2 * time.Second},
		{"in_progress", "Running production checks", 3 * time.Second},
		{"in_progress", "Deploying to production servers", 2 * time.Second},
		{"error", "Deployment failed: Database migration error", 0},
	}

	for _, status := range statuses {
		time.Sleep(status.delay)
		log.Printf("→ Status: %s - %s\n", status.state, status.description)
		
		req := &github.DeploymentStatusRequest{
			State:        github.String(status.state),
			Description:  github.String(status.description),
			AutoInactive: github.Bool(true),
		}

		if status.state == "in_progress" {
			req.LogURL = github.String(fmt.Sprintf("https://example.com/logs/pr-%d/%d", prNum, deployment.GetID()))
		}

		_, _, err := client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deployment.GetID(), req)
		if err != nil {
			log.Printf("Failed to update status: %v\n", err)
		}
	}

	log.Printf("\n✓ Created failed deployment scenario")
	log.Printf("✓ View PR to see the failed deployment status: https://github.com/%s/%s/pull/%d\n", owner, repo, prNum)
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

func createDeployment(ctx context.Context, client *github.Client, owner, repo, ref, environment, description string) (*github.Deployment, error) {
	req := &github.DeploymentRequest{
		Ref:                   github.String(ref),
		Task:                  github.String("deploy"),
		AutoMerge:             github.Bool(false),
		RequiredContexts:      &[]string{}, // Bypass status checks
		Environment:           github.String(environment),
		Description:           github.String(description),
		TransientEnvironment:  github.Bool(false),
		ProductionEnvironment: github.Bool(environment == "production"),
	}

	deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repo, req)
	return deployment, err
}