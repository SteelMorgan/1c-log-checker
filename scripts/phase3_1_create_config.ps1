# Phase 3.1: Создание config файла для сбора логов
# Создает logcfg.xml через configure_techlog MCP tool

param(
    [string]$MCPHost = "localhost",
    [int]$MCPPort = 8080
)

$baseUrl = "http://${MCPHost}:${MCPPort}"

# Test GUIDs (из configs/cluster_map.yaml)
$testClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$testInfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"

# Location path (должен содержать cluster_guid и infobase_guid)
$location = "D:\TechLogs\$testClusterGUID\$testInfobaseGUID"

Write-Host "=== Phase 3.1: Создание config файла ===" -ForegroundColor Cyan
Write-Host "MCP Server: $baseUrl" -ForegroundColor Gray
Write-Host "Cluster GUID: $testClusterGUID" -ForegroundColor Gray
Write-Host "Infobase GUID: $testInfobaseGUID" -ForegroundColor Gray
Write-Host "Location: $location" -ForegroundColor Gray
Write-Host ""

# Проверка доступности MCP сервера
Write-Host "Проверка доступности MCP сервера..." -ForegroundColor Yellow
try {
    $healthCheck = Invoke-RestMethod -Uri "$baseUrl/health" -Method GET -ErrorAction Stop
    Write-Host "OK: MCP сервер доступен" -ForegroundColor Green
}
catch {
    Write-Host "ERROR: MCP сервер недоступен: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "  Убедитесь, что контейнер mcp-server запущен:" -ForegroundColor Yellow
    Write-Host "  docker-compose -f deploy/docker/docker-compose.yml up -d mcp-server" -ForegroundColor Yellow
    exit 1
}

Write-Host ""

# Подготовка запроса
$body = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $location
    history = 24
    format = "json"
    events = @("EXCP", "QERR")
    properties = @("all")
} | ConvertTo-Json -Depth 10

Write-Host "Вызов configure_techlog..." -ForegroundColor Yellow
Write-Host "  Events: EXCP, QERR" -ForegroundColor Gray
Write-Host "  Format: JSON" -ForegroundColor Gray
Write-Host "  History: 24 часа" -ForegroundColor Gray
Write-Host ""

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/tools/configure_techlog" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body `
        -ErrorAction Stop
    
    Write-Host "OK: Конфигурация создана успешно!" -ForegroundColor Green
    Write-Host ""
    
    # Проверка содержимого ответа
    Write-Host "=== Содержимое logcfg.xml ===" -ForegroundColor Cyan
    Write-Host $response -ForegroundColor White
    Write-Host ""
    
    # Проверка ключевых параметров
    $formatCheck = $response -match 'format="json"'
    $clusterCheck = $response -match [regex]::Escape($testClusterGUID)
    $infobaseCheck = $response -match [regex]::Escape($testInfobaseGUID)
    $excpCheck = $response -match "EXCP"
    $qerrCheck = $response -match "QERR"
    $propertyCheck = $response -match 'property name="all"'
    
    Write-Host "=== Проверка параметров ===" -ForegroundColor Cyan
    if ($formatCheck) {
        Write-Host "  OK: format=`"json`"" -ForegroundColor Green
    } else {
        Write-Host "  ERROR: format=`"json`"" -ForegroundColor Red
    }
    
    if ($clusterCheck) {
        Write-Host "  OK: location содержит cluster_guid" -ForegroundColor Green
    } else {
        Write-Host "  ERROR: location содержит cluster_guid" -ForegroundColor Red
    }
    
    if ($infobaseCheck) {
        Write-Host "  OK: location содержит infobase_guid" -ForegroundColor Green
    } else {
        Write-Host "  ERROR: location содержит infobase_guid" -ForegroundColor Red
    }
    
    if ($excpCheck) {
        Write-Host "  OK: событие EXCP" -ForegroundColor Green
    } else {
        Write-Host "  ERROR: событие EXCP" -ForegroundColor Red
    }
    
    if ($qerrCheck) {
        Write-Host "  OK: событие QERR" -ForegroundColor Green
    } else {
        Write-Host "  ERROR: событие QERR" -ForegroundColor Red
    }
    
    if ($propertyCheck) {
        Write-Host "  OK: property all" -ForegroundColor Green
    } else {
        Write-Host "  ERROR: property all" -ForegroundColor Red
    }
    
    Write-Host ""
    
    $allPassed = $formatCheck -and $clusterCheck -and $infobaseCheck -and $excpCheck -and $qerrCheck -and $propertyCheck
    
    if ($allPassed) {
        Write-Host "OK: Все проверки пройдены!" -ForegroundColor Green
        Write-Host ""
        Write-Host "Следующий шаг: Phase 3.2 - Генерация логов через unit-тесты в 1C" -ForegroundColor Cyan
        Write-Host "  После запуска unit-тестов логи будут созданы в:" -ForegroundColor Gray
        Write-Host "  $location\rphost_XXXX\*.log" -ForegroundColor Gray
    } else {
        Write-Host "ERROR: Некоторые проверки не пройдены!" -ForegroundColor Red
        exit 1
    }
    
    # Сохраняем XML в файл для справки
    $outputFile = "phase3_1_logcfg.xml"
    $response | Out-File -FilePath $outputFile -Encoding UTF8
    Write-Host ""
    Write-Host "XML сохранен в: $outputFile" -ForegroundColor Gray
    
}
catch {
    Write-Host "ERROR: Ошибка при создании конфигурации:" -ForegroundColor Red
    Write-Host "  $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.ErrorDetails.Message) {
        Write-Host ""
        Write-Host "Детали ошибки:" -ForegroundColor Yellow
        try {
            $errorDetails = $_.ErrorDetails.Message | ConvertFrom-Json
            Write-Host "  Error: $($errorDetails.error)" -ForegroundColor Red
            Write-Host "  Message: $($errorDetails.message)" -ForegroundColor Red
        }
        catch {
            Write-Host "  $($_.ErrorDetails.Message)" -ForegroundColor Red
        }
    }
    
    exit 1
}
