package eventlog

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
	"github.com/rs/zerolog/log"
)

// LgpParser parses .lgp files in 1C Event Log format
// Format: 1CV8LOG(ver 2.0)
//         <infobase_guid>
//         {timestamp,level,{transaction},session,user,computer,event,record,app,"comment",data_sep,{metadata},"metadata_pres",...,{props}}
type LgpParser struct {
	infobaseGUID string
	clusterGUID  string
}

// NewLgpParser creates a new parser for .lgp files
func NewLgpParser(clusterGUID, infobaseGUID string) *LgpParser {
	return &LgpParser{
		clusterGUID:  clusterGUID,
		infobaseGUID: infobaseGUID,
	}
}

// Parse reads and parses .lgp file
func (p *LgpParser) Parse(r io.Reader) ([]*domain.EventLogRecord, error) {
	scanner := bufio.NewScanner(r)
	
	var records []*domain.EventLogRecord
	lineNum := 0
	
	// Read header
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file")
	}
	header := strings.TrimSpace(scanner.Text())
	// Remove BOM (Byte Order Mark) if present
	header = strings.TrimPrefix(header, "\ufeff")
	if !strings.HasPrefix(header, "1CV8LOG") {
		return nil, fmt.Errorf("invalid header: %s", header)
	}
	lineNum++
	
	// Read infobase GUID (if not already set)
	if !scanner.Scan() {
		return nil, fmt.Errorf("missing infobase GUID")
	}
	guid := strings.TrimSpace(scanner.Text())
	if p.infobaseGUID == "" {
		p.infobaseGUID = guid
	}
	lineNum++
	
	// Skip empty line
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// This might be the first record, process it
			record, err := p.parseRecord(line)
			if err != nil {
				log.Warn().Err(err).Int("line", lineNum).Msg("Failed to parse record, skipping")
			} else {
				records = append(records, record)
			}
		}
		lineNum++
	}
	
	// Read all records
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		// Remove trailing comma if present
		line = strings.TrimSuffix(line, ",")
		
		record, err := p.parseRecord(line)
		if err != nil {
			log.Warn().Err(err).Int("line", lineNum).Str("line_preview", truncate(line, 100)).Msg("Failed to parse record, skipping")
			continue
		}
		
		records = append(records, record)
		lineNum++
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}
	
	log.Info().Int("records", len(records)).Msg("Parsed .lgp file")
	return records, nil
}

// parseRecord parses a single record from .lgp file
// Format based on OneSTools.EventLog C# implementation:
// {timestamp,transaction_status,{transaction_date_hex,transaction_number_hex},user_id,computer_id,application,connection,event_id,severity,comment,metadata,data,data_presentation,server,main_port,add_port,session}
func (p *LgpParser) parseRecord(line string) (*domain.EventLogRecord, error) {
	// Remove outer braces
	line = strings.TrimPrefix(line, "{")
	line = strings.TrimSuffix(line, "}")
	
	record := &domain.EventLogRecord{
		ClusterGUID:  p.clusterGUID,
		InfobaseGUID: p.infobaseGUID,
		Properties:   make(map[string]string),
	}
	
	// Parse fields using tokenizer that handles nested structures
	tokens, err := tokenizeRecord(line)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}
	
	if len(tokens) < 9 {
		return nil, fmt.Errorf("insufficient tokens: %d, expected at least 9", len(tokens))
	}
	
	// Field 0: timestamp (YYYYMMDDHHMMSS)
	timestamp, err := parseLgpTimestamp(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	record.EventTime = timestamp
	record.EventDate = time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
	
	// Field 1: transaction status (U, C, R, N)
	record.TransactionStatus = getTransactionPresentation(tokens[1])
	
	// Field 2: transaction {transaction_date_hex, transaction_number_hex}
	if len(tokens) > 2 {
		transactionDateTime, transactionNumber, connectionID, err := parseTransactionFromHex(tokens[2])
		if err == nil {
			record.TransactionDateTime = transactionDateTime
			record.TransactionNumber = transactionNumber
			record.ConnectionID = connectionID
			// TransactionID is derived from transaction number
			record.TransactionID = fmt.Sprintf("%d", transactionNumber)
		}
	}
	
	// Field 3: user_id (number - will be resolved from LGF file later)
	// For now, store as string in properties
	if len(tokens) > 3 {
		record.Properties["user_id"] = tokens[3]
	}
	
	// Field 4: computer_id (number - will be resolved from LGF file later)
	if len(tokens) > 4 {
		record.Properties["computer_id"] = tokens[4]
	}
	
	// Field 5: application (code - will be resolved from LGF file later)
	if len(tokens) > 5 {
		// Store raw code, will be resolved to presentation later
		record.Application = tokens[5]
		record.ApplicationPresentation = getApplicationPresentation(tokens[5])
	}
	
	// Field 6: connection (string)
	if len(tokens) > 6 {
		record.Connection = unquoteString(tokens[6])
	}
	
	// Field 7: event_id (number - will be resolved from LGF file later)
	if len(tokens) > 7 {
		// Store raw code, will be resolved to presentation later
		record.Event = tokens[7]
		record.EventPresentation = getEventPresentation(tokens[7])
	}
	
	// Field 8: severity (I, E, W, N) - this is the level!
	if len(tokens) > 8 {
		record.Level = getSeverityPresentation(tokens[8])
	}
	
	// Field 9: comment (quoted string)
	if len(tokens) > 9 {
		record.Comment = unquoteString(tokens[9])
	}
	
	// Field 10: metadata (array - will be resolved from LGF file later)
	if len(tokens) > 10 {
		metadata, err := parseMetadataArray(tokens[10])
		if err == nil && len(metadata) > 0 {
			record.MetadataName = metadata[0]
		}
	}
	
	// Field 11: data (complex structure)
	if len(tokens) > 11 {
		data, err := parseDataField(tokens[11])
		if err == nil {
			record.Data = data
		}
	}
	
	// Field 12: data_presentation (quoted string)
	if len(tokens) > 12 {
		record.DataPresentation = unquoteString(tokens[12])
	}
	
	// Field 13: server (code - will be resolved from LGF file later)
	if len(tokens) > 13 {
		record.Server = tokens[13]
	}
	
	// Field 14: main_port (code - will be resolved from LGF file later)
	if len(tokens) > 14 {
		if port, err := strconv.ParseUint(tokens[14], 10, 16); err == nil {
			record.PrimaryPort = uint16(port)
		}
	}
	
	// Field 15: add_port (code - will be resolved from LGF file later)
	if len(tokens) > 15 {
		if port, err := strconv.ParseUint(tokens[15], 10, 16); err == nil {
			record.SecondaryPort = uint16(port)
		}
	}
	
	// Field 16: session (string)
	if len(tokens) > 16 {
		if sessionID, err := strconv.ParseUint(tokens[16], 10, 64); err == nil {
			record.SessionID = sessionID
		} else {
			// If not a number, store as string
			record.Properties["session_string"] = tokens[16]
		}
	}
	
	return record, nil
}

// tokenizeRecord tokenizes a record line, handling nested structures
func tokenizeRecord(line string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	depth := 0
	inQuotes := false
	escape := false
	
	for i, r := range line {
		if escape {
			current.WriteRune(r)
			escape = false
			continue
		}
		
		switch r {
		case '\\':
			escape = true
			current.WriteRune(r)
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case '{':
			if !inQuotes {
				depth++
			}
			current.WriteRune(r)
		case '}':
			if !inQuotes {
				depth--
			}
			current.WriteRune(r)
		case ',':
			if !inQuotes && depth == 0 {
				// End of token
				token := strings.TrimSpace(current.String())
				if token != "" {
					tokens = append(tokens, token)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
		
		// Safety check
		if i > 10000 {
			return nil, fmt.Errorf("line too long or malformed")
		}
	}
	
	// Add last token
	token := strings.TrimSpace(current.String())
	if token != "" {
		tokens = append(tokens, token)
	}
	
	return tokens, nil
}

// parseLgpTimestamp parses timestamp in format YYYYMMDDHHMMSS
func parseLgpTimestamp(s string) (time.Time, error) {
	if len(s) != 14 {
		return time.Time{}, fmt.Errorf("invalid timestamp length: %d", len(s))
	}
	
	year, _ := strconv.Atoi(s[0:4])
	month, _ := strconv.Atoi(s[4:6])
	day, _ := strconv.Atoi(s[6:8])
	hour, _ := strconv.Atoi(s[8:10])
	min, _ := strconv.Atoi(s[10:12])
	sec, _ := strconv.Atoi(s[12:14])
	
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local), nil
}

// parseTransactionFromHex parses transaction field {transaction_date_hex, transaction_number_hex}
// Based on C#: Convert.ToInt64(transactionData[0], 16) / 10000 for date, Convert.ToInt64(transactionData[1], 16) for number
func parseTransactionFromHex(s string) (time.Time, int64, uint64, error) {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return time.Time{}, 0, 0, fmt.Errorf("invalid transaction format: expected 2 parts, got %d", len(parts))
	}
	
	// Parse transaction date (hex to int64, then divide by 10000 to get seconds)
	// Based on C#: new DateTime().AddSeconds(Convert.ToInt64(transactionData[0], 16) / 10000)
	dateHex := strings.TrimSpace(parts[0])
	dateValue, err := strconv.ParseInt(dateHex, 16, 64)
	if err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("failed to parse transaction date hex: %w", err)
	}
	// Divide by 10000 to get seconds (as in C# code)
	seconds := dateValue / 10000
	// Create time from Unix epoch (1970-01-01) + seconds
	transactionDateTime := time.Unix(seconds, 0).UTC()
	
	// Parse transaction number (hex to int64)
	numberHex := strings.TrimSpace(parts[1])
	transactionNumber, err := strconv.ParseInt(numberHex, 16, 64)
	if err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("failed to parse transaction number hex: %w", err)
	}
	
	// ConnectionID is typically the same as transaction number in this context
	connectionID := uint64(transactionNumber)
	
	return transactionDateTime, transactionNumber, connectionID, nil
}

// parseMetadataArray parses metadata array {item1,item2,...}
func parseMetadataArray(s string) ([]string, error) {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	
	if s == "" {
		return []string{}, nil
	}
	
	// Split by comma, handling quoted strings
	var items []string
	var current strings.Builder
	inQuotes := false
	
	for _, r := range s {
		switch r {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case ',':
			if !inQuotes {
				item := strings.TrimSpace(current.String())
				item = unquoteString(item)
				if item != "" {
					items = append(items, item)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}
	
	// Add last item
	item := strings.TrimSpace(current.String())
	item = unquoteString(item)
	if item != "" {
		items = append(items, item)
	}
	
	return items, nil
}

// unquoteString removes quotes from string
func unquoteString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// getTransactionPresentation maps transaction status code to Russian presentation
// Based on C# GetTransactionPresentation
func getTransactionPresentation(code string) string {
	switch code {
	case "U":
		return "Зафиксирована"
	case "C":
		return "Отменена"
	case "R":
		return "Не завершена"
	case "N":
		return "Нет транзакции"
	default:
		return code
	}
}

// getSeverityPresentation maps severity code to Russian presentation
// Based on C# GetSeverityPresentation
func getSeverityPresentation(code string) string {
	switch code {
	case "I":
		return "Информация"
	case "E":
		return "Ошибка"
	case "W":
		return "Предупреждение"
	case "N":
		return "Примечание"
	default:
		return code
	}
}

// getApplicationPresentation maps application code to Russian presentation
// Based on C# GetApplicationPresentation
func getApplicationPresentation(code string) string {
	switch code {
	case "1CV8":
		return "Толстый клиент"
	case "1CV8C":
		return "Тонкий клиент"
	case "WebClient":
		return "Веб-клиент"
	case "Designer":
		return "Конфигуратор"
	case "COMConnection":
		return "Внешнее соединение (COM, обычное)"
	case "WSConnection":
		return "Сессия web-сервиса"
	case "BackgroundJob":
		return "Фоновое задание"
	case "SystemBackgroundJob":
		return "Системное фоновое задание"
	case "SrvrConsole":
		return "Консоль кластера"
	case "COMConsole":
		return "Внешнее соединение (COM, административное)"
	case "JobScheduler":
		return "Планировщик заданий"
	case "Debugger":
		return "Отладчик"
	case "RAS":
		return "Сервер администрирования"
	default:
		return code
	}
}

// getEventPresentation maps event code to Russian presentation
// Based on C# GetEventPresentation - full mapping
func getEventPresentation(code string) string {
	// Remove quotes if present
	code = unquoteString(code)
	
	eventMap := map[string]string{
		"_$Access$_.Access":                                    "Доступ.Доступ",
		"_$Access$_.AccessDenied":                             "Доступ.Отказ в доступе",
		"_$Data$_.Delete":                                      "Данные.Удаление",
		"_$Data$_.DeletePredefinedData":                        "Данные.Удаление предопределенных данных",
		"_$Data$_.DeleteVersions":                              "Данные.Удаление версий",
		"_$Data$_.New":                                         "Данные.Добавление",
		"_$Data$_.NewPredefinedData":                           "Данные.Добавление предопределенных данных",
		"_$Data$_.NewVersion":                                  "Данные.Добавление версии",
		"_$Data$_.Pos":                                         "Данные.Проведение",
		"_$Data$_.PredefinedDataInitialization":                "Данные.Инициализация предопределенных данных",
		"_$Data$_.PredefinedDataInitializationDataNotFound":    "Данные.Инициализация предопределенных данных.Данные не найдены",
		"_$Data$_.SetPredefinedDataInitialization":              "Данные.Установка инициализации предопределенных данных",
		"_$Data$_.SetStandardODataInterfaceContent":            "Данные.Изменение состава стандартного интерфейса OData",
		"_$Data$_.TotalsMaxPeriodUpdate":                       "Данные.Изменение максимального периода рассчитанных итогов",
		"_$Data$_.TotalsMinPeriodUpdate":                       "Данные.Изменение минимального периода рассчитанных итогов",
		"_$Data$_.Post":                                        "Данные.Проведение",
		"_$Data$_.Unpost":                                      "Данные.Отмена проведения",
		"_$Data$_.Update":                                      "Данные.Изменение",
		"_$Data$_.UpdatePredefinedData":                        "Данные.Изменение предопределенных данных",
		"_$Data$_.VersionCommentUpdate":                        "Данные.Изменение комментария версии",
		"_$InfoBase$_.ConfigExtensionUpdate":                   "Информационная база.Изменение расширения конфигурации",
		"_$InfoBase$_.ConfigUpdate":                            "Информационная база.Изменение конфигурации",
		"_$InfoBase$_.DBConfigBackgroundUpdateCancel":          "Информационная база.Отмена фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateFinish":           "Информационная база.Завершение фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateResume":          "Информационная база.Продолжение (после приостановки) процесса фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateStart":           "Информационная база.Запуск фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateSuspend":         "Информационная база.Приостановка (пауза) процесса фонового обновления",
		"_$InfoBase$_.DBConfigExtensionUpdate":                 "Информационная база.Изменение расширения конфигурации",
		"_$InfoBase$_.DBConfigExtensionUpdateError":             "Информационная база.Ошибка изменения расширения конфигурации",
		"_$InfoBase$_.DBConfigUpdate":                          "Информационная база.Изменение конфигурации базы данных",
		"_$InfoBase$_.DBConfigUpdateStart":                      "Информационная база.Запуск обновления конфигурации базы данных",
		"_$InfoBase$_.DumpError":                               "Информационная база.Ошибка выгрузки в файл",
		"_$InfoBase$_.DumpFinish":                              "Информационная база.Окончание выгрузки в файл",
		"_$InfoBase$_.DumpStart":                               "Информационная база.Начало выгрузки в файл",
		"_$InfoBase$_.EraseData":                               "Информационная база.Удаление данных информационной баз",
		"_$InfoBase$_.EventLogReduce":                          "Информационная база.Сокращение журнала регистрации",
		"_$InfoBase$_.EventLogReduceError":                      "Информационная база.Ошибка сокращения журнала регистрации",
		"_$InfoBase$_.EventLogSettingsUpdate":                  "Информационная база.Изменение параметров журнала регистрации",
		"_$InfoBase$_.EventLogSettingsUpdateError":              "Информационная база.Ошибка при изменение настроек журнала регистрации",
		"_$InfoBase$_.InfoBaseAdmParamsUpdate":                 "Информационная база.Изменение параметров информационной базы",
		"_$InfoBase$_.InfoBaseAdmParamsUpdateError":            "Информационная база.Ошибка изменения параметров информационной базы",
		"_$InfoBase$_.IntegrationServiceActiveUpdate":          "Информационная база.Изменение активности сервиса интеграции",
		"_$InfoBase$_.IntegrationServiceSettingsUpdate":       "Информационная база.Изменение настроек сервиса интеграции",
		"_$InfoBase$_.MasterNodeUpdate":                        "Информационная база.Изменение главного узла",
		"_$InfoBase$_.PredefinedDataUpdate":                    "Информационная база.Обновление предопределенных данных",
		"_$InfoBase$_.RegionalSettingsUpdate":                  "Информационная база.Изменение региональных установок",
		"_$InfoBase$_.RestoreError":                            "Информационная база.Ошибка загрузки из файла",
		"_$InfoBase$_.RestoreFinish":                           "Информационная база.Окончание загрузки из файла",
		"_$InfoBase$_.RestoreStart":                            "Информационная база.Начало загрузки из файла",
		"_$InfoBase$_.SecondFactorAuthTemplateDelete":          "Информационная база.Удаление шаблона вторго фактора аутентификации",
		"_$InfoBase$_.SecondFactorAuthTemplateNew":             "Информационная база.Добавление шаблона вторго фактора аутентификации",
		"_$InfoBase$_.SecondFactorAuthTemplateUpdate":           "Информационная база.Изменение шаблона вторго фактора аутентификации",
		"_$InfoBase$_.SetPredefinedDataUpdate":                  "Информационная база.Установить обновление предопределенных данных",
		"_$InfoBase$_.TARImportant":                            "Тестирование и исправление.Ошибка",
		"_$InfoBase$_.TARInfo":                                 "Тестирование и исправление.Сообщение",
		"_$InfoBase$_.TARMess":                                 "Тестирование и исправление.Предупреждение",
		"_$Job$_.Cancel":                                       "Фоновое задание.Отмена",
		"_$Job$_.Fail":                                         "Фоновое задание.Ошибка выполнения",
		"_$Job$_.Start":                                        "Фоновое задание.Запуск",
		"_$Job$_.Succeed":                                      "Фоновое задание.Успешное завершение",
		"_$Job$_.Terminate":                                    "Фоновое задание.Принудительное завершение",
		"_$OpenIDProvider$_.NegativeAssertion":                  "Провайдер OpenID.Отклонено",
		"_$OpenIDProvider$_.PositiveAssertion":                 "Провайдер OpenID.Подтверждено",
		"_$PerformError$_":                                     "Ошибка выполнения",
		"_$Session$_.Authentication":                           "Сеанс.Аутентификация",
		"_$Session$_.AuthenticationError":                      "Сеанс.Ошибка аутентификации",
		"_$Session$_.AuthenticationFirstFactor":                "Сеанс.Аутентификация первый фактор",
		"_$Session$_.ConfigExtensionApplyError":                "Сеанс.Ошибка применения расширения конфигурации",
		"_$Session$_.Finish":                                   "Сеанс.Завершение",
		"_$Session$_.Start":                                    "Сеанс.Начало",
		"_$Transaction$_.Begin":                                "Транзакция.Начало",
		"_$Transaction$_.Commit":                               "Транзакция.Фиксация",
		"_$Transaction$_.Rollback":                             "Транзакция.Отмена",
		"_$User$_.AuthenticationLock":                           "Пользователи.Блокировка аутентификации",
		"_$User$_.AuthenticationUnlock":                         "Пользователи.Разблокировка аутентификации",
		"_$User$_.AuthenticationUnlockError ":                    "Пользователи.Ошибка разблокировки аутентификации",
		"_$User$_.Delete":                                      "Пользователи.Удаление",
		"_$User$_.DeleteError":                                 "Пользователи.Ошибка удаления",
		"_$User$_.New":                                         "Пользователи.Добавление",
		"_$User$_.NewError":                                    "Пользователи.Ошибка добавления",
		"_$User$_.Update":                                      "Пользователи.Изменение",
		"_$User$_.UpdateError":                                  "Пользователи. Ошибка изменения",
	}
	
	if presentation, ok := eventMap[code]; ok {
		return presentation
	}
	return code
}

// parseDataField parses complex data structure
// Based on C# GetData method
func parseDataField(s string) (string, error) {
	// Remove outer braces if present
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	
	if s == "" {
		return "", nil
	}
	
	// Parse data type and value
	// Format: {type,value} or {type,{subdata}}
	tokens, err := tokenizeRecord(s)
	if err != nil {
		return "", fmt.Errorf("failed to tokenize data field: %w", err)
	}
	
	if len(tokens) < 2 {
		return "", nil
	}
	
	dataType := tokens[0]
	value := tokens[1]
	
	switch dataType {
	case "R": // Reference
		return unquoteString(value), nil
	case "U": // Undefined
		return "", nil
	case "S": // String
		return unquoteString(value), nil
	case "B": // Boolean
		if value == "0" {
			return "false", nil
		}
		return "true", nil
	case "P": // Complex data
		// Parse sub-data recursively
		var result strings.Builder
		subData := value
		subData = strings.TrimPrefix(subData, "{")
		subData = strings.TrimSuffix(subData, "}")
		
		subTokens, err := tokenizeRecord(subData)
		if err != nil {
			return "", fmt.Errorf("failed to parse complex data: %w", err)
		}
		
		// Skip first token (sub-data type)
		for i := 1; i < len(subTokens); i++ {
			subValue, err := parseDataField("{" + subTokens[i] + "}")
			if err == nil && subValue != "" {
				result.WriteString(fmt.Sprintf("Item %d: %s\n", i, subValue))
			}
		}
		
		return strings.TrimSpace(result.String()), nil
	default:
		return "", nil
	}
}

// truncate truncates string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

