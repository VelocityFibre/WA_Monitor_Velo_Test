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

const GOOGLE_SHEETS_ID_DEBUG = "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"
const GOOGLE_CREDENTIALS_PATH_DEBUG = "./credentials.json"

func main() {
	// Check if credentials file exists
	if _, err := os.Stat(GOOGLE_CREDENTIALS_PATH_DEBUG); os.IsNotExist(err) {
		fmt.Printf("❌ Google Sheets credentials not found at %s\n", GOOGLE_CREDENTIALS_PATH_DEBUG)
		return
	}

	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH_DEBUG)
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
	
	fmt.Printf("🔍 Debugging Google Sheets row assignment for '%s' tab\n", tabName)
	fmt.Printf("📊 Checking rows 17-30 to see what's in Column A...\n\n")

	// Read rows 17-30 to see what's there
	readRange := fmt.Sprintf("%s!A17:B30", tabName)
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID_DEBUG, readRange).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌ Failed to read sheet data: %v\n", err)
		return
	}

	if len(resp.Values) == 0 {
		fmt.Printf("✅ No data found in rows 17-30 - should use row 17\n")
		return
	}

	fmt.Printf("📋 Found %d rows of data:\n", len(resp.Values))
	for i, row := range resp.Values {
		rowNum := 17 + i
		if len(row) == 0 {
			fmt.Printf("  Row %d: EMPTY (should be the target row)\n", rowNum)
			break
		} else if len(row) > 0 {
			colA := ""
			colB := ""
			if row[0] != nil {
				colA = fmt.Sprintf("%v", row[0])
			}
			if len(row) > 1 && row[1] != nil {
				colB = fmt.Sprintf("%v", row[1])
			}
			fmt.Printf("  Row %d: A='%s' B='%s'\n", rowNum, colA, colB)
		}
	}

	// Test the findFirstEmptyRow logic
	fmt.Printf("\n🧪 Testing findFirstEmptyRow logic...\n")
	startRow := 17
	for i, row := range resp.Values {
		if len(row) == 0 || row[0] == nil || row[0] == "" {
			targetRow := startRow + i
			fmt.Printf("✅ First empty row should be: %d\n", targetRow)
			return
		}
	}

	// If we get here, all rows 17-30 are filled
	fmt.Printf("⚠️  All checked rows (17-30) are filled in Column A\n")
	fmt.Printf("🔍 Let me check a wider range (17-100)...\n")

	// Check full range that the function uses
	readRange2 := fmt.Sprintf("%s!A17:A100", tabName)
	resp2, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID_DEBUG, readRange2).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌ Failed to read full range: %v\n", err)
		return
	}

	emptyCount := 0
	filledCount := 0
	
	for i, row := range resp2.Values {
		if len(row) == 0 || row[0] == nil || row[0] == "" {
			emptyCount++
			if emptyCount == 1 {
				fmt.Printf("✅ First empty row found at: %d\n", 17 + i)
			}
		} else {
			filledCount++
		}
	}

	fmt.Printf("📊 Summary for rows 17-100:\n")
	fmt.Printf("  Filled rows: %d\n", filledCount)
	fmt.Printf("  Empty rows: %d\n", emptyCount)

	if emptyCount == 0 {
		fmt.Printf("❌ ALL rows 17-100 are filled! That's why it went to row 101\n")
		fmt.Printf("💡 Solution: Either clear some old data or increase the range\n")
	}
	
	// Test the FIXED logic
	fmt.Printf("\n🔧 Testing FIXED findFirstEmptyRow logic...\n")
	startRow2 := 17
	if len(resp2.Values) == 0 {
		fmt.Printf("✅ No data found, would use row %d\n", startRow2)
	} else {
		// Check for empty rows within the data
		foundEmpty := false
		for i, row := range resp2.Values {
			if len(row) == 0 || row[0] == nil || row[0] == "" {
				fmt.Printf("✅ Empty row found within data at row %d\n", startRow2 + i)
				foundEmpty = true
				break
			}
		}
		
		if !foundEmpty {
			// All returned rows have data, next empty is after them
			nextEmptyRow := startRow2 + len(resp2.Values)
			fmt.Printf("✅ FIXED: Next empty row should be %d (after %d filled rows)\n", nextEmptyRow, len(resp2.Values))
			fmt.Printf("📍 This means row 24 should be used, not row 101!\n")
		}
	}
}