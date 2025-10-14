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

func checkVeloCheckboxes() error {
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

	// Check Velo Test tab structure
	fmt.Printf("🔍 Examining Velo Test tab checkbox structure...\n")

	// Read header row (row 1 should contain all column headers)
	readRange := "Velo Test!1:1"
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read Velo Test headers: %v", err)
	}

	if len(resp.Values) == 0 {
		return fmt.Errorf("no headers found in Velo Test tab")
	}

	headers := resp.Values[0]
	fmt.Printf("📊 Velo Test has %d columns:\n", len(headers))

	for i, header := range headers {
		colName := string(rune('A' + i))
		fmt.Printf("   Column %s (%d): %v\n", colName, i+1, header)
	}

	// Also check Mohadin current structure for comparison
	fmt.Printf("\n🔍 Examining Mohadin WA_Tool Monitor current structure...\n")
	mohadinRange := "Mohadin WA_Tool Monitor!1:1"
	mohadinResp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, mohadinRange).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read Mohadin headers: %v", err)
	}

	if len(mohadinResp.Values) > 0 {
		mohadinHeaders := mohadinResp.Values[0]
		fmt.Printf("📊 Mohadin WA_Tool Monitor has %d columns:\n", len(mohadinHeaders))
		for i, header := range mohadinHeaders {
			colName := string(rune('A' + i))
			fmt.Printf("   Column %s (%d): %v\n", colName, i+1, header)
		}
	}

	return nil
}

func main() {
	fmt.Println("🔍 Comparing checkbox structures between Velo Test and Mohadin tabs...")

	err := checkVeloCheckboxes()
	if err != nil {
		fmt.Printf("❌ Check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Structure comparison completed!")
}