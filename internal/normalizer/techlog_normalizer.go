package normalizer

import (
	"regexp"
	"strings"
)

// TechLogNormalizer normalizes tech log raw lines by replacing dynamic parts
// with placeholders to enable grouping similar records together
type TechLogNormalizer struct {
	// Compiled regex patterns for normalization (common patterns from event_log)
	guidPattern      *regexp.Regexp
	timestampPattern *regexp.Regexp
	numberPattern    *regexp.Regexp
	stringPattern    *regexp.Regexp
	userPattern      *regexp.Regexp
	computerPattern  *regexp.Regexp
	
	// SQL-specific patterns
	tempTablePattern *regexp.Regexp // For MS SQL: #tt[0-9]+
	postgresParamPattern *regexp.Regexp // For PostgreSQL: $[0-9]+
}

// NewTechLogNormalizer creates a new tech log normalizer with compiled patterns
func NewTechLogNormalizer() *TechLogNormalizer {
	return &TechLogNormalizer{
		// GUID pattern: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}
		guidPattern: regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
		
		// Timestamp pattern: \d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}
		timestampPattern: regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
		
		// Number pattern: \b\d+\b (word boundaries to match whole numbers)
		numberPattern: regexp.MustCompile(`\b\d+\b`),
		
		// String pattern: "[^"]*" (quoted strings)
		stringPattern: regexp.MustCompile(`"[^"]*"`),
		
		// User pattern: "пользователь: ИмяПользователя" or "user: UserName" (case insensitive)
		userPattern: regexp.MustCompile(`(?i)(?:пользователь|user)\s*:\s*[^,]+`),
		
		// Computer pattern: "компьютер: ИмяКомпьютера" or "computer: ComputerName" (case insensitive)
		computerPattern: regexp.MustCompile(`(?i)(?:компьютер|computer)\s*:\s*[^,]+`),
		
		// SQL-specific patterns
		tempTablePattern: regexp.MustCompile(`#tt\d+`), // MS SQL temporary tables: #tt123
		postgresParamPattern: regexp.MustCompile(`\$(\d+)`), // PostgreSQL parameters: $1, $2, etc.
	}
}

// NormalizeRawLine normalizes a raw line string by replacing dynamic parts with placeholders
// Returns empty string if input is empty
// Pattern order matters - apply in specific order to avoid conflicts
// IMPORTANT: This normalizer is selective - it preserves JSON structure and important keys,
// only normalizing dynamic values to maintain error identification capability
func (n *TechLogNormalizer) NormalizeRawLine(rawLine string) string {
	if rawLine == "" {
		return ""
	}
	
	normalized := rawLine
	
	// Step 1: SQL-specific normalization
	// Always check for PostgreSQL parameters (they can appear without SQL keywords)
	if n.postgresParamPattern.MatchString(normalized) {
		normalized = n.normalizePostgreSQL(normalized)
	}
	
	// Check if line contains SQL keywords or patterns
	if n.containsSQL(normalized) {
		// Detect DBMS type from raw_line
		dbms := n.detectDBMS(normalized)
		
		if dbms == "DBMSSQL" || strings.Contains(strings.ToLower(normalized), "sp_executesql") {
			normalized = n.normalizeMSSQL(normalized)
		} else if dbms == "DBPOSTGRS" {
			normalized = n.normalizePostgreSQL(normalized)
		} else {
			// If SQL detected but DBMS not identified, try to normalize temporary tables anyway
			// (they can appear in raw SQL without sp_executesql)
			if n.tempTablePattern.MatchString(normalized) {
				normalized = n.normalizeMSSQL(normalized)
			}
		}
	}
	
	// Step 2: Apply selective normalization - preserve JSON structure and important values
	// Only normalize clearly dynamic parts, preserve everything else for error identification
	
	// 1. GUIDs first (most specific)
	normalized = n.guidPattern.ReplaceAllString(normalized, "<GUID>")
	
	// 2. Timestamps - normalize ISO timestamps
	normalized = n.timestampPattern.ReplaceAllString(normalized, "<TIMESTAMP>")
	
	// 3. Computer names (before users to process in order)
	normalized = n.computerPattern.ReplaceAllStringFunc(normalized, func(match string) string {
		match = strings.TrimSpace(match)
		match = strings.TrimSuffix(match, ",")
		if strings.Contains(strings.ToLower(match), "компьютер") {
			return "компьютер: <COMPUTER>"
		}
		return "computer: <COMPUTER>"
	})
	
	// 4. User names (before numbers to avoid conflicts)
	normalized = n.userPattern.ReplaceAllStringFunc(normalized, func(match string) string {
		match = strings.TrimSpace(match)
		match = strings.TrimSuffix(match, ",")
		if strings.Contains(strings.ToLower(match), "пользователь") {
			return "пользователь: <USER>"
		}
		return "user: <USER>"
	})
	
	// 5. Large numbers only (IDs, large counts) - preserve small numbers (flags, small counts)
	// Normalize numbers with 6+ digits (likely IDs, timestamps in milliseconds, etc.)
	largeNumberPattern := regexp.MustCompile(`\b\d{6,}\b`)
	normalized = largeNumberPattern.ReplaceAllString(normalized, "<NUMBER>")
	
	// NOTE: We do NOT normalize all strings in quotes - this would destroy error identification
	// Only specific dynamic patterns are normalized above
	
	// Trim whitespace
	normalized = strings.TrimSpace(normalized)
	
	return normalized
}


// containsSQL checks if the raw line contains SQL-related content
func (n *TechLogNormalizer) containsSQL(rawLine string) bool {
	lower := strings.ToLower(rawLine)
	sqlKeywords := []string{
		"select", "insert", "update", "delete", "exec", "execute",
		"sp_executesql", "from", "where", "join", "union",
		"dbmssql", "dbpostgrs", "sql=", "query=",
	}
	
	for _, keyword := range sqlKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	
	return false
}

// detectDBMS detects DBMS type from raw line
func (n *TechLogNormalizer) detectDBMS(rawLine string) string {
	lower := strings.ToLower(rawLine)
	
	if strings.Contains(lower, "dbmssql") || strings.Contains(lower, "sp_executesql") {
		return "DBMSSQL"
	}
	if strings.Contains(lower, "dbpostgrs") || strings.Contains(lower, "execute $") {
		return "DBPOSTGRS"
	}
	
	return ""
}

// normalizeMSSQL normalizes MS SQL queries based on fn_GetSQLNormalized.sql logic
func (n *TechLogNormalizer) normalizeMSSQL(sql string) string {
	// 1. Remove "exec sp_executesql N'" from the beginning
	sql = strings.Replace(sql, "exec sp_executesql N'", "", 1)
	sql = strings.Replace(sql, "EXEC sp_executesql N'", "", 1)
	sql = strings.Replace(sql, "Exec sp_executesql N'", "", 1)
	
	// 2. Find position of "',N'" and truncate everything after it
	// This removes parameter definitions
	if pos := strings.Index(sql, "',N'"); pos > 0 {
		sql = sql[:pos]
	}
	if pos := strings.Index(sql, "',n'"); pos > 0 {
		sql = sql[:pos]
	}
	if pos := strings.Index(sql, "', N'"); pos > 0 {
		sql = sql[:pos]
	}
	
	// 3. Cyclically replace temporary tables #tt[0-9]+ with #tt
	// This matches the logic from fn_GetSQLNormalized.sql
	for {
		if !n.tempTablePattern.MatchString(sql) {
			break
		}
		sql = n.tempTablePattern.ReplaceAllString(sql, "#tt")
	}
	
	return sql
}

// normalizePostgreSQL normalizes PostgreSQL queries
func (n *TechLogNormalizer) normalizePostgreSQL(sql string) string {
	// Replace parameterized query parameters $1, $2, ... with $<NUMBER>
	sql = n.postgresParamPattern.ReplaceAllString(sql, "$<NUMBER>")
	
	// Handle EXECUTE statements: EXECUTE $1, $2, ... -> EXECUTE $<NUMBER>
	// This is already handled by the parameter replacement above
	
	return sql
}

