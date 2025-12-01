# Скрипт для проверки данных в ClickHouse для метода logc_get_event_log
# Параметры запроса:
# - cluster_id = b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3
# - base_id = e6686d6f-1c82-4aed-9981-e4a9908bdba3
# - mode = minimal
# - level = error
# - limit = 100

$clusterGuid = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$infobaseGuid = "e6686d6f-1c82-4aed-9981-e4a9908bdba3"
$level = "error"
$limit = 100

Write-Host "=== Проверка данных в ClickHouse ===" -ForegroundColor Cyan
Write-Host ""

# Подключение к ClickHouse (из docker-compose)
$clickhouseHost = "localhost"
$clickhousePort = 8123
$clickhouseDatabase = "logs"

# 1. Проверка наличия данных для указанных GUID
Write-Host "1. Проверка наличия записей для указанных GUID..." -ForegroundColor Yellow
$checkQuery = @"
SELECT 
    count() as total_count,
    min(event_time) as min_time,
    max(event_time) as max_time,
    uniq(level) as unique_levels,
    groupArray(level) as all_levels
FROM logs.event_log
WHERE cluster_guid = '$clusterGuid' 
  AND infobase_guid = '$infobaseGuid'
FORMAT JSON
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $checkQuery `
        -ContentType "text/plain; charset=utf-8"
    
    $data = $response.data[0]
    Write-Host "  Всего записей: $($data.total_count)" -ForegroundColor Green
    Write-Host "  Минимальное время: $($data.min_time)" -ForegroundColor Green
    Write-Host "  Максимальное время: $($data.max_time)" -ForegroundColor Green
    Write-Host "  Уникальных уровней: $($data.unique_levels)" -ForegroundColor Green
    Write-Host "  Все уровни: $($data.all_levels -join ', ')" -ForegroundColor Green
    Write-Host ""
} catch {
    Write-Host "  ОШИБКА: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host ""
}

# 2. Проверка записей с уровнем Error/Ошибка
Write-Host "2. Проверка записей с уровнем Error/Ошибка..." -ForegroundColor Yellow
$errorQuery = @"
SELECT 
    count() as error_count,
    min(event_time) as min_time,
    max(event_time) as max_time
FROM logs.event_log
WHERE cluster_guid = '$clusterGuid' 
  AND infobase_guid = '$infobaseGuid'
  AND level IN ('Error', 'Ошибка')
FORMAT JSON
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $errorQuery `
        -ContentType "text/plain; charset=utf-8"
    
    $data = $response.data[0]
    Write-Host "  Записей с уровнем Error/Ошибка: $($data.error_count)" -ForegroundColor Green
    if ($data.error_count -gt 0) {
        Write-Host "  Минимальное время: $($data.min_time)" -ForegroundColor Green
        Write-Host "  Максимальное время: $($data.max_time)" -ForegroundColor Green
    }
    Write-Host ""
} catch {
    Write-Host "  ОШИБКА: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host ""
}

# 3. Проверка записей за последние 10 минут (дефолтный диапазон)
Write-Host "3. Проверка записей за последние 10 минут..." -ForegroundColor Yellow
$now = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
$tenMinutesAgo = (Get-Date).AddMinutes(-10).ToString("yyyy-MM-ddTHH:mm:ssZ")

$recentQuery = @"
SELECT 
    count() as recent_count
FROM logs.event_log
WHERE cluster_guid = '$clusterGuid' 
  AND infobase_guid = '$infobaseGuid'
  AND level IN ('Error', 'Ошибка')
  AND event_time BETWEEN '$tenMinutesAgo' AND '$now'
FORMAT JSON
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $recentQuery `
        -ContentType "text/plain; charset=utf-8"
    
    $data = $response.data[0]
    Write-Host "  Записей за последние 10 минут: $($data.recent_count)" -ForegroundColor Green
    Write-Host "  Диапазон: $tenMinutesAgo - $now" -ForegroundColor Gray
    Write-Host ""
} catch {
    Write-Host "  ОШИБКА: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host ""
}

# 4. Выполнение запроса как в методе GetEventLog (minimal mode)
Write-Host "4. Выполнение запроса как в методе GetEventLog (minimal mode)..." -ForegroundColor Yellow
$minimalQuery = @"
SELECT
    event_time,
    level,
    event_presentation,
    user_name,
    comment,
    metadata_presentation
FROM logs.event_log
WHERE cluster_guid = '$clusterGuid' 
  AND infobase_guid = '$infobaseGuid'
  AND event_time BETWEEN '$tenMinutesAgo' AND '$now'
  AND level IN ('Error', 'Ошибка')
ORDER BY event_time DESC 
LIMIT $limit
FORMAT JSON
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $minimalQuery `
        -ContentType "text/plain; charset=utf-8"
    
    $count = $response.data.Count
    Write-Host "  Найдено записей: $count" -ForegroundColor Green
    if ($count -gt 0) {
        Write-Host "  Первая запись:" -ForegroundColor Gray
        $first = $response.data[0]
        Write-Host "    event_time: $($first.event_time)" -ForegroundColor Gray
        Write-Host "    level: $($first.level)" -ForegroundColor Gray
        Write-Host "    event_presentation: $($first.event_presentation)" -ForegroundColor Gray
        Write-Host "    user_name: $($first.user_name)" -ForegroundColor Gray
    }
    Write-Host ""
} catch {
    Write-Host "  ОШИБКА: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host ""
}

# 5. Проверка всех записей (без временного ограничения)
Write-Host "5. Проверка всех записей (без временного ограничения)..." -ForegroundColor Yellow
$allQuery = @"
SELECT
    event_time,
    level,
    event_presentation,
    user_name,
    comment,
    metadata_presentation
FROM logs.event_log
WHERE cluster_guid = '$clusterGuid' 
  AND infobase_guid = '$infobaseGuid'
  AND level IN ('Error', 'Ошибка')
ORDER BY event_time DESC 
LIMIT $limit
FORMAT JSON
"@

try {
    $response = Invoke-RestMethod -Uri "http://${clickhouseHost}:${clickhousePort}/?database=${clickhouseDatabase}" `
        -Method Post `
        -Body $allQuery `
        -ContentType "text/plain; charset=utf-8"
    
    $count = $response.data.Count
    Write-Host "  Найдено записей: $count" -ForegroundColor Green
    if ($count -gt 0) {
        Write-Host "  Первая запись:" -ForegroundColor Gray
        $first = $response.data[0]
        Write-Host "    event_time: $($first.event_time)" -ForegroundColor Gray
        Write-Host "    level: $($first.level)" -ForegroundColor Gray
        Write-Host "    event_presentation: $($first.event_presentation)" -ForegroundColor Gray
        Write-Host "    user_name: $($first.user_name)" -ForegroundColor Gray
        Write-Host ""
        Write-Host "  Последняя запись:" -ForegroundColor Gray
        $last = $response.data[-1]
        Write-Host "    event_time: $($last.event_time)" -ForegroundColor Gray
        Write-Host "    level: $($last.level)" -ForegroundColor Gray
        Write-Host "    event_presentation: $($last.event_presentation)" -ForegroundColor Gray
        Write-Host "    user_name: $($last.user_name)" -ForegroundColor Gray
    }
    Write-Host ""
} catch {
    Write-Host "  ОШИБКА: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host ""
}

Write-Host "=== Проверка завершена ===" -ForegroundColor Cyan



