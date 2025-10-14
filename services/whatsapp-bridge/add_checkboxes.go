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

func addCheckboxesToTabs() error {
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

	// Tabs to update with checkboxes
	tabs := []string{
		"Mohadin WA_Tool Monitor",
		"Lawley WA_Tool Monitor",
	}

	for _, tabName := range tabs {
		fmt.Printf("🔧 Adding checkbox formatting to %s tab...\n", tabName)

		// Add checkbox data validation to columns C-P (steps 1-14)
		for col := 'C'; col <= 'P'; col++ {
			// Create data validation rule for checkbox
			dataValidation := &sheets.DataValidationRule{
				Condition: &sheets.BooleanCondition{
					Type: "BOOLEAN",
				},
				Strict:    true,
				ShowCustomUi: true,
				InputMessage: "Click to check/uncheck",
			}

			// Create the request to add data validation
			req := &sheets.BatchUpdateSpreadsheetRequest{
				Requests: []*sheets.Request{
					{
						SetDataValidation: &sheets.SetDataValidationRequest{
							Range: &sheets.GridRange{
								SheetId:          0, // Will be updated dynamically
								StartColumnIndex: int64(col - 'A'),
								EndColumnIndex:   int64(col - 'A' + 1),
								StartRowIndex:    16, // Row 17 (0-indexed)
								EndRowIndex:      1000,
							},
							Rule: dataValidation,
						},
					},
				},
			}

			// Get the sheet ID first
			sheetInfo, err := srv.Spreadsheets.Get(GOOGLE_SHEETS_ID).Context(ctx).Do()
			if err != nil {
				fmt.Printf("❌ Failed to get sheet info for %s: %v\n", tabName, err)
				continue
			}

			// Find the sheet ID for the current tab
			var sheetId int64 = -1
			for _, sheet := range sheetInfo.Sheets {
				if sheet.Properties.Title == tabName {
					sheetId = sheet.Properties.SheetId
					break
				}
			}

			if sheetId == -1 {
				fmt.Printf("❌ Could not find sheet ID for %s\n", tabName)
				continue
			}

			// Update the request with correct sheet ID
			req.Requests[0].SetDataValidation.Range.SheetId = sheetId

			// Execute the request
			_, err = srv.Spreadsheets.BatchUpdate(GOOGLE_SHEETS_ID, req).Context(ctx).Do()
			if err != nil {
				fmt.Printf("⚠️  Could not add checkbox to column %c in %s: %v\n", col, tabName, err)
			} else {
				fmt.Printf("   ✅ Added checkbox to column %c\n", col)
			}
		}

		fmt.Printf("✅ Completed checkbox formatting for %s\n\n", tabName)
	}

	return nil
}

func main() {
	fmt.Println("🔧 Adding checkbox formatting to Mohadin and Lawley tabs...")

	err := addCheckboxesToTabs()
	if err != nil {
		fmt.Printf("❌ Checkbox addition failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Checkbox formatting completed!")
	fmt.Println("\n📋 Checkboxes added to:")
	fmt.Println("   - Mohadin WA_Tool Monitor: Columns C-P (Steps 1-14)")
	fmt.Println("   - Lawley WA_Tool Monitor: Columns C-P (Steps 1-14)")
}