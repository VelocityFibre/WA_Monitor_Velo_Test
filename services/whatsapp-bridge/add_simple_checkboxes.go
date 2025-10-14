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

func addSimpleCheckboxes() error {
	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %v", err)
	}

	// Create Google Sheets service with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config, err := google.CredentialsFromJSON(ctx, creds, sheets.SpreadsheetsScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(config))
	if err != nil {
		return fmt.Errorf("failed to create sheets service: %v", err)
	}

	// Tabs to update with simple checkbox examples
	tabs := []string{
		"Mohadin WA_Tool Monitor",
		"Lawley WA_Tool Monitor",
	}

	for _, tabName := range tabs {
		fmt.Printf("🔧 Adding example checkboxes to %s tab...\n", tabName)

		// Add sample checkboxes in rows 17-18 to demonstrate the format
		for row := 17; row <= 18; row++ {
			var rowData []interface{}

			if row == 17 {
				// Row 17: Add sample data with checkboxes (TRUE/FALSE values)
				rowData = []interface{}{
					"2025/10/10",    // A: Date
					"DR_CHECKBOX_TEST", // B: Drop Number
					true,  // C: Step 1 - checked
					false, // D: Step 2 - unchecked
					true,  // E: Step 3 - checked
					false, // F: Step 4 - unchecked
					true,  // G: Step 5 - checked
					false, // H: Step 6 - unchecked
					true,  // I: Step 7 - checked
					false, // J: Step 8 - unchecked
					true,  // K: Step 9 - checked
					false, // L: Step 10 - unchecked
					true,  // M: Step 11 - checked
					false, // N: Step 12 - unchecked
					true,  // O: Step 13 - checked
					false, // P: Step 14 - unchecked
					5,     // Q: Completed Photos
					9,     // R: Outstanding Photos
					"SampleUser", // S: User
					"Processing", // T: Status
					"",     // U: Comment
					"",     // V: Incomplete
					false,  // W: Resubmitted
					"",     // X: Additional Notes
				}
			} else {
				// Row 18: Empty checkboxes template
				rowData = []interface{}{
					"",     // A: Date
					"",     // B: Drop Number
					false,  // C: Step 1
					false,  // D: Step 2
					false,  // E: Step 3
					false,  // F: Step 4
					false,  // G: Step 5
					false,  // H: Step 6
					false,  // I: Step 7
					false,  // J: Step 8
					false,  // K: Step 9
					false,  // L: Step 10
					false,  // M: Step 11
					false,  // N: Step 12
					false,  // O: Step 13
					false,  // P: Step 14
					0,      // Q: Completed Photos
					14,     // R: Outstanding Photos
					"",     // S: User
					"",     // T: Status
					"",     // U: Comment
					"",     // V: Incomplete
					false,  // W: Resubmitted
					"",     // X: Additional Notes
				}
			}

			sheetRange := fmt.Sprintf("%s!A%d:X%d", tabName, row, row)
			vr := &sheets.ValueRange{
				Values: [][]interface{}{rowData},
			}

			_, err = srv.Spreadsheets.Values.Update(GOOGLE_SHEETS_ID, sheetRange, vr).
				ValueInputOption("USER_ENTERED").
				Context(ctx).
				Do()

			if err != nil {
				fmt.Printf("❌ Failed to add sample data to row %d in %s: %v\n", row, tabName, err)
			} else {
				fmt.Printf("   ✅ Added sample checkbox data to row %d\n", row)
			}
		}

		// Try to add basic data validation for checkboxes in column C as a test
		fmt.Printf("   🔧 Adding checkbox validation to column C...\n")

		// Get sheet info
		sheetInfo, err := srv.Spreadsheets.Get(GOOGLE_SHEETS_ID).Context(ctx).Do()
		if err != nil {
			fmt.Printf("⚠️  Could not get sheet info for %s: %v\n", tabName, err)
			continue
		}

		// Find the sheet ID
		var sheetId int64 = -1
		for _, sheet := range sheetInfo.Sheets {
			if sheet.Properties.Title == tabName {
				sheetId = sheet.Properties.SheetId
				break
			}
		}

		if sheetId == -1 {
			fmt.Printf("⚠️  Could not find sheet ID for %s\n", tabName)
			continue
		}

		// Add checkbox validation to column C only (as test)
		req := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{
				{
					SetDataValidation: &sheets.SetDataValidationRequest{
						Range: &sheets.GridRange{
							SheetId:          sheetId,
							StartColumnIndex: 2, // Column C (0-indexed)
							EndColumnIndex:   3, // Column D (exclusive)
							StartRowIndex:    16, // Row 17 (0-indexed)
							EndRowIndex:      1000,
						},
						Rule: &sheets.DataValidationRule{
							Condition: &sheets.BooleanCondition{
								Type: "BOOLEAN",
							},
							Strict:    false,
							ShowCustomUi: true,
						},
					},
				},
			},
		}

		_, err = srv.Spreadsheets.BatchUpdate(GOOGLE_SHEETS_ID, req).Context(ctx).Do()
		if err != nil {
			fmt.Printf("⚠️  Could not add checkbox validation to column C: %v\n", err)
		} else {
			fmt.Printf("   ✅ Added checkbox validation to column C\n")
		}

		fmt.Printf("✅ Completed checkbox setup for %s\n\n", tabName)
	}

	return nil
}

func main() {
	fmt.Println("🔧 Adding simple checkbox examples to Mohadin and Lawley tabs...")

	err := addSimpleCheckboxes()
	if err != nil {
		fmt.Printf("❌ Checkbox setup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Simple checkbox setup completed!")
	fmt.Println("\n📋 Added to each tab:")
	fmt.Println("   - Row 17: Sample data with checked/unchecked examples")
	fmt.Println("   - Row 18: Empty template with FALSE checkboxes")
	fmt.Println("   - Column C: Checkbox data validation")
}