# Упрощенный скрипт для тестирования только configure_techlog
# Быстрая проверка валидации пути

param(
    [string]$MCPHost = "localhost",
    [int]$MCPPort = 8080
)

$baseUrl = "http://${MCPHost}:${MCPPort}"

# Тестовые GUID'ы
$testClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$testInfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"

Write-Host "=== Тест configure_techlog ===" -ForegroundColor Cyan
Write-Host "URL: $baseUrl/tools/configure_techlog" -ForegroundColor Gray
Write-Host ""

# Тест 1: Правильный путь
Write-Host "Тест 1: Правильный путь (должен пройти)" -ForegroundColor Yellow
$location = "D:\TechLogs\$testClusterGUID\$testInfobaseGUID"
$body = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $location
    history = 24
    format = "json"
    events = @("EXCP", "QERR")
    properties = @("all")
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/tools/configure_techlog" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body
    
    Write-Host "✓ Успех! Конфигурация создана:" -ForegroundColor Green
    Write-Host $response -ForegroundColor White
    $response | Out-File -FilePath "test_logcfg_valid.xml" -Encoding UTF8
    Write-Host "`nСохранено в test_logcfg_valid.xml" -ForegroundColor Gray
}
catch {
    Write-Host "✗ Ошибка: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host $_.ErrorDetails.Message -ForegroundColor Red
    }
}

Write-Host ""

# Тест 2: Неправильный путь (без GUID'ов)
Write-Host "Тест 2: Неправильный путь (должна быть ошибка валидации)" -ForegroundColor Yellow
$invalidLocation = "D:\TechLogs"
$invalidBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $invalidLocation
    history = 24
    format = "json"
    events = @("EXCP")
    properties = @("all")
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/tools/configure_techlog" `
        -Method POST `
        -ContentType "application/json" `
        -Body $invalidBody
    
    Write-Host "✗ ОШИБКА: Валидация не сработала!" -ForegroundColor Red
    Write-Host "Ответ: $response" -ForegroundColor Red
}
catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 400) {
        Write-Host "✓ Валидация работает! Ошибка возвращена:" -ForegroundColor Green
        try {
            $errorDetails = $_.ErrorDetails.Message | ConvertFrom-Json
            Write-Host "  Error: $($errorDetails.error)" -ForegroundColor Yellow
            Write-Host "  Message: $($errorDetails.message)" -ForegroundColor Yellow
        }
        catch {
            Write-Host "  $($_.ErrorDetails.Message)" -ForegroundColor Yellow
        }
    } else {
        Write-Host "✗ Неожиданная ошибка: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""

# Тест 3: Неправильный GUID в пути
Write-Host "Тест 3: Неправильный GUID в пути (должна быть ошибка)" -ForegroundColor Yellow
$wrongGuidLocation = "D:\TechLogs\wrong-cluster-guid\$testInfobaseGUID"
$wrongGuidBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $wrongGuidLocation
    history = 24
    format = "json"
    events = @("EXCP")
    properties = @("all")
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/tools/configure_techlog" `
        -Method POST `
        -ContentType "application/json" `
        -Body $wrongGuidBody
    
    Write-Host "✗ ОШИБКА: Валидация не сработала!" -ForegroundColor Red
}
catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 400) {
        Write-Host "✓ Валидация работает! Ошибка возвращена:" -ForegroundColor Green
        try {
            $errorDetails = $_.ErrorDetails.Message | ConvertFrom-Json
            Write-Host "  Error: $($errorDetails.error)" -ForegroundColor Yellow
            Write-Host "  Message: $($errorDetails.message)" -ForegroundColor Yellow
        }
        catch {
            Write-Host "  $($_.ErrorDetails.Message)" -ForegroundColor Yellow
        }
    } else {
        Write-Host "✗ Неожиданная ошибка: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host "`n=== Тестирование завершено ===" -ForegroundColor Cyan

