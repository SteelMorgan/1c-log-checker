package normalizer

import (
	"testing"
)

func TestNormalizeRawLine_CommonPatterns(t *testing.T) {
	normalizer := NewTechLogNormalizer()
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple text without patterns",
			input:    "2023-08-01T15:01:45.259000-14998,SCALL,1",
			expected: "<TIMESTAMP>.<NUMBER>-14998,SCALL,1",
		},
		{
			name:     "GUID replacement",
			input:    "Event with GUID 12345678-1234-1234-1234-123456789abc",
			expected: "Event with GUID <GUID>",
		},
		{
			name:     "timestamp replacement",
			input:    "2023-08-01T15:01:45,SCALL,1",
			expected: "<TIMESTAMP>,SCALL,1",
		},
		{
			name:     "number replacement - large numbers only",
			input:    "Duration: 123456 microseconds",
			expected: "Duration: <NUMBER> microseconds",
		},
		{
			name:     "string in quotes - preserved for error identification",
			input:    `Error in field "FieldName"`,
			expected: `Error in field "FieldName"`,
		},
		{
			name:     "computer and user name",
			input:    "компьютер: STEEL-PC, пользователь: Администратор,",
			expected: "компьютер: <COMPUTER>, пользователь: <USER>,",
		},
		{
			name:     "multiple patterns combined",
			input:    `2023-08-01T15:01:45,DBMSSQL,1,sql=SELECT * FROM "Table" WHERE id=123 AND guid='12345678-1234-1234-1234-123456789abc'`,
			expected: `<TIMESTAMP>,DBMSSQL,1,sql=SELECT * FROM "Table" WHERE id=123 AND guid='<GUID>'`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.NormalizeRawLine(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeRawLine(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeRawLine_MSSQL(t *testing.T) {
	normalizer := NewTechLogNormalizer()
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "MS SQL with sp_executesql",
			input:    `exec sp_executesql N'SELECT * FROM Table WHERE id=@p1',N'@p1 int',@p1=123`,
			expected: `SELECT * FROM Table WHERE id=@p1`,
		},
		{
			name:     "MS SQL with temporary tables",
			input:    `SELECT * FROM #tt123 INNER JOIN #tt456 ON #tt123.id = #tt456.id`,
			expected: `SELECT * FROM #tt INNER JOIN #tt ON #tt.id = #tt.id`,
		},
		{
			name:     "MS SQL combined: sp_executesql + temp tables",
			input:    `exec sp_executesql N'SELECT * FROM #tt123 WHERE id=@p1',N'@p1 int',@p1=123`,
			expected: `SELECT * FROM #tt WHERE id=@p1`,
		},
		{
			name:     "MS SQL case insensitive",
			input:    `EXEC sp_executesql N'SELECT * FROM Table',N'@p1 int',@p1=123`,
			expected: `SELECT * FROM Table`,
		},
		{
			name:     "MS SQL with multiple temp tables",
			input:    `SELECT * FROM #tt1, #tt2, #tt999 WHERE #tt1.id = #tt2.id`,
			expected: `SELECT * FROM #tt, #tt, #tt WHERE #tt.id = #tt.id`,
		},
		{
			name:     "MS SQL in raw_line format",
			input:    `2023-08-01T15:01:45,DBMSSQL,1,sql=exec sp_executesql N'SELECT * FROM #tt123',N'@p1 int',@p1=123`,
			expected: `<TIMESTAMP>,DBMSSQL,1,sql=SELECT * FROM #tt`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.NormalizeRawLine(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeRawLine(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeRawLine_PostgreSQL(t *testing.T) {
	normalizer := NewTechLogNormalizer()
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PostgreSQL parameterized query",
			input:    `SELECT * FROM table WHERE id = $1 AND name = $2`,
			expected: `SELECT * FROM table WHERE id = $<NUMBER> AND name = $<NUMBER>`,
		},
		{
			name:     "PostgreSQL EXECUTE statement",
			input:    `EXECUTE $1, $2, $3`,
			expected: `EXECUTE $<NUMBER>, $<NUMBER>, $<NUMBER>`,
		},
		{
			name:     "PostgreSQL in raw_line format",
			input:    `2023-08-01T15:01:45,DBPOSTGRS,1,sql=SELECT * FROM table WHERE id = $1`,
			expected: `<TIMESTAMP>,DBPOSTGRS,1,sql=SELECT * FROM table WHERE id = $<NUMBER>`,
		},
		{
			name:     "PostgreSQL with multiple parameters",
			input:    `SELECT * FROM table WHERE id IN ($1, $2, $3, $10, $99)`,
			expected: `SELECT * FROM table WHERE id IN ($<NUMBER>, $<NUMBER>, $<NUMBER>, $<NUMBER>, $<NUMBER>)`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.NormalizeRawLine(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeRawLine(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeRawLine_Combined(t *testing.T) {
	normalizer := NewTechLogNormalizer()
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "MS SQL with all patterns",
			input:    `2023-08-01T15:01:45,DBMSSQL,1,компьютер: PC1, пользователь: User1, sql=exec sp_executesql N'SELECT * FROM #tt123 WHERE guid=''12345678-1234-1234-1234-123456789abc'' AND id=@p1',N'@p1 int',@p1=123`,
			expected: `<TIMESTAMP>,DBMSSQL,1,компьютер: <COMPUTER>, пользователь: <USER>, sql=SELECT * FROM #tt WHERE guid=''<GUID>'' AND id=@p1`,
		},
		{
			name:     "PostgreSQL with all patterns",
			input:    `2023-08-01T15:01:45,DBPOSTGRS,1,компьютер: PC1, пользователь: User1, sql=SELECT * FROM "table" WHERE id = $1 AND guid = '12345678-1234-1234-1234-123456789abc'`,
			expected: `<TIMESTAMP>,DBPOSTGRS,1,компьютер: <COMPUTER>, пользователь: <USER>, sql=SELECT * FROM "table" WHERE id = $<NUMBER> AND guid = '<GUID>'`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.NormalizeRawLine(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeRawLine(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeMSSQL(t *testing.T) {
	normalizer := NewTechLogNormalizer()
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic sp_executesql",
			input:    `exec sp_executesql N'SELECT * FROM Table',N'@p1 int',@p1=123`,
			expected: `SELECT * FROM Table`,
		},
		{
			name:     "sp_executesql with parameters",
			input:    `exec sp_executesql N'SELECT * FROM Table WHERE id=@p1',N'@p1 int',@p1=123`,
			expected: `SELECT * FROM Table WHERE id=@p1`,
		},
		{
			name:     "temporary tables",
			input:    `SELECT * FROM #tt123`,
			expected: `SELECT * FROM #tt`,
		},
		{
			name:     "multiple temporary tables",
			input:    `SELECT * FROM #tt1, #tt2, #tt999`,
			expected: `SELECT * FROM #tt, #tt, #tt`,
		},
		{
			name:     "combined: sp_executesql + temp tables",
			input:    `exec sp_executesql N'SELECT * FROM #tt123 WHERE id=@p1',N'@p1 int',@p1=123`,
			expected: `SELECT * FROM #tt WHERE id=@p1`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.normalizeMSSQL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeMSSQL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizePostgreSQL(t *testing.T) {
	normalizer := NewTechLogNormalizer()
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single parameter",
			input:    `SELECT * FROM table WHERE id = $1`,
			expected: `SELECT * FROM table WHERE id = $<NUMBER>`,
		},
		{
			name:     "multiple parameters",
			input:    `SELECT * FROM table WHERE id = $1 AND name = $2`,
			expected: `SELECT * FROM table WHERE id = $<NUMBER> AND name = $<NUMBER>`,
		},
		{
			name:     "EXECUTE statement",
			input:    `EXECUTE $1, $2, $3`,
			expected: `EXECUTE $<NUMBER>, $<NUMBER>, $<NUMBER>`,
		},
		{
			name:     "large parameter numbers",
			input:    `SELECT * FROM table WHERE id IN ($1, $10, $99, $100)`,
			expected: `SELECT * FROM table WHERE id IN ($<NUMBER>, $<NUMBER>, $<NUMBER>, $<NUMBER>)`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.normalizePostgreSQL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePostgreSQL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

