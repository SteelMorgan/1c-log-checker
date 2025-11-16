# PowerShell скрипт для тестирования MCP-сервера напрямую через HTTP
# Использует тестовые GUID'ы для проверки работы инструментов

param(
    [string]$MCPHost = "localhost",
    [int]$MCPPort = 8080
)

$baseUrl = "http://${MCPHost}:${MCPPort}"

# Тестовые GUID'ы
$testClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$testInfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"

Write-Host "=== Тестирование MCP-сервера ===" -ForegroundColor Cyan
Write-Host "URL: $baseUrl" -ForegroundColor Gray
Write-Host "Cluster GUID: $testClusterGUID" -ForegroundColor Gray
Write-Host "Infobase GUID: $testInfobaseGUID" -ForegroundColor Gray
Write-Host ""

# Функция для выполнения HTTP запроса
function Invoke-MCPRequest {
    param(
        [string]$Endpoint,
        [string]$Method = "POST",
        [object]$Body = $null
    )
    
    $url = "$baseUrl$Endpoint"
    
    try {
        $headers = @{
            "Content-Type" = "application/json"
        }
        
        $params = @{
            Uri = $url
            Method = $Method
            Headers = $headers
        }
        
        if ($Body) {
            $jsonBody = $Body | ConvertTo-Json -Depth 10
            $params.Body = $jsonBody
            Write-Host "Request body:" -ForegroundColor Yellow
            Write-Host $jsonBody -ForegroundColor DarkYellow
        }
        
        Write-Host "`n--- $Method $Endpoint ---" -ForegroundColor Green
        $response = Invoke-RestMethod @params -ErrorAction Stop
        
        Write-Host "Response:" -ForegroundColor Green
        if ($response -is [string]) {
            Write-Host $response -ForegroundColor White
        } else {
            $response | ConvertTo-Json -Depth 10 | Write-Host -ForegroundColor White
        }
        
        return $response
    }
    catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.ErrorDetails.Message) {
            Write-Host "Details: $($_.ErrorDetails.Message)" -ForegroundColor Red
        }
        return $null
    }
}

# 1. Проверка health check
Write-Host "`n=== 1. Health Check ===" -ForegroundColor Cyan
$health = Invoke-MCPRequest -Endpoint "/health" -Method "GET"
if ($health) {
    Write-Host "✓ Сервер работает" -ForegroundColor Green
} else {
    Write-Host "✗ Сервер не отвечает" -ForegroundColor Red
    exit 1
}

# 2. Тест configure_techlog (с валидацией пути)
Write-Host "`n=== 2. Тест configure_techlog ===" -ForegroundColor Cyan
$location = "D:\TechLogs\$testClusterGUID\$testInfobaseGUID"
$configureBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $location
    history = 24
    format = "json"
    events = @("EXCP", "QERR")
    properties = @("all")
}

$configResult = Invoke-MCPRequest -Endpoint "/tools/configure_techlog" -Body $configureBody

if ($configResult) {
    Write-Host "✓ Конфигурация создана успешно" -ForegroundColor Green
    # Сохранить в файл для проверки
    $configResult | Out-File -FilePath "test_logcfg.xml" -Encoding UTF8
    Write-Host "Конфигурация сохранена в test_logcfg.xml" -ForegroundColor Gray
}

# 3. Тест configure_techlog с неправильным путем (должна быть ошибка валидации)
Write-Host "`n=== 3. Тест configure_techlog (неправильный путь - должна быть ошибка) ===" -ForegroundColor Cyan
$invalidLocation = "D:\TechLogs"
$invalidBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $invalidLocation
    history = 24
    format = "json"
    events = @("EXCP")
    properties = @("all")
}

$invalidResult = Invoke-MCPRequest -Endpoint "/tools/configure_techlog" -Body $invalidBody
if (-not $invalidResult) {
    Write-Host "✓ Валидация работает - ошибка возвращена корректно" -ForegroundColor Green
}

# 4. Тест get_event_log (если ClickHouse доступен)
Write-Host "`n=== 4. Тест get_event_log ===" -ForegroundColor Cyan
$fromTime = (Get-Date).AddHours(-1).ToString("yyyy-MM-ddTHH:mm:ssZ")
$toTime = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ssZ")

$eventLogBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    from = $fromTime
    to = $toTime
    mode = "minimal"
    limit = 10
}

$eventLogResult = Invoke-MCPRequest -Endpoint "/tools/get_event_log" -Body $eventLogBody
if ($eventLogResult) {
    Write-Host "✓ Запрос выполнен" -ForegroundColor Green
}

# 5. Тест get_tech_log (если ClickHouse доступен)
Write-Host "`n=== 5. Тест get_tech_log ===" -ForegroundColor Cyan
$techLogBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    from = $fromTime
    to = $toTime
    name = "EXCP"
    mode = "minimal"
    limit = 10
}

$techLogResult = Invoke-MCPRequest -Endpoint "/tools/get_tech_log" -Body $techLogBody
if ($techLogResult) {
    Write-Host "✓ Запрос выполнен" -ForegroundColor Green
}

# 6. Тест get_new_errors (если ClickHouse доступен)
Write-Host "`n=== 6. Тест get_new_errors ===" -ForegroundColor Cyan
$newErrorsBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    limit = 10
}

$newErrorsResult = Invoke-MCPRequest -Endpoint "/tools/get_new_errors" -Body $newErrorsBody
if ($newErrorsResult) {
    Write-Host "✓ Запрос выполнен" -ForegroundColor Green
}

Write-Host "`n=== Тестирование завершено ===" -ForegroundColor Cyan

