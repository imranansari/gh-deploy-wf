package main

import (
	"context"
	"flag"
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

	// Parse command line flags
	var (
		prNum = flag.Int("pr", 0, "Pull request number")
		status = flag.String("status", "success", "Final deployment status (success/failure/error)")
	)
	flag.Parse()

	if *prNum == 0 {
		log.Fatal("Please provide a PR number with -pr=<number>")
	}

	// Create GitHub client
	client, err := createGitHubClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx := context.Background()
	owner := os.Getenv("TEST_REPO_OWNER")
	repo := os.Getenv("TEST_REPO_NAME")

	// Get PR details
	pr, _, err := client.PullRequests.Get(ctx, owner, repo, *prNum)
	if err != nil {
		log.Fatalf("Failed to get PR: %v", err)
	}

	log.Printf("=== PR Dev Environment Deployment ===\n")
	log.Printf("PR #%d: %s\n", pr.GetNumber(), pr.GetTitle())
	log.Printf("Branch: %s\n", pr.GetHead().GetRef())
	log.Printf("Commit: %s\n", pr.GetHead().GetSHA()[:8])

	// Create dev deployment
	deploymentDesc := fmt.Sprintf("Deploy PR #%d to dev environment", *prNum)
	deployment, err := createDeployment(ctx, client, owner, repo, pr.GetHead().GetSHA(), "development", deploymentDesc)
	if err != nil {
		log.Fatalf("Failed to create deployment: %v", err)
	}
	
	log.Printf("\n✓ Created dev deployment (ID: %d)\n", deployment.GetID())

	// Simulate typical dev deployment workflow
	deploymentSteps := []struct {
		state       string
		description string
		delay       time.Duration
	}{
		{"queued", "Deployment queued for dev environment", 1 * time.Second},
		{"in_progress", "Building application...", 2 * time.Second},
		{"in_progress", "Running tests...", 3 * time.Second},
		{"in_progress", "Deploying to dev cluster...", 2 * time.Second},
	}

	// Execute deployment steps
	for _, step := range deploymentSteps {
		time.Sleep(step.delay)
		log.Printf("→ %s: %s\n", step.state, step.description)
		
		req := &github.DeploymentStatusRequest{
			State:        github.String(step.state),
			Description:  github.String(step.description),
			AutoInactive: github.Bool(true),
		}

		// Add log URL for in_progress states
		if step.state == "in_progress" {
			req.LogURL = github.String(fmt.Sprintf("https://ci.example.com/pr-%d/build/%d", *prNum, deployment.GetID()))
		}

		_, _, err := client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deployment.GetID(), req)
		if err != nil {
			log.Printf("Failed to update status: %v\n", err)
		}
	}

	// Final status based on flag
	time.Sleep(2 * time.Second)
	
	finalReq := &github.DeploymentStatusRequest{
		State:        github.String(*status),
		AutoInactive: github.Bool(true),
	}

	switch *status {
	case "success":
		finalReq.Description = github.String("Successfully deployed to dev environment")
		finalReq.EnvironmentURL = github.String(fmt.Sprintf("https://pr-%d.dev.example.com", *prNum))
		finalReq.LogURL = github.String(fmt.Sprintf("https://ci.example.com/pr-%d/build/%d", *prNum, deployment.GetID()))
		log.Printf("→ success: Successfully deployed to dev environment\n")
	case "failure":
		finalReq.Description = github.String("Deployment failed: Tests did not pass")
		finalReq.LogURL = github.String(fmt.Sprintf("https://ci.example.com/pr-%d/build/%d", *prNum, deployment.GetID()))
		log.Printf("→ failure: Deployment failed: Tests did not pass\n")
	case "error":
		finalReq.Description = github.String("Deployment error: Infrastructure issue")
		log.Printf("→ error: Deployment error: Infrastructure issue\n")
	}

	_, _, err = client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deployment.GetID(), finalReq)
	if err != nil {
		log.Printf("Failed to update final status: %v\n", err)
	}

	// Summary
	log.Printf("\n=== Deployment Complete ===\n")
	log.Printf("✓ PR #%d deployed to dev environment\n", *prNum)
	log.Printf("✓ Final status: %s\n", *status)
	log.Printf("✓ View PR: https://github.com/%s/%s/pull/%d\n", owner, repo, *prNum)
	
	if *status == "success" {
		log.Printf("✓ Dev environment URL: https://pr-%d.dev.example.com\n", *prNum)
	}
	
	log.Printf("\nThe deployment status is now visible on the PR!")
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
		TransientEnvironment:  github.Bool(true), // Dev environments are transient
		ProductionEnvironment: github.Bool(false),
	}

	deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repo, req)
	return deployment, err
}