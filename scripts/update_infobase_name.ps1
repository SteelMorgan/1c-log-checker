# Скрипт для обновления infobase_name в таблице event_log
# Заполняет пустые значения как 'DSSL_WRK'

$clickhouseHost = "localhost"
$clickhousePort = 8123
$clickhouseDatabase = "logs"

Write-Host "=== Обновление infobase_name в event_log ===" -ForegroundColor Cyan
Write-Host ""

# 1. Проверка количества строк с пустым infobase_name
Write-Host "1. Проверка количества строк с пустым infobase_name..." -ForegroundColor Yellow
$checkQuery = @"
SELECT 
    count() as empty_count,
    countIf(infobase_name = '') as empty_string_count,
    countIf(isNull(infobase_name)) as null_count
FROM logs.event_log
WHERE infobase_name = '' OR isNull(infobase_name)
FORMAT JSON
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $checkQuery `
        -ContentType "text/plain; charset=utf-8"
    
    $data = $response.data[0]
    $emptyCount = $data.empty_count
    
    Write-Host "  Всего строк с пустым infobase_name: $emptyCount" -ForegroundColor Green
    Write-Host "  Из них пустых строк: $($data.empty_string_count)" -ForegroundColor Gray
    Write-Host "  Из них NULL: $($data.null_count)" -ForegroundColor Gray
    Write-Host ""
    
    if ($emptyCount -eq 0) {
        Write-Host "  Нет строк для обновления. Завершение." -ForegroundColor Yellow
        exit 0
    }
} catch {
    Write-Host "  ОШИБКА при проверке: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "  Детали: $($_.Exception)" -ForegroundColor Red
    exit 1
}

# 2. Подтверждение обновления
Write-Host "2. Подтверждение обновления..." -ForegroundColor Yellow
Write-Host "  Будет обновлено строк: $emptyCount" -ForegroundColor Yellow
Write-Host "  Новое значение: DSSL_WRK" -ForegroundColor Yellow
Write-Host ""
$confirm = Read-Host "  Продолжить? (y/n)"
if ($confirm -ne "y" -and $confirm -ne "Y") {
    Write-Host "  Обновление отменено." -ForegroundColor Yellow
    exit 0
}

# 3. Выполнение обновления
Write-Host "3. Выполнение обновления..." -ForegroundColor Yellow
$updateQuery = @"
ALTER TABLE logs.event_log 
UPDATE infobase_name = 'DSSL_WRK' 
WHERE infobase_name = '' OR isNull(infobase_name)
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $updateQuery `
        -ContentType "text/plain; charset=utf-8"
    
    Write-Host "  Обновление выполнено успешно!" -ForegroundColor Green
    Write-Host "  Ответ сервера: $response" -ForegroundColor Gray
    Write-Host ""
} catch {
    Write-Host "  ОШИБКА при обновлении: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "  Детали: $($_.Exception)" -ForegroundColor Red
    exit 1
}

# 4. Проверка результата
Write-Host "4. Проверка результата..." -ForegroundColor Yellow
Start-Sleep -Seconds 2  # Небольшая задержка для завершения обновления

$verifyQuery = @"
SELECT 
    count() as empty_count,
    countIf(infobase_name = 'DSSL_WRK') as updated_count
FROM logs.event_log
WHERE (infobase_name = '' OR isNull(infobase_name)) OR infobase_name = 'DSSL_WRK'
FORMAT JSON
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $verifyQuery `
        -ContentType "text/plain; charset=utf-8"
    
    $data = $response.data[0]
    Write-Host "  Осталось пустых строк: $($data.empty_count)" -ForegroundColor $(if ($data.empty_count -eq 0) { "Green" } else { "Yellow" })
    Write-Host "  Обновлено строк: $($data.updated_count)" -ForegroundColor Green
    Write-Host ""
} catch {
    Write-Host "  ОШИБКА при проверке: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "  Детали: $($_.Exception)" -ForegroundColor Red
}

Write-Host "=== Обновление завершено ===" -ForegroundColor Cyan

