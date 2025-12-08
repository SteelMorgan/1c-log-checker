# Скрипт для автоматического определения часового пояса хоста
# и установки переменной окружения TZ для Docker контейнеров

# Маппинг Windows TimeZone ID в IANA TimeZone
$timezoneMap = @{
    'Russian Standard Time' = 'Europe/Moscow'
    'UTC' = 'UTC'
    'GMT Standard Time' = 'Europe/London'
    'Central European Standard Time' = 'Europe/Berlin'
    'Eastern Standard Time' = 'America/New_York'
    'Pacific Standard Time' = 'America/Los_Angeles'
    'Tokyo Standard Time' = 'Asia/Tokyo'
    'China Standard Time' = 'Asia/Shanghai'
    'India Standard Time' = 'Asia/Kolkata'
    'AUS Eastern Standard Time' = 'Australia/Sydney'
}

# Получаем часовой пояс хоста
$hostTimezone = [System.TimeZoneInfo]::Local.Id
Write-Host "Host timezone: $hostTimezone"

# Преобразуем в IANA формат
if ($timezoneMap.ContainsKey($hostTimezone)) {
    $ianaTimezone = $timezoneMap[$hostTimezone]
    Write-Host "IANA timezone: $ianaTimezone"
    $env:TZ = $ianaTimezone
    Write-Host "Environment variable TZ set to: $env:TZ"
} else {
    Write-Warning "Unknown timezone '$hostTimezone', using UTC"
    $env:TZ = 'UTC'
}

# Экспортируем для использования в docker-compose
return $env:TZ






















