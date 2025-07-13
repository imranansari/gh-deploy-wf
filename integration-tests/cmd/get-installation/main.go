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
		log.Println("No .env file found, using environment variables")
	}

	// Get app ID
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		log.Fatal("GITHUB_APP_ID not set")
	}
	
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid GITHUB_APP_ID: %v", err)
	}

	privateKeyPath := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		log.Fatal("GITHUB_PRIVATE_KEY_PATH not set")
	}

	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}

	// Create app transport (not installation transport)
	atr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appID, privateKey)
	if err != nil {
		log.Fatalf("Failed to create app transport: %v", err)
	}

	client := github.NewClient(&http.Client{Transport: atr})
	ctx := context.Background()

	// Get app info
	app, _, err := client.Apps.Get(ctx, "")
	if err != nil {
		log.Fatalf("Failed to get app info: %v", err)
	}
	
	fmt.Printf("GitHub App: %s (ID: %d)\n", app.GetName(), app.GetID())
	fmt.Printf("Description: %s\n", app.GetDescription())
	fmt.Println("\n=== Installations ===")

	// List installations
	installations, _, err := client.Apps.ListInstallations(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list installations: %v", err)
	}

	if len(installations) == 0 {
		fmt.Println("No installations found")
		return
	}

	for _, inst := range installations {
		fmt.Printf("\nInstallation ID: %d\n", inst.GetID())
		fmt.Printf("Account: %s (%s)\n", inst.GetAccount().GetLogin(), inst.GetAccount().GetType())
		fmt.Printf("Repository Selection: %s\n", inst.GetRepositorySelection())
		fmt.Printf("Created: %s\n", inst.GetCreatedAt().Format("2006-01-02"))
		
		// Try to list some accessible repos
		if inst.GetRepositorySelection() != "all" {
			itr, err := ghinstallation.New(http.DefaultTransport, appID, inst.GetID(), privateKey)
			if err != nil {
				continue
			}
			
			instClient := github.NewClient(&http.Client{Transport: itr})
			
			// List first few repositories
			repos, _, err := instClient.Apps.ListRepos(ctx, &github.ListOptions{PerPage: 5})
			if err == nil && repos.TotalCount != nil && *repos.TotalCount > 0 {
				fmt.Printf("Accessible repositories (%d total):\n", *repos.TotalCount)
				for _, repo := range repos.Repositories {
					fmt.Printf("  - %s\n", repo.GetFullName())
				}
			}
		}
		
		fmt.Println("---")
	}

	fmt.Printf("\nAdd the Installation ID to your .env file:\n")
	fmt.Printf("GITHUB_INSTALLATION_ID=<installation-id>\n")
}