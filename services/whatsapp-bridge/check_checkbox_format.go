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

const GOOGLE_SHEETS_ID_CHECK = "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"
const GOOGLE_CREDENTIALS_PATH_CHECK = "./credentials.json"

func main() {
	// Check if credentials file exists
	if _, err := os.Stat(GOOGLE_CREDENTIALS_PATH_CHECK); os.IsNotExist(err) {
		fmt.Printf("❌ Google Sheets credentials not found at %s\n", GOOGLE_CREDENTIALS_PATH_CHECK)
		return
	}

	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH_CHECK)
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
	
	fmt.Printf("🔍 Checking checkbox format in existing rows...\n")
	fmt.Printf("📊 Looking at row 17 (first data row) for checkbox format\n\n")

	// Read row 17 completely to see the exact format
	readRange := fmt.Sprintf("%s!C17:P17", tabName) // Checkbox columns C-P
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID_CHECK, readRange).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌ Failed to read sheet data: %v\n", err)
		return
	}

	if len(resp.Values) == 0 {
		fmt.Printf("❌ No data found in row 17\n")
		return
	}

	row := resp.Values[0]
	fmt.Printf("📋 Row 17 checkbox values (Columns C-P):\n")
	for i, value := range row {
		column := string(rune('C' + i))
		fmt.Printf("  Column %s: %v (type: %T)\n", column, value, value)
	}

	// Also check DR00000010 row (row 24)
	fmt.Printf("\n🔍 Checking DR00000010 row (row 24)...\n")
	readRange2 := fmt.Sprintf("%s!A24:X24", tabName)
	resp2, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID_CHECK, readRange2).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌ Failed to read row 24: %v\n", err)
		return
	}

	if len(resp2.Values) > 0 {
		row24 := resp2.Values[0]
		fmt.Printf("📋 Row 24 (DR00000010) - All columns:\n")
		columns := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X"}
		for i, value := range row24 {
			if i < len(columns) {
				fmt.Printf("  Column %s: %v (type: %T)\n", columns[i], value, value)
			}
		}
	} else {
		fmt.Printf("❌ No data found in row 24\n")
	}
}