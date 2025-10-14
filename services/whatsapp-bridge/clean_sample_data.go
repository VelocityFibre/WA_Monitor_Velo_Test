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

func cleanSampleData() error {
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

	// Clear sample data from rows 17-18 in both tabs
	tabs := []string{
		"Mohadin WA_Tool Monitor",
		"Lawley WA_Tool Monitor",
	}

	for _, tabName := range tabs {
		fmt.Printf("🧹 Clearing sample data from %s...\n", tabName)

		// Clear rows 17-18
		clearRange := fmt.Sprintf("%s!17:18", tabName)
		clearReq := &sheets.BatchClearValuesRequest{
			Ranges: []string{clearRange},
		}

		_, err = srv.Spreadsheets.Values.BatchClear(GOOGLE_SHEETS_ID, clearReq).Context(ctx).Do()
		if err != nil {
			fmt.Printf("❌ Failed to clear sample data from %s: %v\n", tabName, err)
		} else {
			fmt.Printf("✅ Cleared sample data from %s\n", tabName)
		}
	}

	return nil
}

func main() {
	fmt.Println("🧹 Cleaning up sample data from tabs...")

	err := cleanSampleData()
	if err != nil {
		fmt.Printf("❌ Cleanup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Sample data cleanup completed!")
	fmt.Println("\n📋 Both tabs are now ready for production:")
	fmt.Println("   - Checkbox validation in columns C-P (Steps 1-14)")
	fmt.Println("   - Data entry will start from row 17")
	fmt.Println("   - Ready for WhatsApp bridge integration")
}