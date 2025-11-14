package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	_ "github.com/1c-log-checker/internal/observability"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
	"github.com/1c-log-checker/internal/logreader/eventlog"
	"github.com/rs/zerolog/log"
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
	if len(os.Args) < 3 {
		fmt.Println("Usage: compare <mxl_file> <lgp_file> [lgf_file]")
		fmt.Println("Example: compare \"Тек конец ЖР_DSSL.mxl\" \"path/to/file.lgp\" \"path/to/1Cv8.lgf\"")
		os.Exit(1)
	}

	mxlFile := os.Args[1]
	lgpFile := os.Args[2]
	lgfFile := ""
	if len(os.Args) > 3 {
		lgfFile = os.Args[3]
	}

	fmt.Printf("Comparing:\n")
	fmt.Printf("  MXL (expected): %s\n", mxlFile)
	fmt.Printf("  LGP (parsed):  %s\n", lgpFile)
	if lgfFile != "" {
		fmt.Printf("  LGF (resolver): %s\n", lgfFile)
	}
	fmt.Println()

	// Extract records from MXL
	mxlRecords, err := extractMxlRecords(mxlFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to extract MXL records")
	}

	fmt.Printf("Extracted %d records from MXL file\n", len(mxlRecords))
	fmt.Println()

	// Parse LGP file
	lgpRecords, err := parseLgpFile(lgpFile, lgfFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse LGP file")
	}

	fmt.Printf("Parsed %d records from LGP file\n", len(lgpRecords))
	fmt.Println()

	// Compare records
	compareRecords(mxlRecords, lgpRecords)
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

	for _, row := range mxlData.Sheets {
		for _, cell := range row.Cells {
			value := strings.TrimSpace(cell.Data)

			// Skip empty cells
			if value == "" {
				continue
			}

			// Skip header row values
			if isHeader(value) {
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

func isHeader(value string) bool {
	headers := []string{
		"Дата, время",
		"Разделение данных",
		"Пользователь",
		"Компьютер",
		"Приложение",
		"Событие",
		"Комментарий",
		"Метаданные",
		"Данные",
		"Представление данных",
		"Сеанс",
		"Транзакция",
		"Статус транзакции",
		"Рабочий сервер",
		"Основной IP порт",
		"Вспомогательный IP порт",
	}

	for _, h := range headers {
		if value == h {
			return true
		}
	}
	return false
}

func parseLgpFile(lgpFile, lgfFile string) ([]*domain.EventLogRecord, error) {
	file, err := os.Open(lgpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open LGP file: %w", err)
	}
	defer file.Close()

	// Create LGF reader if provided
	var lgfReader *eventlog.LgfReader
	if lgfFile != "" {
		lgfReader = eventlog.NewLgfReader(lgfFile)
	}

	// Create parser
	parser := eventlog.NewLgpParser("", "", lgfReader)

	// Parse file
	records, err := parser.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LGP file: %w", err)
	}

	return records, nil
}

func compareRecords(mxlRecords []MxlRecord, lgpRecords []*domain.EventLogRecord) {
	fmt.Println("=== COMPARISON RESULTS ===")
	fmt.Println()

	// Compare by count
	if len(mxlRecords) != len(lgpRecords) {
		fmt.Printf("⚠️  Record count mismatch: MXL=%d, LGP=%d\n", len(mxlRecords), len(lgpRecords))
		fmt.Println()
	}

	// Compare each record
	maxRecords := len(mxlRecords)
	if len(lgpRecords) > maxRecords {
		maxRecords = len(lgpRecords)
	}

	matchCount := 0
	mismatchCount := 0

	for i := 0; i < maxRecords; i++ {
		var mxlRec *MxlRecord
		var lgpRec *domain.EventLogRecord

		if i < len(mxlRecords) {
			mxlRec = &mxlRecords[i]
		}
		if i < len(lgpRecords) {
			lgpRec = lgpRecords[i]
		}

		if mxlRec == nil {
			fmt.Printf("Record %d: ❌ Missing in MXL\n", i+1)
			mismatchCount++
			continue
		}

		if lgpRec == nil {
			fmt.Printf("Record %d: ❌ Missing in LGP\n", i+1)
			mismatchCount++
			continue
		}

		// Compare timestamp
		mxlTime, err1 := time.Parse("02.01.2006 15:04:05", mxlRec.DateTime)
		if err1 != nil {
			fmt.Printf("Record %d: ⚠️  Failed to parse MXL timestamp: %v\n", i+1, err1)
			mismatchCount++
			continue
		}

		// Compare (allow small time difference due to timezone conversion)
		timeDiff := mxlTime.Sub(lgpRec.EventTime)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}

		matches := true
		var issues []string

		if timeDiff > time.Minute {
			matches = false
			issues = append(issues, fmt.Sprintf("time mismatch: MXL=%s, LGP=%s (diff=%v)", mxlRec.DateTime, lgpRec.EventTime.Format("02.01.2006 15:04:05"), timeDiff))
		}

		// Compare other fields (simplified - just check if values start with expected)
		// Note: MXL fields may be truncated, so we check if LGP values start with MXL values
		if len(mxlRec.Fields) > 0 {
			// Field order in MXL (approximate):
			// 0: DataSeparation, 1: User, 2: Computer, 3: Application, 4: Event, 5: Comment,
			// 6: Metadata, 7: Data, 8: DataPresentation, 9: Session, 10: Transaction, etc.

			fieldIdx := 0

			// User
			if fieldIdx < len(mxlRec.Fields) && mxlRec.Fields[fieldIdx] != "" {
				if !strings.HasPrefix(lgpRec.UserName, mxlRec.Fields[fieldIdx]) && !strings.HasPrefix(mxlRec.Fields[fieldIdx], lgpRec.UserName) {
					matches = false
					issues = append(issues, fmt.Sprintf("user: MXL=%s, LGP=%s", mxlRec.Fields[fieldIdx], lgpRec.UserName))
				}
			}
			fieldIdx++

			// Computer
			if fieldIdx < len(mxlRec.Fields) && mxlRec.Fields[fieldIdx] != "" {
				if !strings.HasPrefix(lgpRec.Computer, mxlRec.Fields[fieldIdx]) && !strings.HasPrefix(mxlRec.Fields[fieldIdx], lgpRec.Computer) {
					matches = false
					issues = append(issues, fmt.Sprintf("computer: MXL=%s, LGP=%s", mxlRec.Fields[fieldIdx], lgpRec.Computer))
				}
			}
			fieldIdx++

			// Application
			if fieldIdx < len(mxlRec.Fields) && mxlRec.Fields[fieldIdx] != "" {
				appMatch := strings.HasPrefix(lgpRec.ApplicationPresentation, mxlRec.Fields[fieldIdx]) ||
					strings.HasPrefix(mxlRec.Fields[fieldIdx], lgpRec.ApplicationPresentation) ||
					strings.HasPrefix(lgpRec.Application, mxlRec.Fields[fieldIdx])
				if !appMatch {
					matches = false
					issues = append(issues, fmt.Sprintf("application: MXL=%s, LGP=%s", mxlRec.Fields[fieldIdx], lgpRec.ApplicationPresentation))
				}
			}
			fieldIdx++

			// Event
			if fieldIdx < len(mxlRec.Fields) && mxlRec.Fields[fieldIdx] != "" {
				eventMatch := strings.HasPrefix(lgpRec.EventPresentation, mxlRec.Fields[fieldIdx]) ||
					strings.HasPrefix(mxlRec.Fields[fieldIdx], lgpRec.EventPresentation)
				if !eventMatch {
					matches = false
					issues = append(issues, fmt.Sprintf("event: MXL=%s, LGP=%s", mxlRec.Fields[fieldIdx], lgpRec.EventPresentation))
				}
			}
			fieldIdx++

			// Comment
			if fieldIdx < len(mxlRec.Fields) && mxlRec.Fields[fieldIdx] != "" {
				if !strings.HasPrefix(lgpRec.Comment, mxlRec.Fields[fieldIdx]) && !strings.HasPrefix(mxlRec.Fields[fieldIdx], lgpRec.Comment) {
					matches = false
					issues = append(issues, fmt.Sprintf("comment: MXL=%s, LGP=%s", truncate(mxlRec.Fields[fieldIdx], 50), truncate(lgpRec.Comment, 50)))
				}
			}
			fieldIdx++

			// Metadata
			if fieldIdx < len(mxlRec.Fields) && mxlRec.Fields[fieldIdx] != "" {
				metadataMatch := strings.HasPrefix(lgpRec.MetadataPresentation, mxlRec.Fields[fieldIdx]) ||
					strings.HasPrefix(mxlRec.Fields[fieldIdx], lgpRec.MetadataPresentation) ||
					strings.HasPrefix(lgpRec.MetadataName, mxlRec.Fields[fieldIdx])
				if !metadataMatch {
					matches = false
					issues = append(issues, fmt.Sprintf("metadata: MXL=%s, LGP=%s", mxlRec.Fields[fieldIdx], lgpRec.MetadataPresentation))
				}
			}
		}

		if matches {
			fmt.Printf("Record %d: ✅ Match\n", i+1)
			matchCount++
		} else {
			fmt.Printf("Record %d: ❌ Mismatch\n", i+1)
			for _, issue := range issues {
				fmt.Printf("  - %s\n", issue)
			}
			mismatchCount++
		}

		// Show details for first few records
		if i < 3 {
			fmt.Printf("  MXL: %s | Fields: %d\n", mxlRec.DateTime, len(mxlRec.Fields))
			fmt.Printf("  LGP: %s | User: %s | Event: %s\n", lgpRec.EventTime.Format("02.01.2006 15:04:05"), lgpRec.UserName, lgpRec.EventPresentation)
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println("=== SUMMARY ===")
	fmt.Printf("Total records: %d\n", maxRecords)
	fmt.Printf("✅ Matches: %d\n", matchCount)
	fmt.Printf("❌ Mismatches: %d\n", mismatchCount)

	// Save detailed comparison to JSON
	outputFile := "comparison_result.json"
	saveComparisonJSON(mxlRecords, lgpRecords, outputFile)
	fmt.Printf("\nDetailed comparison saved to: %s\n", outputFile)
}

func saveComparisonJSON(mxlRecords []MxlRecord, lgpRecords []*domain.EventLogRecord, filename string) {
	type ComparisonResult struct {
		MxlRecord MxlRecord                `json:"mxl"`
		LgpRecord *domain.EventLogRecord   `json:"lgp"`
		Match     bool                      `json:"match"`
	}

	var results []ComparisonResult

	maxLen := len(mxlRecords)
	if len(lgpRecords) > maxLen {
		maxLen = len(lgpRecords)
	}

	for i := 0; i < maxLen; i++ {
		var mxl *MxlRecord
		var lgp *domain.EventLogRecord

		if i < len(mxlRecords) {
			mxl = &mxlRecords[i]
		}
		if i < len(lgpRecords) {
			lgp = lgpRecords[i]
		}

		results = append(results, ComparisonResult{
			MxlRecord: *mxl,
			LgpRecord: lgp,
			Match:     mxl != nil && lgp != nil,
		})
	}

	file, err := os.Create(filename)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create output file")
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

