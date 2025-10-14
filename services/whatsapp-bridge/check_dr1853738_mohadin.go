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

func checkDR1853738() error {
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

	// Check Mohadin WA_Tool Monitor tab for DR1853738
	fmt.Printf("🔍 Checking Mohadin WA_Tool Monitor tab for DR1853738...\n")

	// Read rows 17-50 where new data should be
	readRange := "Mohadin WA_Tool Monitor!17:50"
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read Mohadin WA_Tool Monitor tab: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Printf("❌ No data found in rows 17-50\n")
	} else {
		fmt.Printf("📊 Found %d rows in range 17-50:\n", len(resp.Values))
		found := false

		for i, row := range resp.Values {
			rowNum := 17 + i
			date := safeGet(row, 0)
			dropNumber := safeGet(row, 1)
			user := safeGet(row, 17) // Column R (User)
			status := safeGet(row, 19) // Column T (Status)

			fmt.Printf("   Row %d: Date=%s, DR=%s, User=%s, Status=%s\n", rowNum, date, dropNumber, user, status)

			if dropNumber == "DR1853738" {
				found = true
				fmt.Printf("      🎯 FOUND DR1853738! Full row details:\n")
				for j, cell := range row {
					if j < 26 { // Show all 26 columns
						colName := string(rune('A' + j))
						fmt.Printf("         Column %s: %v\n", colName, cell)
					}
				}
				break
			}
		}

		if !found {
			fmt.Printf("❌ DR1853738 NOT FOUND in rows 17-50\n")
		}
	}

	// Also check if there's data anywhere else in the sheet
	fmt.Printf("\n🔍 Checking entire Mohadin WA_Tool Monitor tab...\n")
	readRange2 := "Mohadin WA_Tool Monitor!A:Z"
	resp2, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange2).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read entire Mohadin tab: %v", err)
	}

	totalRows := len(resp2.Values)
	fmt.Printf("📊 Total rows in Mohadin WA_Tool Monitor: %d\n", totalRows)

	if totalRows > 16 {
		fmt.Printf("📋 Data rows (17+): %d\n", totalRows-16)

		// Show the most recent entries
		start := 17
		if totalRows > 22 {
			start = totalRows - 5 // Show last 5 rows
		}

		fmt.Printf("Recent entries:\n")
		for i := start - 1; i < totalRows && i < start + 4; i++ {
			row := resp2.Values[i]
			rowNum := i + 1
			dropNumber := safeGet(row, 1)
			user := safeGet(row, 17)

			fmt.Printf("   Row %d: %s | User: %s", rowNum, dropNumber, user)
			if dropNumber == "DR1853738" {
				fmt.Printf(" ⭐ FOUND DR1853738!")
			}
			fmt.Printf("\n")
		}
	} else {
		fmt.Printf("ℹ️  No data below row 16\n")
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
	fmt.Println("🔍 Checking for DR1853738 in Mohadin WA_Tool Monitor tab...")

	err := checkDR1853738()
	if err != nil {
		fmt.Printf("❌ Check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Check completed!")
}