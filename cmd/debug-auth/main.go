package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	privateKeyPath := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		log.Fatal("GITHUB_PRIVATE_KEY_PATH not set")
	}

	// Read private key
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("Failed to read private key: %v", err)
	}

	fmt.Printf("Private key file size: %d bytes\n", len(privateKeyData))
	fmt.Printf("First line: %s...\n", string(privateKeyData[:27]))

	// Parse PEM block
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		log.Fatal("Failed to parse PEM block")
	}

	fmt.Printf("PEM Type: %s\n", block.Type)
	fmt.Printf("PEM Headers: %v\n", block.Headers)

	// Try to parse the private key
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		keyInterface, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			log.Fatalf("Failed to parse private key:\nPKCS1 error: %v\nPKCS8 error: %v", err, err2)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			log.Fatal("Private key is not RSA")
		}
		fmt.Println("Successfully parsed as PKCS8")
	} else {
		fmt.Println("Successfully parsed as PKCS1")
	}

	fmt.Printf("Key size: %d bits\n", key.Size()*8)
	
	// Try to create a simple JWT
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		log.Fatal("GITHUB_APP_ID not set")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(key)
	if err != nil {
		log.Fatalf("Failed to sign token: %v", err)
	}

	fmt.Println("\nSuccessfully created JWT!")
	fmt.Printf("Token (first 50 chars): %s...\n", tokenString[:50])
	fmt.Println("\nThe private key appears to be valid.")
}