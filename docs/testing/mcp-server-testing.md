# Тестирование MCP-сервера напрямую

Руководство по тестированию MCP-сервера без подключения через IDE Cursor.

## Быстрый старт

### 1. Запуск MCP-сервера

```powershell
# Сборка
go build -o bin/mcp.exe ./cmd/mcp

# Запуск (требует deploy/docker/.env файл или переменные окружения)
.\bin\mcp.exe
```

Или через Docker:

```powershell
cd deploy/docker
docker-compose up mcp-server
```

### 2. Проверка доступности

```powershell
# Health check
Invoke-RestMethod -Uri "http://localhost:8080/health"
```

Ожидаемый ответ:
```json
{
  "status": "ok"
}
```

## Тестовые GUID'ы

Для тестирования используйте следующие GUID'ы:

- **Cluster GUID:** `b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3`
- **Infobase GUID:** `d723aefd-7992-420d-b5f9-a273fd4146be`

## Автоматическое тестирование

### Полный набор тестов

```powershell
.\scripts\test_mcp_server.ps1
```

Тестирует все доступные инструменты:
- Health check
- configure_techlog (с валидацией)
- get_event_log
- get_tech_log
- get_new_errors

### Тест только configure_techlog

```powershell
.\scripts\test_configure_techlog.ps1
```

Проверяет:
- ✅ Правильный путь (должен пройти)
- ❌ Неправильный путь (должна быть ошибка)
- ❌ Неправильный GUID (должна быть ошибка)

## Ручное тестирование через PowerShell

### 1. configure_techlog

**Правильный запрос:**
```powershell
$body = @{
    cluster_guid = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
    infobase_guid = "d723aefd-7992-420d-b5f9-a273fd4146be"
    location = "D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be"
    history = 24
    format = "json"
    events = @("EXCP", "QERR")
    properties = @("all")
} | ConvertTo-Json -Depth 10

Invoke-RestMethod -Uri "http://localhost:8080/tools/configure_techlog" `
    -Method POST `
    -ContentType "application/json" `
    -Body $body
```

**Ожидаемый результат:** XML конфигурация logcfg.xml

**Неправильный запрос (для проверки валидации):**
```powershell
$invalidBody = @{
    cluster_guid = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
    infobase_guid = "d723aefd-7992-420d-b5f9-a273fd4146be"
    location = "D:\TechLogs"  # ❌ Нет GUID'ов в пути
    history = 24
    format = "json"
    events = @("EXCP")
    properties = @("all")
} | ConvertTo-Json -Depth 10

try {
    Invoke-RestMethod -Uri "http://localhost:8080/tools/configure_techlog" `
        -Method POST `
        -ContentType "application/json" `
        -Body $invalidBody
} catch {
    Write-Host "Ошибка валидации (ожидаемо): $($_.ErrorDetails.Message)"
}
```

**Ожидаемый результат:** HTTP 400 с описанием ошибки валидации

### 2. get_event_log

```powershell
$fromTime = (Get-Date).AddHours(-1).ToString("yyyy-MM-ddTHH:mm:ssZ")
$toTime = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ssZ")

$body = @{
    cluster_guid = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
    infobase_guid = "d723aefd-7992-420d-b5f9-a273fd4146be"
    from = $fromTime
    to = $toTime
    mode = "minimal"
    limit = 10
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/tools/get_event_log" `
    -Method POST `
    -ContentType "application/json" `
    -Body $body
```

### 3. get_tech_log

```powershell
$fromTime = (Get-Date).AddHours(-1).ToString("yyyy-MM-ddTHH:mm:ssZ")
$toTime = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ssZ")

$body = @{
    cluster_guid = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
    infobase_guid = "d723aefd-7992-420d-b5f9-a273fd4146be"
    from = $fromTime
    to = $toTime
    name = "EXCP"
    mode = "minimal"
    limit = 10
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/tools/get_tech_log" `
    -Method POST `
    -ContentType "application/json" `
    -Body $body
```

### 4. get_new_errors

```powershell
$body = @{
    cluster_guid = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
    infobase_guid = "d723aefd-7992-420d-b5f9-a273fd4146be"
    limit = 10
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/tools/get_new_errors" `
    -Method POST `
    -ContentType "application/json" `
    -Body $body
```

## Тестирование через curl (для Linux/WSL)

### configure_techlog

```bash
curl -X POST http://localhost:8080/tools/configure_techlog \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_guid": "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
    "infobase_guid": "d723aefd-7992-420d-b5f9-a273fd4146be",
    "location": "D:\\TechLogs\\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\\d723aefd-7992-420d-b5f9-a273fd4146be",
    "history": 24,
    "format": "json",
    "events": ["EXCP", "QERR"],
    "properties": ["all"]
  }'
```

## Проверка валидации пути

Валидация пути проверяет, что:
1. Путь содержит 2 GUID'а (cluster_guid и infobase_guid)
2. GUID'ы в пути соответствуют переданным параметрам
3. Структура пути: `<base>/<cluster_guid>/<infobase_guid>`

**Примеры правильных путей:**
- `D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be`
- `C:\Logs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be`

**Примеры неправильных путей:**
- `D:\TechLogs` (нет GUID'ов)
- `D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3` (только один GUID)
- `D:\TechLogs\wrong-guid\d723aefd-7992-420d-b5f9-a273fd4146be` (неправильный cluster_guid)

## Отладка

### Просмотр логов MCP-сервера

Если запущен через Docker:
```powershell
docker logs -f 1c-log-mcp
```

Если запущен напрямую:
```powershell
# Логи выводятся в консоль
# Уровень логирования настраивается через LOG_LEVEL (debug/info/warn/error)
```

### Проверка конфигурации

Убедитесь, что:
- ClickHouse доступен (если используются инструменты чтения логов)
- `cluster_map.yaml` существует в корне проекта (опционально, для маппинга имен, образец в `configs/cluster_map.yaml`)
- Переменные окружения настроены (см. `deploy/docker/.env.example`)

## Примеры ответов

### Успешный configure_techlog

**HTTP 200 OK**
**Content-Type: application/xml**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
  <dump create="false"></dump>
  <log location="D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be" history="24" format="json">
    <event>
      <eq property="name" value="EXCP"></eq>
    </event>
    <event>
      <eq property="name" value="QERR"></eq>
    </event>
    <property name="all"></property>
  </log>
</config>
```

### Ошибка валидации

**HTTP 400 Bad Request**
**Content-Type: application/json**

```json
{
  "error": "Configuration validation failed",
  "message": "invalid techlog location path: path validation failed: expected 2 GUIDs in path, found 0: D:\\TechLogs"
}
```

---

**См. также:**
- [README.md](../../README.md) - общая информация о проекте
- [docs/mcp/usage.md](../mcp/usage.md) - использование MCP tools через IDE

