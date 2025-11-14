# Анализ проблем OneSTools.EventLog

**Дата:** 2025-11-13  
**Репозиторий:** https://github.com/akpaevj/OneSTools.EventLog  
**Язык:** C# (.NET 5)

---

## 🔴 КРИТИЧЕСКИЕ ПРОБЛЕМЫ

### 1. **Утечка ресурсов в LgpReader.Dispose()**

**Файл:** `OneSTools.EventLog/LgpReader.cs`

**Проблема:**
```csharp
protected virtual void Dispose(bool disposing)
{
    if (_disposedValue) return;

    _bracketsReader?.Dispose();
    _bracketsReader = null;
    _fileStream = null;  // ❌ ПРОБЛЕМА: не вызывается Dispose()!
    
    _lgpFileWatcher?.Dispose();
    _lgpFileWatcher = null;
    
    _lgfReader = null;
    
    _disposedValue = true;
}
```

**Последствия:**
- `FileStream` не закрывается явно, полагается на финализатор
- При большом количестве файлов → утечка дескрипторов файлов
- Windows: "Too many open files" → падение системы
- Linux: аналогично, лимит `ulimit -n`

**Исправление:**
```csharp
_fileStream?.Dispose();  // ✅ Правильно
_fileStream = null;
```

---

### 2. **Race condition в FileSystemWatcher**

**Файл:** `OneSTools.EventLog/LgpReader.cs`

**Проблема:**
```csharp
private void LgpFileWatcher_Deleted(object sender, FileSystemEventArgs e)
{
    if (e.ChangeType == WatcherChangeTypes.Deleted && LgpPath == e.FullPath) 
        Dispose();  // ❌ ПРОБЛЕМА: Dispose() вызывается из потока FileSystemWatcher
}
```

**Последствия:**
- `Dispose()` вызывается из фонового потока FileSystemWatcher
- Одновременный доступ к `_fileStream`, `_bracketsReader` из разных потоков
- `ObjectDisposedException` при попытке чтения после удаления
- Возможен deadlock при одновременном вызове `ReadNextEventLogItem()` и `Dispose()`

**Исправление:**
- Использовать `lock` или `Interlocked` для синхронизации
- Или использовать `CancellationToken` для корректной остановки

---

### 3. **Неправильная обработка ошибок в EventLogExporter**

**Файл:** `OneSTools.EventLog.Exporter.Core/EventLogExporter.cs`

**Проблема:**
```csharp
_writeBlock = new ActionBlock<EventLogItem[]>(async c =>
{
    try
    {
        await _storage.WriteEventLogDataAsync(c.ToList(), cancellationToken);
    }
    catch (Exception)
    {
        _batchBlock.Complete();  // ❌ ПРОБЛЕМА: немедленное завершение
        _writeBlock.Complete();
        throw;  // ❌ ПРОБЛЕМА: исключение пробрасывается, но блоки уже завершены
    }
}, writeBlockSettings);
```

**Последствия:**
- При любой ошибке записи (сеть, БД) → все блоки завершаются
- Данные в `_batchBlock` теряются (не записываются)
- Нет retry механизма
- Нет логирования ошибки
- Система падает без возможности восстановления

**Исправление:**
- Retry с экспоненциальной задержкой
- Dead letter queue для проблемных записей
- Логирование ошибок
- Graceful degradation вместо немедленного падения

---

### 4. **Бесконечный цикл в SendAsync при переполнении очереди**

**Файл:** `OneSTools.EventLog.Exporter.Core/EventLogExporter.cs`

**Проблема:**
```csharp
private static async Task SendAsync(ITargetBlock<EventLogItem> nextBlock, EventLogItem item,
    CancellationToken stoppingToken = default)
{
    while (!stoppingToken.IsCancellationRequested && !nextBlock.Completion.IsCompleted)
        if (await nextBlock.SendAsync(item, stoppingToken))
            break;  // ❌ ПРОБЛЕМА: если очередь переполнена, цикл будет бесконечным
}
```

**Последствия:**
- Если `_batchBlock` переполнен (`BoundedCapacity` достигнут)
- `SendAsync` возвращает `false`, но цикл продолжается
- 100% загрузка CPU на одном потоке
- Нет таймаута
- Нет backpressure механизма

**Исправление:**
- Добавить таймаут
- Использовать `OfferMessage` с проверкой результата
- Или увеличить `BoundedCapacity`

---

### 5. **Потенциальная утечка памяти в StringBuilder**

**Файл:** `OneSTools.EventLog/LgpReader.cs`

**Проблема:**
```csharp
private (StringBuilder Data, long EndPosition) ReadNextEventLogItemData()
{
    var data = _bracketsReader.NextNodeAsStringBuilder();  // ❌ ПРОБЛЕМА: кто освобождает StringBuilder?
    return (data, _bracketsReader.Position);
}
```

**Последствия:**
- `StringBuilder` может быть большим (особенно для событий с большим `Data`)
- Если `BracketsListReader` не освобождает внутренние буферы → утечка памяти
- При обработке миллионов событий → OOM (Out of Memory)

**Проверка:** Нужно посмотреть реализацию `BracketsListReader.NextNodeAsStringBuilder()`

---

### 6. **Отсутствие освобождения Dataflow блоков**

**Файл:** `OneSTools.EventLog.Exporter.Core/EventLogExporter.cs`

**Проблема:**
```csharp
protected virtual void Dispose(bool disposing)
{
    if (_disposedValue) return;

    if (disposing) _storage?.Dispose();

    _eventLogReader?.Dispose();
    
    // ❌ ПРОБЛЕМА: _batchBlock и _writeBlock не освобождаются явно
    // Они реализуют IDisposable, но Dispose() не вызывается
    
    _disposedValue = true;
}
```

**Последствия:**
- Блоки Dataflow могут держать ссылки на данные
- Память не освобождается до GC
- При частых созданиях/уничтожениях экспортеров → утечка памяти

**Исправление:**
```csharp
_batchBlock?.Complete();
_writeBlock?.Complete();
await Task.WhenAll(_batchBlock.Completion, _writeBlock.Completion);
_batchBlock = null;
_writeBlock = null;
```

---

## ⚠️ СРЕДНИЕ ПРОБЛЕМЫ

### 7. **Нет обработки исключений при парсинге**

**Файл:** `OneSTools.EventLog/LgpReader.cs`

**Проблема:**
```csharp
private EventLogItem ParseEventLogItemData(StringBuilder eventLogItemData, long endPosition,
    CancellationToken cancellationToken = default)
{
    var parsedData = BracketsParser.ParseBlock(eventLogItemData);
    
    DateTime dateTime = default;
    try
    {
        dateTime = _timeZone.ToUtc(DateTime.ParseExact(parsedData[0], "yyyyMMddHHmmss",
            CultureInfo.InvariantCulture));
    }
    catch
    {
        dateTime = DateTime.MinValue;  // ❌ ПРОБЛЕМА: молча игнорируется ошибка
    }
    // ... дальше нет try-catch для других полей
}
```

**Последствия:**
- При поврежденном файле → `IndexOutOfRangeException` или `FormatException`
- Исключение пробрасывается наверх → падение всего экспортера
- Нет логирования проблемных записей

---

### 8. **Потенциальный deadlock в InitializeStreams**

**Файл:** `OneSTools.EventLog/LgpReader.cs`

**Проблема:**
```csharp
private void InitializeStreams()
{
    if (_fileStream is null)
    {
        if (!File.Exists(LgpPath))
            throw new Exception($"Cannot find lgp file by path {LgpPath}");  // ❌ ПРОБЛЕМА: между проверкой и открытием файл может быть удален
        
        _lgpFileWatcher = new FileSystemWatcher(Path.GetDirectoryName(LgpPath)!, "*.lgp")
        {
            NotifyFilter = NotifyFilters.CreationTime | NotifyFilters.LastWrite | NotifyFilters.FileName | NotifyFilters.Attributes
        };
        _lgpFileWatcher.Deleted += LgpFileWatcher_Deleted;
        _lgpFileWatcher.EnableRaisingEvents = true;
        
        _fileStream = new FileStream(LgpPath, FileMode.Open, FileAccess.Read,
            FileShare.ReadWrite | FileShare.Delete);  // ❌ ПРОБЛЕМА: если файл удален между проверкой и открытием → исключение
        _bracketsReader = new BracketsListReader(_fileStream);
    }
}
```

**Последствия:**
- Race condition: файл может быть удален между `File.Exists()` и `new FileStream()`
- `FileNotFoundException` → падение экспортера
- Нет retry механизма

---

## 📋 РЕКОМЕНДАЦИИ

### Приоритет 1 (Критично):
1. ✅ Исправить `_fileStream?.Dispose()` в `LgpReader.Dispose()`
2. ✅ Добавить синхронизацию для `FileSystemWatcher.Deleted`
3. ✅ Исправить обработку ошибок в `_writeBlock` (retry + логирование)
4. ✅ Добавить таймаут в `SendAsync`

### Приоритет 2 (Важно):
5. ✅ Освобождать Dataflow блоки в `Dispose()`
6. ✅ Добавить try-catch для всех полей в `ParseEventLogItemData`
7. ✅ Исправить race condition в `InitializeStreams`

### Приоритет 3 (Желательно):
8. ✅ Проверить освобождение `StringBuilder` в `BracketsListReader`
9. ✅ Добавить метрики и мониторинг
10. ✅ Добавить unit-тесты для edge cases

---

## 🔍 ДОПОЛНИТЕЛЬНЫЕ ЗАМЕЧАНИЯ

### Архитектурные проблемы:
- **Нет graceful shutdown:** при остановке теряются данные в буферах
- **Нет backpressure:** при медленной записи в БД очередь растет без ограничений
- **Нет метрик:** невозможно отследить производительность и проблемы

### Производительность:
- `c.ToList()` в `_writeBlock` создает копию массива → лишние аллокации
- Нет пулинга объектов для часто создаваемых структур

---

## 📚 ССЫЛКИ

- Репозиторий: https://github.com/akpaevj/OneSTools.EventLog
- Issues: https://github.com/akpaevj/OneSTools.EventLog/issues (27 открытых)
- Pull Requests: https://github.com/akpaevj/OneSTools.EventLog/pulls (8 открытых)

---

**Вывод:** Репозиторий содержит несколько критических проблем, которые могут приводить к падению системы при длительной работе. Основные проблемы связаны с утечками ресурсов, race conditions и неправильной обработкой ошибок.

