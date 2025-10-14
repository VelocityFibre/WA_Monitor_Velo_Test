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

func testDREntry() error {
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

	// Test adding a DR entry to Mohadin WA_Tool Monitor
	tabName := "Mohadin WA_Tool Monitor"
	dropNumber := "DR_TEST_123"
	userName := "TestUser"
	today := time.Now().Format("2006/01/02")

	fmt.Printf("🧪 Testing DR entry to %s tab...\n", tabName)

	// Find first empty row starting from row 17
	startRow := 17
	readRange := fmt.Sprintf("%s!A%d:A%d", tabName, startRow, startRow + 10)
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read rows to find empty spot: %v", err)
	}

	targetRow := startRow
	if len(resp.Values) > 0 {
		// Find first empty row
		for i, row := range resp.Values {
			if len(row) == 0 || row[0] == nil || row[0] == "" {
				targetRow = startRow + i
				break
			}
		}
		if targetRow == startRow {
			targetRow = startRow + len(resp.Values)
		}
	}

	fmt.Printf("📍 Writing test entry to row %d\n", targetRow)

	// Create test data matching the 24-column structure
	rowData := []interface{}{
		today,        // A: Date
		dropNumber,   // B: Drop Number
		false, false, false, false, false, false, false, // C-I: Steps 1-7 (checkboxes)
		false, false, false, false, false, false, false, // J-P: Steps 8-14 (checkboxes)
		0,            // Q: Completed Photos
		14,           // R: Outstanding Photos
		userName,     // S: Contractor Name
		"Processing", // T: Status
		"",           // U: QA Notes
		"",           // V: Comments
		false,        // W: Resubmitted
		"",           // X: Additional Notes
	}

	sheetRange := fmt.Sprintf("%s!A%d:X%d", tabName, targetRow, targetRow)
	vr := &sheets.ValueRange{
		Values: [][]interface{}{rowData},
	}

	_, err = srv.Spreadsheets.Values.Update(GOOGLE_SHEETS_ID, sheetRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("failed to write test entry: %v", err)
	}

	fmt.Printf("✅ Test DR entry added successfully to row %d\n", targetRow)

	// Verify the entry was written
	verifyRange := fmt.Sprintf("%s!A%d:X%d", tabName, targetRow, targetRow)
	verifyResp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, verifyRange).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to verify entry: %v", err)
	}

	if len(verifyResp.Values) > 0 {
		verifyRow := verifyResp.Values[0]
		actualDate := safeGet(verifyRow, 0)
		actualDR := safeGet(verifyRow, 1)
		actualUser := safeGet(verifyRow, 18)

		fmt.Printf("✅ Verification successful:\n")
		fmt.Printf("   Row %d: Date=%s, DR=%s, User=%s\n", targetRow, actualDate, actualDR, actualUser)
	} else {
		return fmt.Errorf("verification failed - no data found at row %d", targetRow)
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
	fmt.Println("🧪 Testing DR entry to Mohadin WA_Tool Monitor tab...")

	err := testDREntry()
	if err != nil {
		fmt.Printf("❌ Test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Test completed successfully!")
}