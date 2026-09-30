package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MxlRecord represents a record extracted from .mxl file
type MxlRecord struct {
	DateTime string   `json:"DateTime"`
	Fields   []string `json:"Fields"`
}

// MxlData represents the structure of .mxl XML file
type MxlData struct {
	XMLName xml.Name `xml:"Workbook"`
	Sheets  []Sheet  `xml:"Worksheet>Table>Row"`
}

type Sheet struct {
	Cells []Cell `xml:"Cell>Data"`
}

type Cell struct {
	Data string `xml:",chardata"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: extract_mxl <mxl_file>")
		fmt.Println("Example: extract_mxl \"Тек конец ЖР_DSSL.mxl\"")
		os.Exit(1)
	}

	mxlFile := os.Args[1]

	fmt.Printf("Extracting records from: %s\n", mxlFile)

	records, err := extractMxlRecords(mxlFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Extracted %d records\n", len(records))

	// Save to JSON
	outputFile := strings.TrimSuffix(mxlFile, filepath.Ext(mxlFile)) + ".json"
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(records); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved to: %s\n", outputFile)

	// Print first few records
	fmt.Println("\nFirst 3 records:")
	for i := 0; i < len(records) && i < 3; i++ {
		fmt.Printf("\nRecord %d:\n", i+1)
		fmt.Printf("  DateTime: %s\n", records[i].DateTime)
		fmt.Printf("  Fields (%d):\n", len(records[i].Fields))
		for j, field := range records[i].Fields {
			if j < 10 { // Show first 10 fields
				fmt.Printf("    [%d] %s\n", j, truncate(field, 60))
			}
		}
	}
}

func extractMxlRecords(mxlFile string) ([]MxlRecord, error) {
	file, err := os.Open(mxlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open MXL file: %w", err)
	}
	defer file.Close()

	// Read XML content
	var mxlData MxlData
	decoder := xml.NewDecoder(file)
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}

	if err := decoder.Decode(&mxlData); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	// Extract text values from cells
	var records []MxlRecord
	var currentRecord MxlRecord
	var inRecord bool

	// Pattern to match date/time: "DD.MM.YYYY HH:MM:SS"
	dateTimePattern := "02.01.2006 15:04:05"

	// Headers to skip
	headers := map[string]bool{
		"Дата, время":              true,
		"Разделение данных":        true,
		"Пользователь":             true,
		"Компьютер":                true,
		"Приложение":               true,
		"Событие":                  true,
		"Комментарий":              true,
		"Метаданные":               true,
		"Данные":                   true,
		"Представление данных":     true,
		"Сеанс":                    true,
		"Транзакция":               true,
		"Статус транзакции":        true,
		"Рабочий сервер":           true,
		"Основной IP порт":         true,
		"Вспомогательный IP порт": true,
	}

	for _, row := range mxlData.Sheets {
		for _, cell := range row.Cells {
			value := strings.TrimSpace(cell.Data)

			// Skip empty cells
			if value == "" {
				continue
			}

			// Skip header row values
			if headers[value] {
				continue
			}

			// Check if this is a timestamp (starts a new record)
			if _, err := time.Parse(dateTimePattern, value); err == nil {
				// Save previous record if exists
				if inRecord && len(currentRecord.Fields) > 0 {
					records = append(records, currentRecord)
				}
				// Start new record
				currentRecord = MxlRecord{
					DateTime: value,
					Fields:   []string{},
				}
				inRecord = true
			} else if inRecord {
				// Add field to current record
				currentRecord.Fields = append(currentRecord.Fields, value)
			}
		}
	}

	// Add last record
	if inRecord && len(currentRecord.Fields) > 0 {
		records = append(records, currentRecord)
	}

	return records, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

