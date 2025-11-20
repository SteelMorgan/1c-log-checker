package eventlog

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// LgfReader reads and resolves objects from .lgf file
// Based on OneSTools.EventLog LgfReader.cs
// The .lgf file uses brackets format (binary-like structure)
// We need to parse it using a brackets parser similar to BracketsListReader
type LgfReader struct {
	lgfPath string
	
	// Dictionary: (ObjectType, number) -> value
	objects map[objectTypeKey]string
	
	// Referenced objects: (ObjectType, number) -> (value, uuid)
	// Used for Users and Metadata which have UUIDs
	referencedObjects map[objectTypeKey]referencedValue
	
	// Mutex to protect concurrent access to maps
	mu sync.RWMutex
	
	// Once ensures file is loaded only once
	loadOnce sync.Once
}

type objectTypeKey struct {
	ObjectType ObjectType
	Number     int
}

type referencedValue struct {
	Value string
	UUID  string
}

// ObjectType represents the type of object in .lgf file
// Based on OneSTools.EventLog ObjectType.cs
type ObjectType int

const (
	ObjectTypeNone ObjectType = iota
	ObjectTypeUsers
	ObjectTypeComputers
	ObjectTypeApplications
	ObjectTypeEvents
	ObjectTypeMetadata
	ObjectTypeServers
	ObjectTypeMainPorts
	ObjectTypeAddPorts
	ObjectTypeUnknown
)

// NewLgfReader creates a new LGF reader
func NewLgfReader(lgfPath string) *LgfReader {
	return &LgfReader{
		lgfPath:           lgfPath,
		objects:           make(map[objectTypeKey]string),
		referencedObjects: make(map[objectTypeKey]referencedValue),
	}
}

// GetObjectValue returns the value for a given object type and number
// Returns empty string if not found
// Based on OneSTools.EventLog LgfReader.GetObjectValue
func (r *LgfReader) GetObjectValue(objectType ObjectType, number int, ctx context.Context) string {
	if number == 0 {
		return ""
	}
	
	// Ensure file is loaded (only once, even if called from multiple goroutines)
	var loadErr error
	r.loadOnce.Do(func() {
		loadErr = r.readTill(ObjectTypeNone, 0, 0, ctx)
		if loadErr != nil {
			log.Warn().Err(loadErr).Int("object_type", int(objectType)).Int("number", number).Msg("Failed to read from LGF file")
		}
	})
	
	if loadErr != nil {
			return ""
		}
	
	// Read lock for reading from map (multiple readers can proceed concurrently)
	key := objectTypeKey{ObjectType: objectType, Number: number}
	r.mu.RLock()
	value, ok := r.objects[key]
	r.mu.RUnlock()
	
	if ok {
		return value
	}
	
	return ""
}

// GetReferencedObjectValue returns the value and UUID for a referenced object (Users, Metadata)
// Based on OneSTools.EventLog LgfReader.GetReferencedObjectValue
func (r *LgfReader) GetReferencedObjectValue(objectType ObjectType, number int, ctx context.Context) (string, string) {
	if number == 0 {
		return "", ""
	}
	
	// Ensure file is loaded (only once, even if called from multiple goroutines)
	var loadErr error
	r.loadOnce.Do(func() {
		loadErr = r.readTill(ObjectTypeNone, 0, 0, ctx)
		if loadErr != nil {
			log.Warn().Err(loadErr).Int("object_type", int(objectType)).Int("number", number).Msg("Failed to read referenced object from LGF file")
		}
	})
	
	if loadErr != nil {
			return "", ""
		}
	
	// Read lock for reading from map (multiple readers can proceed concurrently)
	key := objectTypeKey{ObjectType: objectType, Number: number}
	r.mu.RLock()
	value, ok := r.referencedObjects[key]
	r.mu.RUnlock()
	
	if ok {
		return value.Value, value.UUID
	}
	
	return "", ""
}

// readTill reads from .lgf file until the specified object is found
// Based on OneSTools.EventLog LgfReader.ReadTill
// Format: {ObjectType, Value/UUID, Name, Number}
func (r *LgfReader) readTill(objectType ObjectType, number int, position int64, ctx context.Context) error {
	file, err := os.Open(r.lgfPath)
	if err != nil {
		return fmt.Errorf("failed to open lgf file: %w", err)
	}
	defer file.Close()
	
	// Read entire file (it's small, typically < 10KB)
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read lgf file: %w", err)
	}
	
	// Remove BOM if present
	contentStr := string(content)
	contentStr = strings.TrimPrefix(contentStr, "\ufeff")
	
	// Split by lines
	lines := strings.Split(contentStr, "\n")
	
	// Skip header (first 2 lines: version and GUID)
	if len(lines) < 2 {
		return fmt.Errorf("invalid lgf file: too short")
	}
	
	// Parse dictionary entries (lines starting with {)
	loadedCount := 0
	for i := 2; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		
		// Remove trailing comma if present
		line = strings.TrimSuffix(line, ",")
		
		// Parse bracket entry: {ObjectType, Value/UUID, Name, Number}
		// Remove outer braces
		line = strings.TrimPrefix(line, "{")
		line = strings.TrimSuffix(line, "}")
		line = strings.TrimSpace(line)
		
		if line == "" {
			continue
		}
		
		// Tokenize by comma (but respect quotes)
		tokens := tokenizeLgfLine(line)
		if len(tokens) < 3 {
			continue
		}
		
		// Parse ObjectType (first token)
		objTypeNum := parseNumberString(tokens[0])
		objType := ObjectType(objTypeNum)
		
		// Skip unknown types
		if objType >= ObjectTypeUnknown {
			continue
		}
		
		// Parse Number (last token - can be 3rd or 4th depending on format)
		objNumber := 0
		if len(tokens) >= 4 {
			objNumber = parseNumberString(tokens[3])
		} else if len(tokens) >= 3 {
			// Some entries have format: {ObjectType, Value, Number}
			objNumber = parseNumberString(tokens[2])
		}
		
		// Parse value based on object type
		switch objType {
		case ObjectTypeUsers, ObjectTypeMetadata:
			// Referenced objects: {ObjectType, UUID, Name, Number}
			if len(tokens) >= 4 {
				uuid := unquoteLgfString(tokens[1])
				name := unquoteLgfString(tokens[2])
				key := objectTypeKey{ObjectType: objType, Number: objNumber}
				r.referencedObjects[key] = referencedValue{
					Value: name,
					UUID:  uuid,
				}
				loadedCount++
			}
		default:
			// Regular objects: {ObjectType, Value, Number} or {ObjectType, Value, Name, Number}
			value := unquoteLgfString(tokens[1])
			key := objectTypeKey{ObjectType: objType, Number: objNumber}
			r.objects[key] = value
			loadedCount++
		}
	}
	
	log.Debug().
		Str("lgf_path", r.lgfPath).
		Int("loaded_objects", loadedCount).
		Int("users", len(r.referencedObjects)).
		Int("regular_objects", len(r.objects)).
		Msg("LGF file loaded successfully")
	
	return nil
}

// tokenizeLgfLine tokenizes a line from .lgf file, respecting quoted strings
func tokenizeLgfLine(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	escapeNext := false
	
	for _, r := range line {
		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}
		
		if r == '\\' && inQuotes {
			escapeNext = true
			current.WriteRune(r)
			continue
		}
		
		if r == '"' {
			inQuotes = !inQuotes
			current.WriteRune(r)
			continue
		}
		
		if r == ',' && !inQuotes {
			tokens = append(tokens, current.String())
			current.Reset()
			continue
		}
		
		current.WriteRune(r)
	}
	
	// Add last token
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	
	return tokens
}

// unquoteLgfString removes quotes from a string in .lgf file
func unquoteLgfString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseNumberString converts a string to int, handling empty strings
func parseNumberString(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	
	// Try to parse as int
	if num, err := strconv.Atoi(s); err == nil {
		return num
	}
	
	return 0
}

