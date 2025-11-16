# Phase 3.2: Проверка создания логов после unit-тестов
# Проверяет наличие .log файлов в каталоге techlog

param(
    [int]$WaitSeconds = 5  # Время ожидания после запуска тестов
)

# Получаем базовый путь из TECHLOG_DIRS
$techLogDirs = $env:TECHLOG_DIRS
if (-not $techLogDirs) {
    if (Test-Path ".env") {
        $line = Get-Content .env | Select-String "^TECHLOG_DIRS="
        if ($line) {
            $techLogDirs = ($line.ToString() -replace "^TECHLOG_DIRS=", "").Trim()
        }
    }
}

if (-not $techLogDirs) {
    $techLogDirs = "C:\ProgramData\1C\1Cv8\logs"
    Write-Host "WARNING: TECHLOG_DIRS not found, using default: $techLogDirs" -ForegroundColor Yellow
}

# Test GUIDs
$testClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$testInfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"

# Формируем путь к каталогу логов
$logDir = Join-Path $techLogDirs $testClusterGUID
$logDir = Join-Path $logDir $testInfobaseGUID

Write-Host "=== Phase 3.2: Checking for generated logs ===" -ForegroundColor Cyan
Write-Host "Log directory: $logDir" -ForegroundColor Gray
Write-Host "Waiting $WaitSeconds seconds for logs to be created..." -ForegroundColor Yellow
Write-Host ""

Start-Sleep -Seconds $WaitSeconds

# Проверяем существование каталога
if (-not (Test-Path $logDir)) {
    Write-Host "ERROR: Log directory does not exist: $logDir" -ForegroundColor Red
    Write-Host "  Make sure:" -ForegroundColor Yellow
    Write-Host "  1. Unit tests were executed in 1C" -ForegroundColor Yellow
    Write-Host "  2. logcfg.xml is properly configured" -ForegroundColor Yellow
    Write-Host "  3. 1C platform has write permissions to the directory" -ForegroundColor Yellow
    exit 1
}

Write-Host "OK: Log directory exists" -ForegroundColor Green
Write-Host ""

# Ищем .log файлы в подкаталогах (rphost_XXXX, 1cv8c_XXXX, etc.)
$logFiles = Get-ChildItem -Path $logDir -Recurse -Filter "*.log" -ErrorAction SilentlyContinue

if ($logFiles.Count -eq 0) {
    Write-Host "WARNING: No .log files found in: $logDir" -ForegroundColor Yellow
    Write-Host "  Subdirectories found:" -ForegroundColor Gray
    $subdirs = Get-ChildItem -Path $logDir -Directory -ErrorAction SilentlyContinue
    if ($subdirs) {
        foreach ($subdir in $subdirs) {
            Write-Host "    - $($subdir.Name)" -ForegroundColor Gray
        }
    } else {
        Write-Host "    (no subdirectories)" -ForegroundColor Gray
    }
    Write-Host ""
    Write-Host "  Make sure:" -ForegroundColor Yellow
    Write-Host "  1. Unit tests were executed and generated exceptions/queries" -ForegroundColor Yellow
    Write-Host "  2. 1C platform process (rphost, 1cv8c) is running" -ForegroundColor Yellow
    Write-Host "  3. logcfg.xml is in the correct location and readable by 1C" -ForegroundColor Yellow
    exit 1
}

Write-Host "OK: Found $($logFiles.Count) log file(s)" -ForegroundColor Green
Write-Host ""

# Показываем информацию о файлах
Write-Host "=== Log files ===" -ForegroundColor Cyan
foreach ($file in $logFiles) {
    Write-Host "File: $($file.FullName)" -ForegroundColor White
    Write-Host "  Size: $($file.Length) bytes" -ForegroundColor Gray
    Write-Host "  Created: $($file.CreationTime)" -ForegroundColor Gray
    Write-Host "  Modified: $($file.LastWriteTime)" -ForegroundColor Gray
    
    # Проверяем формат (JSON или text)
    $firstLine = Get-Content $file.FullName -First 1 -ErrorAction SilentlyContinue
    if ($firstLine -match '^\s*\{') {
        Write-Host "  Format: JSON" -ForegroundColor Green
    } else {
        Write-Host "  Format: Text" -ForegroundColor Yellow
    }
    
    Write-Host ""
}

# Проверяем содержимое первого файла на наличие событий EXCP и QERR
Write-Host "=== Checking for events ===" -ForegroundColor Cyan
$firstFile = $logFiles[0]
$content = Get-Content $firstFile.FullName -Raw -ErrorAction SilentlyContinue

if ($content) {
    $hasEXCP = $content -match '"name"\s*:\s*"EXCP"' -or $content -match 'EXCP'
    $hasQERR = $content -match '"name"\s*:\s*"QERR"' -or $content -match 'QERR'
    
    if ($hasEXCP) {
        Write-Host "OK: Found EXCP events" -ForegroundColor Green
    } else {
        Write-Host "WARNING: No EXCP events found" -ForegroundColor Yellow
    }
    
    if ($hasQERR) {
        Write-Host "OK: Found QERR events" -ForegroundColor Green
    } else {
        Write-Host "WARNING: No QERR events found" -ForegroundColor Yellow
    }
    
    # Показываем первые несколько строк для проверки формата
    Write-Host ""
    Write-Host "First 3 lines of log file:" -ForegroundColor Cyan
    $firstLines = Get-Content $firstFile.FullName -First 3 -ErrorAction SilentlyContinue
    foreach ($line in $firstLines) {
        Write-Host "  $line" -ForegroundColor Gray
    }
} else {
    Write-Host "WARNING: Could not read file content" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== Phase 3.2: Log generation check complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "Next step: Phase 3.3 - Manual verification of generated logs" -ForegroundColor Cyan


