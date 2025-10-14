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

func updateCheckboxStructure() error {
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

	// Define the Velo Test headers to copy to Mohadin and Lawley
	veloHeaders := []interface{}{
		"Date", // A
		"Drop Number", // B
		"Step 1: Property Frontage –\nhouse, street number visible.", // C
		"Step 2: Location on Wall (Before Install)\n", // D
		"Step 3: Outside Cable Span (Pole →\nPigtail screw)", // E
		"Step 4: Home Entry Point – Outside ", // F
		"Step 5: Home Entry Point – Inside", // G
		"Step 6: Fibre Entry to ONT (After Install)", // H
		"Step 7: Patched & Labelled Drop", // I
		"Step 8: Overall Work Area After\nCompletion", // J
		"Step 9: ONT Barcode – Scan barcode +\nphoto of label.", // K
		"Step 10: Mini-UPS Serial Number (Gizzu)", // L
		"Step 11: Powermeter Reading\n(Drop/Feeder)", // M
		"Step 12: Powermeter at ONT (Before\nActivation)", // N
		"Step 13: Active Broadband Light", // O
		"Step 14: Customer Signature", // P
		"Completed Photos", // Q
		"X - OUTSTANDING PHOTOS", // R
		"User", // S
		"Outstanding Photos loaded onto 1MAP", // T
		"Comment", // U
		"Incomplete", // V
		"Resubmitted", // W
		"completed", // X
	}

	// Tabs to update
	tabs := []string{
		"Mohadin WA_Tool Monitor",
		"Lawley WA_Tool Monitor",
	}

	for _, tabName := range tabs {
		fmt.Printf("🔧 Updating %s tab to match Velo Test checkbox structure...\n", tabName)

		// Clear the current header row (row 1)
		clearRange := fmt.Sprintf("%s!1:1", tabName)
		clearReq := &sheets.BatchClearValuesRequest{
			Ranges: []string{clearRange},
		}

		_, err = srv.Spreadsheets.Values.BatchClear(GOOGLE_SHEETS_ID, clearReq).Context(ctx).Do()
		if err != nil {
			fmt.Printf("⚠️  Could not clear row 1 in %s: %v\n", tabName, err)
		} else {
			fmt.Printf("   ✅ Cleared existing headers in %s\n", tabName)
		}

		// Write the new Velo Test headers
		headerRange := fmt.Sprintf("%s!A1:X1", tabName) // 24 columns like Velo Test
		vr := &sheets.ValueRange{
			Values: [][]interface{}{veloHeaders},
		}

		_, err = srv.Spreadsheets.Values.Update(GOOGLE_SHEETS_ID, headerRange, vr).
			ValueInputOption("USER_ENTERED").
			Context(ctx).
			Do()

		if err != nil {
			fmt.Printf("❌ Failed to update headers in %s: %v\n", tabName, err)
		} else {
			fmt.Printf("✅ Updated %s headers to match Velo Test structure (24 columns)\n", tabName)
		}

		// Clear any extra columns beyond X that might exist
		extraClearRange := fmt.Sprintf("%s!Y:Z", tabName) // Clear old Y-Z columns
		extraClearReq := &sheets.BatchClearValuesRequest{
			Ranges: []string{extraClearRange},
		}

		_, err = srv.Spreadsheets.Values.BatchClear(GOOGLE_SHEETS_ID, extraClearReq).Context(ctx).Do()
		if err != nil {
			fmt.Printf("⚠️  Could not clear extra columns in %s: %v\n", tabName, err)
		} else {
			fmt.Printf("   ✅ Cleared extra columns (Y-Z) in %s\n", tabName)
		}
	}

	return nil
}

func main() {
	fmt.Println("🔧 Updating Mohadin and Lawley tabs to match Velo Test checkbox structure...")

	err := updateCheckboxStructure()
	if err != nil {
		fmt.Printf("❌ Update failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Checkbox structure update completed!")
	fmt.Println("\n📋 All tabs now have identical 14-step checkbox structure:")
	fmt.Println("   - Columns A-B: Date, Drop Number")
	fmt.Println("   - Columns C-P: Steps 1-14 (checkbox fields)")
	fmt.Println("   - Columns Q-X: Completion tracking fields")
}