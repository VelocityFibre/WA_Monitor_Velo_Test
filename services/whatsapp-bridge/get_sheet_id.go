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

const GOOGLE_SHEETS_ID_GETID = "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"
const GOOGLE_CREDENTIALS_PATH_GETID = "./credentials.json"

func main() {
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH_GETID)
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

	fmt.Printf("🔍 Getting sheet IDs...\n")

	// Get spreadsheet metadata
	spreadsheet, err := srv.Spreadsheets.Get(GOOGLE_SHEETS_ID_GETID).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌ Failed to get spreadsheet: %v\n", err)
		return
	}

	fmt.Printf("📋 Found %d sheets:\n", len(spreadsheet.Sheets))
	for i, sheet := range spreadsheet.Sheets {
		fmt.Printf("  %d. Sheet ID: %d, Title: '%s'\n", i+1, sheet.Properties.SheetId, sheet.Properties.Title)
	}

	// Find the Velo Test sheet
	var veloTestSheetId int64 = -1
	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title == "Velo Test" {
			veloTestSheetId = sheet.Properties.SheetId
			break
		}
	}

	if veloTestSheetId != -1 {
		fmt.Printf("\n✅ Found 'Velo Test' sheet with ID: %d\n", veloTestSheetId)
	} else {
		fmt.Printf("\n❌ 'Velo Test' sheet not found\n")
	}
}