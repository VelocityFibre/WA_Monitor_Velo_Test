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

func checkEntireVeloTest() error {
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

	// Check entire Velo Test tab for any DR entries
	fmt.Printf("🔍 Checking entire Velo Test tab for DR0000001 and DR0000005...\n")

	// Read all data in the tab
	readRange := "Velo Test!A:X"
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read Velo Test tab: %v", err)
	}

	totalRows := len(resp.Values)
	fmt.Printf("📊 Total rows in Velo Test tab: %d\n", totalRows)

	if totalRows == 0 {
		fmt.Printf("❌ No data found in Velo Test tab\n")
		return nil
	}

	foundDRs := 0
	// Show header and recent entries
	fmt.Printf("\n📋 Header row (Row 1):\n")
	if len(resp.Values) > 0 {
		headerRow := resp.Values[0]
		dropNumber := safeGet(headerRow, 1)
		fmt.Printf("   Column B: %s\n", dropNumber)
	}

	fmt.Printf("\n🔍 Searching for DR0000001 and DR0000005...\n")

	// Check all rows for our target DR numbers
	for i, row := range resp.Values {
		rowNum := i + 1
		dropNumber := safeGet(row, 1)

		if dropNumber == "DR0000001" || dropNumber == "DR0000005" {
			foundDRs++
			date := safeGet(row, 0)
			user := safeGet(row, 18)
			status := safeGet(row, 19)

			fmt.Printf("   🎯 FOUND %s at Row %d: Date=%s, User=%s, Status=%s\n",
				dropNumber, rowNum, date, user, status)

			// Show checkbox values for this row
			fmt.Printf("      📋 Checkboxes (C-P): ")
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
						fmt.Printf("-")
					}
				} else {
					fmt.Printf("-")
				}
			}
			fmt.Printf(" (%d/14 checked)\n", checkedCount)
		}
	}

	if foundDRs == 0 {
		fmt.Printf("❌ DR0000001 and DR0000005 not found in any row\n")

		// Show last few entries to see what's there
		fmt.Printf("\n📊 Last 5 entries in Velo Test:\n")
		start := totalRows - 5
		if start < 0 {
			start = 0
		}

		for i := start; i < totalRows; i++ {
			row := resp.Values[i]
			rowNum := i + 1
			dropNumber := safeGet(row, 1)
			user := safeGet(row, 18)

			fmt.Printf("   Row %d: %s | User: %s\n", rowNum, dropNumber, user)
		}
	} else {
		fmt.Printf("\n✅ Found %d DR entries that were processed from WhatsApp\n", foundDRs)
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
	fmt.Println("🔍 Checking entire Velo Test tab for processed DR numbers...")

	err := checkEntireVeloTest()
	if err != nil {
		fmt.Printf("❌ Check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Entire Velo Test tab check completed!")
}