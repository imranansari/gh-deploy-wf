package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v58/github"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Parse configuration
	appID, err := strconv.ParseInt(os.Getenv("GITHUB_APP_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Invalid GITHUB_APP_ID: %v", err)
	}

	installationID, err := strconv.ParseInt(os.Getenv("GITHUB_INSTALLATION_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Invalid GITHUB_INSTALLATION_ID: %v", err)
	}

	privateKeyPath := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	fmt.Printf("App ID: %d\n", appID)
	fmt.Printf("Installation ID: %d\n", installationID)
	fmt.Printf("Private Key Path: %s\n", privateKeyPath)

	// Read private key
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}
	fmt.Printf("Private key loaded: %d bytes\n", len(privateKey))

	// Create transport - try with explicit options
	fmt.Println("\nCreating installation transport...")
	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		log.Fatalf("Failed to create installation transport: %v", err)
	}
	fmt.Println("✓ Transport created")

	// Create client
	client := github.NewClient(&http.Client{Transport: itr})
	ctx := context.Background()

	// Try a simple API call
	fmt.Println("\nTesting API access...")
	
	// First, let's try to get the authenticated app
	fmt.Println("Getting authenticated app info...")
	app, resp, err := client.Apps.Get(ctx, "")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		if resp != nil {
			fmt.Printf("Response Status: %s\n", resp.Status)
			fmt.Printf("Rate Limit: %d/%d\n", resp.Rate.Remaining, resp.Rate.Limit)
		}
	} else {
		fmt.Printf("✓ Authenticated as: %s\n", app.GetName())
	}

	// Try to get installation info
	fmt.Println("\nGetting installation info...")
	inst, resp, err := client.Apps.GetInstallation(ctx, installationID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		if resp != nil {
			fmt.Printf("Response Status: %s\n", resp.Status)
		}
	} else {
		fmt.Printf("✓ Installation for: %s\n", inst.GetAccount().GetLogin())
	}

	// Try to access the test repository
	owner := os.Getenv("TEST_REPO_OWNER")
	repo := os.Getenv("TEST_REPO_NAME")
	fmt.Printf("\nAccessing repository %s/%s...\n", owner, repo)
	
	repoInfo, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		if resp != nil {
			fmt.Printf("Response Status: %s\n", resp.Status)
		}
		return
	}
	
	fmt.Printf("✓ Repository: %s\n", repoInfo.GetFullName())
	fmt.Printf("  Description: %s\n", repoInfo.GetDescription())
	fmt.Printf("  Default Branch: %s\n", repoInfo.GetDefaultBranch())
	fmt.Printf("  Private: %v\n", repoInfo.GetPrivate())

	fmt.Println("\n✓ Authentication successful! Ready to create deployments.")
}