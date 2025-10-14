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

func addCheckboxesFromRow1() error {
	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %v", err)
	}

	// Create Google Sheets service with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config, err := google.CredentialsFromJSON(ctx, creds, sheets.SpreadsheetsScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(config))
	if err != nil {
		return fmt.Errorf("failed to create sheets service: %v", err)
	}

	// Get sheet info and IDs
	sheetInfo, err := srv.Spreadsheets.Get(GOOGLE_SHEETS_ID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get sheet info: %v", err)
	}

	// Find sheet IDs for both tabs
	sheetIds := make(map[string]int64)
	for _, sheet := range sheetInfo.Sheets {
		if sheet.Properties.Title == "Mohadin WA_Tool Monitor" || sheet.Properties.Title == "Lawley WA_Tool Monitor" {
			sheetIds[sheet.Properties.Title] = sheet.Properties.SheetId
		}
	}

	// Columns to update (C-P = indices 2-15)
	columns := []struct {
		name string
		index int64
	}{
		{"C", 2}, {"D", 3}, {"E", 4}, {"F", 5}, {"G", 6}, {"H", 7}, {"I", 8}, {"J", 9},
		{"K", 10}, {"L", 11}, {"M", 12}, {"N", 13}, {"O", 14}, {"P", 15},
	}

	// Tabs to update
	tabs := []string{"Mohadin WA_Tool Monitor", "Lawley WA_Tool Monitor"}

	for _, tabName := range tabs {
		sheetId, exists := sheetIds[tabName]
		if !exists {
			fmt.Printf("❌ Could not find sheet ID for %s\n", tabName)
			continue
		}

		fmt.Printf("🔧 Adding checkbox validation from row 1 in %s...\n", tabName)

		// Add validation for each column from row 1 to row 1000
		for _, col := range columns {
			req := &sheets.BatchUpdateSpreadsheetRequest{
				Requests: []*sheets.Request{
					{
						SetDataValidation: &sheets.SetDataValidationRequest{
							Range: &sheets.GridRange{
								SheetId:          sheetId,
								StartColumnIndex: col.index,
								EndColumnIndex:   col.index + 1,
								StartRowIndex:    0, // Row 1 (0-indexed)
								EndRowIndex:      1000,
							},
							Rule: &sheets.DataValidationRule{
								Condition: &sheets.BooleanCondition{
									Type: "BOOLEAN",
								},
								Strict:    false,
								ShowCustomUi: true,
								InputMessage: "Click to check/uncheck",
							},
						},
					},
				},
			}

			_, err = srv.Spreadsheets.BatchUpdate(GOOGLE_SHEETS_ID, req).Context(ctx).Do()
			if err != nil {
				fmt.Printf("⚠️  Could not add checkbox validation to column %s: %v\n", col.name, err)
			} else {
				fmt.Printf("   ✅ Added checkbox validation to column %s (rows 1-1000)\n", col.name)
			}

			// Small delay to avoid rate limiting
			time.Sleep(200 * time.Millisecond)
		}

		fmt.Printf("✅ Completed checkbox validation for all columns in %s\n\n", tabName)
	}

	return nil
}

func main() {
	fmt.Println("🔧 Adding checkbox validation from row 1 in Mohadin and Lawley tabs...")

	err := addCheckboxesFromRow1()
	if err != nil {
		fmt.Printf("❌ Checkbox validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Checkbox validation from row 1 completed!")
	fmt.Println("\n📋 Checkbox validation added to:")
	fmt.Println("   - Mohadin WA_Tool Monitor: Columns C-P, Rows 1-1000")
	fmt.Println("   - Lawley WA_Tool Monitor: Columns C-P, Rows 1-1000")
	fmt.Println("\n💡 Now checkboxes are available from the very first row!")
}