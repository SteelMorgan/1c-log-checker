package techlog

import (
	"testing"
)

func TestParseTextLine_PlainFormat(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr bool
		checks  func(t *testing.T, record interface{})
	}{
		{
			name: "valid plain format",
			line: "2023-08-01T15:01:45.259000-14998,SCALL,0,level=INFO,process=1cv8c,OSThread=2732,ClientID=8",
			wantErr: false,
			checks: func(t *testing.T, r interface{}) {
				record := r.(*TechLogRecord)
				if record.Name != "SCALL" {
					t.Errorf("expected Name=SCALL, got %s", record.Name)
				}
				if record.Duration != 14998 {
					t.Errorf("expected Duration=14998, got %d", record.Duration)
				}
				if record.Level != "INFO" {
					t.Errorf("expected Level=INFO, got %s", record.Level)
				}
			},
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			line:    "invalid line without commas",
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := ParseTextLine(tt.line)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTextLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && tt.checks != nil {
				tt.checks(t, record)
			}
		})
	}
}

func TestParseTextLine_HierarchicalFormat(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr bool
		checks  func(t *testing.T, record interface{})
	}{
		{
			name: "valid hierarchical format",
			line: "45:31.831006-1,SCALL,2,level=INFO,process=1cv8,OSThread=13476,ClientID=1",
			wantErr: false,
			checks: func(t *testing.T, r interface{}) {
				record := r.(*TechLogRecord)
				if record.Name != "SCALL" {
					t.Errorf("expected Name=SCALL, got %s", record.Name)
				}
				if record.Depth != 2 {
					t.Errorf("expected Depth=2, got %d", record.Depth)
				}
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := ParseTextLine(tt.line)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTextLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && tt.checks != nil {
				tt.checks(t, record)
			}
		})
	}
}

func TestParseProperties(t *testing.T) {
	tests := []struct {
		name    string
		props   string
		wantErr bool
		checks  func(t *testing.T, record interface{})
	}{
		{
			name: "simple properties",
			props: "level=INFO,process=1cv8,OSThread=1234",
			wantErr: false,
			checks: func(t *testing.T, r interface{}) {
				record := r.(*TechLogRecord)
				if record.Level != "INFO" {
					t.Errorf("expected Level=INFO, got %s", record.Level)
				}
				if record.Process != "1cv8" {
					t.Errorf("expected Process=1cv8, got %s", record.Process)
				}
			},
		},
		{
			name: "quoted property with comma",
			props: "level=INFO,Txt='Hello, world',process=1cv8",
			wantErr: false,
			checks: func(t *testing.T, r interface{}) {
				record := r.(*TechLogRecord)
				if record.Properties["Txt"] != "Hello, world" {
					t.Errorf("expected Txt='Hello, world', got %s", record.Properties["Txt"])
				}
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &TechLogRecord{
				Properties: make(map[string]string),
			}
			
			err := parseProperties(tt.props, record)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseProperties() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && tt.checks != nil {
				tt.checks(t, record)
			}
		})
	}
}

