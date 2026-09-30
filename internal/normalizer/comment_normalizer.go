package normalizer

import (
	"regexp"
	"strings"
)

// CommentNormalizer normalizes error comments by replacing dynamic parts
// with placeholders to enable grouping similar errors together
type CommentNormalizer struct {
	// Compiled regex patterns for normalization
	guidPattern      *regexp.Regexp
	timestampPattern *regexp.Regexp
	numberPattern    *regexp.Regexp
	stringPattern    *regexp.Regexp
	userPattern      *regexp.Regexp
	computerPattern  *regexp.Regexp
}

// NewCommentNormalizer creates a new comment normalizer with compiled patterns
func NewCommentNormalizer() *CommentNormalizer {
	return &CommentNormalizer{
		// GUID pattern: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}
		guidPattern: regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
		
		// Timestamp pattern: \d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}
		timestampPattern: regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
		
		// Number pattern: \b\d+\b (word boundaries to match whole numbers)
		numberPattern: regexp.MustCompile(`\b\d+\b`),
		
		// String pattern: "[^"]*" (quoted strings)
		stringPattern: regexp.MustCompile(`"[^"]*"`),
		
		// User pattern: "пользователь: ИмяПользователя" or "user: UserName" (case insensitive)
		// Matches: "пользователь: Администратор,", "user: Admin,", "Пользователь: Иван Иванов"
		// Captures name until comma or end of string (non-greedy)
		userPattern: regexp.MustCompile(`(?i)(?:пользователь|user)\s*:\s*[^,]+`),
		
		// Computer pattern: "компьютер: ИмяКомпьютера" or "computer: ComputerName" (case insensitive)
		// Matches: "компьютер: STEEL-PC,", "computer: WORKSTATION,"
		// Captures name until comma or end of string
		computerPattern: regexp.MustCompile(`(?i)(?:компьютер|computer)\s*:\s*[^,]+`),
	}
}

// NormalizeComment normalizes a comment string by replacing dynamic parts with placeholders
// Returns empty string if input is empty
// Pattern order matters - apply in specific order to avoid conflicts
func (n *CommentNormalizer) NormalizeComment(comment string) string {
	if comment == "" {
		return ""
	}
	
	// Apply patterns in order:
	// 1. GUIDs first (most specific)
	normalized := n.guidPattern.ReplaceAllString(comment, "<GUID>")
	
	// 2. Timestamps
	normalized = n.timestampPattern.ReplaceAllString(normalized, "<TIMESTAMP>")
	
	// 3. Computer names (before users to process in order)
	// Replace "компьютер: Имя" or "computer: Name" with "компьютер: <COMPUTER>"
	normalized = n.computerPattern.ReplaceAllStringFunc(normalized, func(match string) string {
		// Extract the prefix (компьютер: or computer:) and replace the name part
		// Trim trailing whitespace and comma if present
		match = strings.TrimSpace(match)
		match = strings.TrimSuffix(match, ",")
		
		if strings.Contains(strings.ToLower(match), "компьютер") {
			return "компьютер: <COMPUTER>"
		}
		return "computer: <COMPUTER>"
	})
	
	// 4. User names (before numbers to avoid conflicts with numbers in usernames)
	// Replace "пользователь: Имя" or "user: Name" with "пользователь: <USER>"
	normalized = n.userPattern.ReplaceAllStringFunc(normalized, func(match string) string {
		// Extract the prefix (пользователь: or user:) and replace the name part
		// Trim trailing whitespace and comma if present
		match = strings.TrimSpace(match)
		match = strings.TrimSuffix(match, ",")
		
		if strings.Contains(strings.ToLower(match), "пользователь") {
			// Extract just "пользователь:" part and add <USER>
			return "пользователь: <USER>"
		}
		return "user: <USER>"
	})
	
	// 5. Numbers (after timestamps, computers and users to avoid conflicts)
	normalized = n.numberPattern.ReplaceAllString(normalized, "<NUMBER>")
	
	// 6. Strings in quotes (last, as they may contain other patterns)
	normalized = n.stringPattern.ReplaceAllString(normalized, "<STRING>")
	
	// Trim whitespace
	normalized = strings.TrimSpace(normalized)
	
	return normalized
}

