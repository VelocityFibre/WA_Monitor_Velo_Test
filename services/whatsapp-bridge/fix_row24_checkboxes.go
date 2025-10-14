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

const GOOGLE_SHEETS_ID_FIX24 = "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"
const GOOGLE_CREDENTIALS_PATH_FIX24 = "./credentials.json"

func main() {
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH_FIX24)
	if err != nil {
		fmt.Printf("❌ Failed to read credentials file: %v\n", err)
		return
	}

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

	fmt.Printf("🔧 Fixing row 24 checkbox formatting (one-time fix)...\n")

	targetRow := 24

	// Copy checkbox data validation from row 17 to row 24
	
	// Source: Row 17 columns C-P (checkbox columns)
	sourceRange := &sheets.GridRange{
		SheetId:          1654167750, // Velo Test sheet ID
		StartRowIndex:    16, // Row 17 (0-based)
		EndRowIndex:      17, // Row 17 (exclusive end)
		StartColumnIndex: 2,  // Column C (0-based)
		EndColumnIndex:   16, // Column P (exclusive end)
	}
	
	// Destination: Row 24 columns C-P
	destinationRange := &sheets.GridRange{
		SheetId:          1654167750,
		StartRowIndex:    int64(targetRow - 1), // Row 24 (0-based)
		EndRowIndex:      int64(targetRow),     // Exclusive end
		StartColumnIndex: 2,                    // Column C
		EndColumnIndex:   16,                   // Column P
	}
	
	// Source: Row 17 column W (Resubmitted checkbox)
	sourceRangeW := &sheets.GridRange{
		SheetId:          1654167750,
		StartRowIndex:    16, // Row 17
		EndRowIndex:      17,
		StartColumnIndex: 22, // Column W (0-based)
		EndColumnIndex:   23,
	}
	
	// Destination: Row 24 column W
	destinationRangeW := &sheets.GridRange{
		SheetId:          1654167750,
		StartRowIndex:    int64(targetRow - 1), // Row 24 (0-based)
		EndRowIndex:      int64(targetRow),
		StartColumnIndex: 22, // Column W
		EndColumnIndex:   23,
	}
	
	// Create batch request to copy formatting for both ranges
	batchRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				CopyPaste: &sheets.CopyPasteRequest{
					Source:      sourceRange,
					Destination: destinationRange,
					PasteType:   "PASTE_DATA_VALIDATION", // Only copy validation rules
				},
			},
			{
				CopyPaste: &sheets.CopyPasteRequest{
					Source:      sourceRangeW,
					Destination: destinationRangeW,
					PasteType:   "PASTE_DATA_VALIDATION",
				},
			},
		},
	}
	
	_, err = srv.Spreadsheets.BatchUpdate(GOOGLE_SHEETS_ID_FIX24, batchRequest).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌ Failed to copy checkbox formatting: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Successfully copied checkbox formatting to row %d\n", targetRow)
	fmt.Printf("🔍 Please check Google Sheets - row 24 should now have proper checkboxes!\n")
	fmt.Printf("🚀 Future drops will automatically get proper checkboxes too!\n")
}