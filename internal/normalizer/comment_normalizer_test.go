package normalizer

import (
	"testing"
)

func TestNormalizeComment(t *testing.T) {
	normalizer := NewCommentNormalizer()
	
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
			input:    "Ошибка выполнения запроса",
			expected: "Ошибка выполнения запроса",
		},
		{
			name:     "GUID replacement",
			input:    "Ошибка с GUID 12345678-1234-1234-1234-123456789abc",
			expected: "Ошибка с GUID <GUID>",
		},
		{
			name:     "timestamp replacement",
			input:    "Ошибка в 2024-01-15 10:30:45",
			expected: "Ошибка в <TIMESTAMP>",
		},
		{
			name:     "number replacement",
			input:    "Ошибка номер 12345",
			expected: "Ошибка номер <NUMBER>",
		},
		{
			name:     "string in quotes replacement",
			input:    `Ошибка в поле "ИмяПоля"`,
			expected: "Ошибка в поле <STRING>",
		},
		{
			name:     "multiple patterns",
			input:    `Ошибка с GUID 12345678-1234-1234-1234-123456789abc в поле "ИмяПоля" номер 12345 в 2024-01-15 10:30:45`,
			expected: "Ошибка с GUID <GUID> в поле <STRING> номер <NUMBER> в <TIMESTAMP>",
		},
		{
			name:     "timestamp with T separator",
			input:    "Ошибка в 2024-01-15T10:30:45",
			expected: "Ошибка в <TIMESTAMP>",
		},
		{
			name:     "multiple numbers",
			input:    "Ошибка с числами 123 и 456",
			expected: "Ошибка с числами <NUMBER> и <NUMBER>",
		},
		{
			name:     "multiple GUIDs",
			input:    "GUID1: 12345678-1234-1234-1234-123456789abc, GUID2: 87654321-4321-4321-4321-cba987654321",
			expected: "GUID1: <GUID>, GUID2: <GUID>",
		},
		{
			name:     "real-world error example",
			input:    `Ошибка выполнения запроса к базе данных "БазаДанных" с параметром 12345 в транзакции 67890`,
			expected: "Ошибка выполнения запроса к базе данных <STRING> с параметром <NUMBER> в транзакции <NUMBER>",
		},
		{
			name:     "user name normalization (Russian)",
			input:    "Ошибка выполнения запроса пользователь: Администратор",
			expected: "Ошибка выполнения запроса пользователь: <USER>",
		},
		{
			name:     "user name normalization (English)",
			input:    "Error executing query user: Admin",
			expected: "Error executing query user: <USER>",
		},
		{
			name:     "computer and user name with comma (real format)",
			input:    "компьютер: STEEL-PC, пользователь: Администратор,",
			expected: "компьютер: <COMPUTER>, пользователь: <USER>,",
		},
		{
			name:     "computer name only",
			input:    "компьютер: WORKSTATION-01,",
			expected: "компьютер: <COMPUTER>,",
		},
		{
			name:     "computer name (English)",
			input:    "Error on computer: SERVER-01,",
			expected: "Error on computer: <COMPUTER>,",
		},
		{
			name:     "user name with spaces and comma",
			input:    "Ошибка пользователь: Иван Иванов, выполнил операцию",
			expected: "Ошибка пользователь: <USER>, выполнил операцию",
		},
		{
			name:     "user name with multiple patterns",
			input:    `Ошибка пользователь: Администратор, в базе "БазаДанных" номер 12345`,
			expected: "Ошибка пользователь: <USER>, в базе <STRING> номер <NUMBER>",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.NormalizeComment(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeComment(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeComment_Whitespace(t *testing.T) {
	normalizer := NewCommentNormalizer()
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "leading whitespace",
			input:    "   Ошибка",
			expected: "Ошибка",
		},
		{
			name:     "trailing whitespace",
			input:    "Ошибка   ",
			expected: "Ошибка",
		},
		{
			name:     "both sides whitespace",
			input:    "   Ошибка   ",
			expected: "Ошибка",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.NormalizeComment(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeComment(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

