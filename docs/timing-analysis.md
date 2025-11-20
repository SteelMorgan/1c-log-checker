# Анализ временных метрик парсера

## Текущая проблема

Временные метрики **НЕ покрывают весь процесс** и **имеют пересечения**:

### Текущая структура времени:

1. **Reader (processFile):**
   - `fileOpenStartTime` - начало открытия файла (строка 292)
   - `parsingStartTime` - начало парсинга (строка 295) - почти сразу после fileOpenStartTime
   - `parsingEndTime` - конец парсинга (строка 435)
   - `readingEndTime` - конец чтения (строка 440) - почти сразу после parsingEndTime
   - `recordParsingTime` = parsingEndTime - parsingStartTime
   - `readingDuration` = readingEndTime - fileOpenStartTime
   - `fileReadingTimeMs` = readingDurationMs - recordParsingTimeMs (ОЦЕНКА!)

2. **Writer (flushEventLogSnapshot):**
   - `startTime` - начало обработки батча (строка 418)
   - `deduplicationTime` - накапливается для каждой записи (строка 428, 476-478)
   - `writingStartTime` - начало записи (строка 596)
   - `writingTime` = time.Since(writingStartTime) (строка 673)
   - `totalTime` = time.Since(startTime) (строка 674)

### Проблемы:

1. **Время записи НЕ синхронизировано с временем чтения/парсинга**
   - Записи отправляются в канал асинхронно (`r.recordChan <- record`)
   - Запись происходит параллельно с чтением/парсингом
   - Время записи измеряется в writer, но оно не входит в общее время обработки файла

2. **Время чтения и парсинга пересекаются**
   - В streaming mode чтение и парсинг происходят одновременно
   - `fileReadingTimeMs` вычисляется как разница, что может быть неточно

3. **Время дедупликации и записи пересекаются**
   - Дедупликация происходит перед записью, но они измеряются отдельно
   - `totalTime` в writer включает и дедупликацию, и запись

## Правильная структура времени

Весь процесс должен быть покрыт **БЕЗ пересечений** и **БЕЗ выпадений**:

```
[fileOpenStartTime] ────────────────────────────────────────── [fileCloseTime]
│                                                                 │
├─ [parsingStartTime] ──────── [parsingEndTime] ────────────────┤
│   │                              │                              │
│   ├─ File Reading (I/O)          │                              │
│   ├─ Record Parsing (CPU)         │                              │
│   └─ Channel Send (async)         │                              │
│                                   │                              │
│                                   ├─ [batchStartTime] ──────── [batchEndTime]
│                                   │   │                          │
│                                   │   ├─ Deduplication          │
│                                   │   ├─ Batch Prepare          │
│                                   │   └─ Batch Send             │
│                                   │                              │
└───────────────────────────────────┴──────────────────────────────┘
```

### Правильные метрики:

1. **FileReadingTimeMs** = время I/O операций (чтение с диска)
   - Измеряется от начала чтения до конца чтения
   - В streaming mode это часть общего времени парсинга

2. **RecordParsingTimeMs** = время парсинга записей (CPU)
   - Измеряется от начала парсинга до конца парсинга
   - Включает парсинг всех записей из файла

3. **DeduplicationTimeMs** = время проверки дубликатов
   - Измеряется в writer для каждой записи
   - Накапливается для всех записей в батче

4. **WritingTimeMs** = время записи в ClickHouse
   - Измеряется от PrepareBatch до Send
   - Включает подготовку батча, Append всех записей, и Send

### Общее время должно быть:

```
TotalTime = FileReadingTimeMs + RecordParsingTimeMs + DeduplicationTimeMs + WritingTimeMs
```

Но это **НЕ работает**, потому что:
- FileReadingTimeMs и RecordParsingTimeMs пересекаются (streaming mode)
- DeduplicationTimeMs и WritingTimeMs происходят ПОСЛЕ RecordParsingTimeMs
- Время записи происходит асинхронно, параллельно с чтением/парсингом

## Решение

Нужно изменить структуру измерения времени:

1. **Общее время обработки файла:**
   - От `fileOpenStartTime` до момента, когда ВСЕ записи записаны в ClickHouse
   - Это включает: чтение, парсинг, дедупликацию, запись

2. **Детализация времени:**
   - FileReadingTimeMs - время I/O (чтение с диска)
   - RecordParsingTimeMs - время парсинга (CPU)
   - DeduplicationTimeMs - время дедупликации (накапливается)
   - WritingTimeMs - время записи (накапливается)

3. **Проверка покрытия:**
   - FileReadingTimeMs + RecordParsingTimeMs ≈ ParsingTimeMs (в streaming mode они пересекаются)
   - DeduplicationTimeMs + WritingTimeMs = время обработки батчей (после парсинга)
   - TotalTime = ParsingTimeMs + (DeduplicationTimeMs + WritingTimeMs)

## Текущее состояние кода

### Reader (reader.go):
- ✅ `fileOpenStartTime` - начало
- ✅ `parsingStartTime` - начало парсинга
- ✅ `parsingEndTime` - конец парсинга
- ✅ `readingEndTime` - конец чтения
- ❌ `fileReadingTimeMs` - вычисляется как разница (неточно)
- ✅ `recordParsingTimeMs` - измеряется правильно

### Writer (clickhouse.go):
- ✅ `deduplicationTime` - накапливается правильно
- ✅ `writingTime` - измеряется правильно
- ❌ НЕ синхронизировано с временем чтения/парсинга

## Что нужно исправить

1. **Убрать оценку fileReadingTimeMs** - использовать реальное время
2. **Синхронизировать время записи с временем чтения/парсинга**
3. **Добавить проверку покрытия** - убедиться, что сумма времен покрывает весь процесс

