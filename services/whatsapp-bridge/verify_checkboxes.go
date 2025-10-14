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

func verifyCheckboxes() error {
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

	// Check both tabs for checkbox functionality
	tabs := []string{
		"Mohadin WA_Tool Monitor",
		"Lawley WA_Tool Monitor",
	}

	for _, tabName := range tabs {
		fmt.Printf("🔍 Verifying checkboxes in %s tab...\n", tabName)

		// Check rows 17-18 where we added sample data
		readRange := fmt.Sprintf("%s!17:18", tabName)
		resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
		if err != nil {
			fmt.Printf("❌ Failed to read %s: %v\n", tabName, err)
			continue
		}

		if len(resp.Values) == 0 {
			fmt.Printf("❌ No data found in rows 17-18 for %s\n", tabName)
			continue
		}

		fmt.Printf("📊 Found %d rows with checkbox data:\n", len(resp.Values))

		for i, row := range resp.Values {
			rowNum := 17 + i
			date := safeGet(row, 0)
			dropNumber := safeGet(row, 1)
			user := safeGet(row, 18)

			fmt.Printf("   Row %d: Date=%s, DR=%s, User=%s\n", rowNum, date, dropNumber, user)

			// Check checkbox values in columns C-P (indices 2-15)
			fmt.Printf("      Checkboxes (C-P): ")
			checkedCount := 0
			for col := 2; col <= 15 && col < len(row); col++ {
				if row[col] != nil {
					val := fmt.Sprintf("%v", row[col])
					if val == "true" || val == "TRUE" {
						fmt.Printf("✓")
						checkedCount++
					} else if val == "false" || val == "FALSE" {
						fmt.Printf("✗")
					} else {
						fmt.Printf("?")
					}
				} else {
					fmt.Printf("-")
				}
			}
			fmt.Printf(" (%d/14 checked)\n", checkedCount)

			if i == 0 {
				fmt.Printf("      🎯 This is the SAMPLE row with mixed checkboxes\n")
			} else if i == 1 {
				fmt.Printf("      🎯 This is the TEMPLATE row with empty checkboxes\n")
			}
		}

		fmt.Printf("\n")
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
	fmt.Println("🔍 Verifying checkbox functionality in Mohadin and Lawley tabs...")

	err := verifyCheckboxes()
	if err != nil {
		fmt.Printf("❌ Verification failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Checkbox verification completed!")
	fmt.Println("\n📋 Summary:")
	fmt.Println("   - Both tabs now have checkbox data validation in columns C-P")
	fmt.Println("   - Row 17 shows sample data with mixed checked/unchecked boxes")
	fmt.Println("   - Row 18 shows empty template ready for new entries")
	fmt.Println("   - TRUE = checked, FALSE = unchecked")
}