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

func clearTabsBelowLine17() error {
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

	// Tabs to clear
	tabs := []string{
		"Velo Test",
		"Mohadin WA_Tool Monitor",
		"Lawley WA_Tool Monitor",
	}

	for _, tabName := range tabs {
		fmt.Printf("🧹 Clearing rows 18+ in tab: %s\n", tabName)

		// First, check current content
		readRange := fmt.Sprintf("%s!A18:X", tabName)
		resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
		if err != nil {
			fmt.Printf("⚠️  Could not read %s tab: %v\n", tabName, err)
			continue
		}

		if len(resp.Values) > 0 {
			fmt.Printf("   Found %d rows to clear in %s\n", len(resp.Values), tabName)

			// Clear all rows from 18 onwards
			clearRange := fmt.Sprintf("%s!18:1000", tabName) // Clear rows 18-1000
			clearReq := &sheets.BatchClearValuesRequest{
				Ranges: []string{clearRange},
			}

			_, err = srv.Spreadsheets.Values.BatchClear(GOOGLE_SHEETS_ID, clearReq).Context(ctx).Do()
			if err != nil {
				fmt.Printf("❌ Failed to clear %s tab: %v\n", tabName, err)
			} else {
				fmt.Printf("✅ Cleared all rows below 17 in %s tab\n", tabName)
			}
		} else {
			fmt.Printf("   No rows to clear in %s (already empty below row 17)\n", tabName)
		}
	}

	return nil
}

func main() {
	fmt.Println("🧹 Clearing Google Sheets tabs below row 17...")

	err := clearTabsBelowLine17()
	if err != nil {
		fmt.Printf("❌ Clear operation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Clear operation completed!")
}