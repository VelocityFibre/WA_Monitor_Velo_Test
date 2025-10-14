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

const GOOGLE_SHEETS_ID_FIX = "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"
const GOOGLE_CREDENTIALS_PATH_FIX = "./credentials.json"

func main() {
	// Check if credentials file exists
	if _, err := os.Stat(GOOGLE_CREDENTIALS_PATH_FIX); os.IsNotExist(err) {
		fmt.Printf("❌ Google Sheets credentials not found at %s\n", GOOGLE_CREDENTIALS_PATH_FIX)
		return
	}

	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH_FIX)
	if err != nil {
		fmt.Printf("❌ Failed to read credentials file: %v\n", err)
		return
	}

	// Create Google Sheets service with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config, err := google.CredentialsFromJSON(ctx, creds, sheets.SpreadsheetsScope)
	if err != nil {
		fmt.Printf("❌ Failed to parse credentials: %v\n", err)
		return
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(config))
	if err != nil {
		fmt.Printf("❌ Failed to create sheets service: %v\n", err)
		return
	}

	tabName := "Velo Test"
	targetRow := 24
	
	fmt.Printf("🔧 Fixing row %d to have proper checkboxes...\n", targetRow)

	// Step 1: Clear row 24 completely
	clearRange := fmt.Sprintf("%s!A%d:X%d", tabName, targetRow, targetRow)
	clearReq := &sheets.ClearValuesRequest{}
	_, err = srv.Spreadsheets.Values.Clear(GOOGLE_SHEETS_ID_FIX, clearRange, clearReq).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌ Failed to clear row %d: %v\n", targetRow, err)
		return
	}
	fmt.Printf("✅ Cleared row %d\n", targetRow)

	// Step 2: Write new data with correct format
	today := "2025/10/14"
	dropNumber := "DR00000010"
	userName := "36563643842564"

	rowData := []interface{}{
		today,        // A: Date
		dropNumber,   // B: Drop Number
		"FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", // C-I: Steps 1-7 (checkboxes)
		"FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", // J-P: Steps 8-14 (checkboxes)
		0,            // Q: Completed Photos
		14,           // R: Outstanding Photos
		userName,     // S: Contractor Name
		"Processing", // T: Status
		"",           // U: QA Notes
		"",           // V: Comments  
		"FALSE",      // W: Resubmitted
		"",           // X: Additional Notes
	}

	sheetRange := fmt.Sprintf("%s!A%d:X%d", tabName, targetRow, targetRow)
	vr := &sheets.ValueRange{
		Values: [][]interface{}{rowData},
	}

	_, err = srv.Spreadsheets.Values.Update(GOOGLE_SHEETS_ID_FIX, sheetRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	if err != nil {
		fmt.Printf("❌ Failed to write corrected data: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully updated row %d with string format checkboxes\n", targetRow)
	fmt.Printf("📝 Data written: %s in row %d\n", dropNumber, targetRow)
	fmt.Printf("🔍 Please check Google Sheets to see if checkboxes appear correctly\n")
	fmt.Printf("💡 If they're still showing as text, the sheet may need data validation applied manually\n")
}