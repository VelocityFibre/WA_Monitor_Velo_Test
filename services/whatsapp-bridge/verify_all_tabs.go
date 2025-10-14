package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	GOOGLE_SHEETS_ID        = "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"
	GOOGLE_CREDENTIALS_PATH = "/app/credentials.json"
)

func verifyAllTabs() error {
	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %v", err)
	}

	// Create Google Sheets service with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config, err := google.CredentialsFromJSON(ctx, creds, sheets.SpreadsheetsScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(config))
	if err != nil {
		return fmt.Errorf("failed to create sheets service: %v", err)
	}

	// All tabs to verify
	tabs := []string{
		"Velo Test",
		"Mohadin WA_Tool Monitor",
		"Lawley WA_Tool Monitor",
	}

	expectedColumns := 24
	expectedSteps := 14

	fmt.Printf("🔍 Verifying all three tabs have identical checkbox structures...\n\n")

	for _, tabName := range tabs {
		// Read header row
		readRange := fmt.Sprintf("%s!1:1", tabName)
		resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
		if err != nil {
			fmt.Printf("❌ Failed to read %s headers: %v\n", tabName, err)
			continue
		}

		if len(resp.Values) == 0 {
			fmt.Printf("❌ No headers found in %s tab\n", tabName)
			continue
		}

		headers := resp.Values[0]
		actualColumns := len(headers)

		// Count step columns (should be 14 steps from columns C-P)
		stepColumns := 0
		if actualColumns >= 3 {
			for i := 2; i < 16 && i < actualColumns; i++ { // Columns C-P (indices 2-15)
				if headers[i] != nil && fmt.Sprintf("%v", headers[i]) != "" {
					stepColumns++
				}
			}
		}

		// Check structure
		status := "✅"
		if actualColumns != expectedColumns {
			status = "❌"
		} else if stepColumns != expectedSteps {
			status = "⚠️"
		}

		fmt.Printf("%s %s:\n", status, tabName)
		fmt.Printf("   📊 Columns: %d (expected: %d)\n", actualColumns, expectedColumns)
		fmt.Printf("   📋 Steps: %d (expected: %d)\n", stepColumns, expectedSteps)

		if actualColumns == expectedColumns {
			fmt.Printf("   🏗️  Structure: A-B: Date/Drop, C-P: Steps 1-14, Q-X: Tracking\n")
		}

		fmt.Printf("\n")
	}

	return nil
}

func main() {
	fmt.Println("🔍 Verifying checkbox structures for all three tabs...")

	err := verifyAllTabs()
	if err != nil {
		fmt.Printf("❌ Verification failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Tab structure verification completed!")
}