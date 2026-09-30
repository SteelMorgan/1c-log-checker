package techlog

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Filename timestamp pattern: yymmddhh (8 digits)
// Examples:
//   - hierarchical: 25011408.log → 2025-01-14 08:00:00
//   - plain: rphost_1234_25011408.log → 2025-01-14 08:00:00
var filenameTimestampRegex = regexp.MustCompile(`(\d{8})`)

// ExtractTimestampFromFilename extracts timestamp from tech log filename
//
// Format: yymmddhh (8 digits)
//   - yy: last 2 digits of year (00-99, interpreted as 2000-2099)
//   - mm: month (01-12)
//   - dd: day (01-31)
//   - hh: hour (00-23)
//
// Examples:
//   - "25011408.log" → 2025-01-14 08:00:00
//   - "rphost_1234_25011408.log" → 2025-01-14 08:00:00
//   - "1cv8c_5678_25011409.log" → 2025-01-14 09:00:00
//
// Returns:
//   - time.Time: Timestamp with year, month, day, hour set to 00:00:00
//   - error: if timestamp cannot be extracted or is invalid
func ExtractTimestampFromFilename(filename string) (time.Time, error) {
	// Get just the filename without path
	baseName := filepath.Base(filename)
	
	// Remove extension
	baseName = strings.TrimSuffix(baseName, ".log")
	baseName = strings.TrimSuffix(baseName, ".zip")
	baseName = strings.TrimSuffix(baseName, ".gz")
	
	// Find 8-digit pattern (yymmddhh)
	matches := filenameTimestampRegex.FindStringSubmatch(baseName)
	if len(matches) < 2 {
		return time.Time{}, fmt.Errorf("no timestamp pattern (yymmddhh) found in filename: %s", filename)
	}
	
	timestampStr := matches[1]
	if len(timestampStr) != 8 {
		return time.Time{}, fmt.Errorf("invalid timestamp length in filename: %s (expected 8 digits, got %d)", filename, len(timestampStr))
	}
	
	// Parse components
	yy, err := strconv.Atoi(timestampStr[0:2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year in filename: %s", filename)
	}
	
	mm, err := strconv.Atoi(timestampStr[2:4])
	if err != nil || mm < 1 || mm > 12 {
		return time.Time{}, fmt.Errorf("invalid month in filename: %s", filename)
	}
	
	dd, err := strconv.Atoi(timestampStr[4:6])
	if err != nil || dd < 1 || dd > 31 {
		return time.Time{}, fmt.Errorf("invalid day in filename: %s", filename)
	}
	
	hh, err := strconv.Atoi(timestampStr[6:8])
	if err != nil || hh < 0 || hh > 23 {
		return time.Time{}, fmt.Errorf("invalid hour in filename: %s", filename)
	}
	
	// Convert 2-digit year to 4-digit (00-99 → 2000-2099)
	year := 2000 + yy
	
	// Create timestamp
	timestamp := time.Date(year, time.Month(mm), dd, hh, 0, 0, 0, time.Local)
	
	return timestamp, nil
}

