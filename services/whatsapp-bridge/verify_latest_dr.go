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

func verifyLatestDRs() error {
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

	// Check Mohadin WA_Tool Monitor tab for the latest DR numbers
	fmt.Printf("🔍 Checking Mohadin WA_Tool Monitor tab for latest DR entries...\n")

	// Read rows 17-25 where new data should be
	readRange := "Mohadin WA_Tool Monitor!17:25"
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read Mohadin WA_Tool Monitor tab: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Printf("❌ No data found in rows 17-25\n")
	} else {
		fmt.Printf("📊 Found %d rows in range 17-25:\n", len(resp.Values))

		for i, row := range resp.Values {
			rowNum := 17 + i
			date := safeGet(row, 0)
			dropNumber := safeGet(row, 1)
			user := safeGet(row, 18) // Column S (User)
			status := safeGet(row, 19) // Column T (Status)

			fmt.Printf("   Row %d: Date=%s, DR=%s, User=%s, Status=%s\n", rowNum, date, dropNumber, user, status)

			// Check for the DR numbers we just processed
			if dropNumber == "DR1853803" || dropNumber == "DR1854483" || dropNumber == "DR1854544" {
				fmt.Printf("      🎯 FOUND LATEST DR ENTRY! ✅\n")
			}
		}
	}

	return nil
}

func safeGet(row []interface{}, index int) string {
	if index >= len(row) {
		return ""
	}
	if row[index] == nil {
		return ""
	}
	return fmt.Sprintf("%v", row[index])
}

func main() {
	fmt.Println("🔍 Verifying latest DR numbers in Mohadin WA_Tool Monitor tab...")

	err := verifyLatestDRs()
	if err != nil {
		fmt.Printf("❌ Verification failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Latest DR verification completed!")
}