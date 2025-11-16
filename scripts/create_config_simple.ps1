# Phase 3.1: Создание config файла для сбора логов
# Использует путь из TECHLOG_DIRS для формирования location

# Test GUIDs (из configs/cluster_map.yaml)
$testClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$testInfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"

# Получаем базовый путь из TECHLOG_DIRS
# Сначала пробуем из переменной окружения
$techLogDirs = $env:TECHLOG_DIRS

# Если не найдено, читаем из .env файла
if (-not $techLogDirs) {
    if (Test-Path ".env") {
        $line = Get-Content .env | Select-String "^TECHLOG_DIRS="
        if ($line) {
            $techLogDirs = ($line.ToString() -replace "^TECHLOG_DIRS=", "").Trim()
        }
    }
}

# Если все еще не найдено, используем значение по умолчанию из env.example
if (-not $techLogDirs) {
    $techLogDirs = "C:\ProgramData\1C\1Cv8\logs"
    Write-Host "WARNING: TECHLOG_DIRS not found, using default: $techLogDirs" -ForegroundColor Yellow
}

# Формируем полный путь согласно структуре: <TECHLOG_DIRS>/<cluster_guid>/<infobase_guid>
# Нормализуем разделители для кроссплатформенности (в коде используется forward slash)
$basePath = $techLogDirs -replace "\\", "/"
$location = "$basePath/$testClusterGUID/$testInfobaseGUID"

Write-Host "=== Phase 3.1: Config creation ===" -ForegroundColor Cyan
Write-Host "Base path (TECHLOG_DIRS): $techLogDirs" -ForegroundColor Gray
Write-Host "Full location path: $location" -ForegroundColor Gray
Write-Host ""

$body = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $location
    history = 24
    format = "json"
    events = @("EXCP", "QERR")
    properties = @("all")
} | ConvertTo-Json -Depth 10

$response = Invoke-RestMethod -Uri "http://localhost:8080/tools/configure_techlog" -Method POST -ContentType "application/json" -Body $body

Write-Host "=== Phase 3.1: Config created ===" -ForegroundColor Green
Write-Host $response
Write-Host ""

Write-Host "Checking file:" -ForegroundColor Yellow
Start-Sleep -Seconds 1

if (Test-Path "configs\techlog\logcfg.xml") {
    $content = Get-Content "configs\techlog\logcfg.xml" -Raw
    Write-Host "OK: File exists" -ForegroundColor Green
    Write-Host "Size: $((Get-Item 'configs\techlog\logcfg.xml').Length) bytes" -ForegroundColor Gray
    
    if ($content.Length -gt 0) {
        Write-Host "OK: File is not empty" -ForegroundColor Green
        Write-Host ""
        Write-Host "Content:" -ForegroundColor Cyan
        Write-Host $content
    } else {
        Write-Host "ERROR: File is empty" -ForegroundColor Red
    }
} else {
    Write-Host "ERROR: File not found" -ForegroundColor Red
}
