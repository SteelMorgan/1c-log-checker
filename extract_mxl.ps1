# Extract event log records from .mxl files (MOXCEL format)
# This script parses bracket notation and extracts field values

param(
    [string]$MxlFile
)

$content = Get-Content $MxlFile -Raw -Encoding UTF8

# Extract records by finding patterns like {"#","13.11.2025 HH:MM:SS"}
$datePattern = '\{"#","(\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}:\d{2})"\}'
$textPattern = '\{"#","([^"]+)"\}'

# Find all date/time entries (these mark record boundaries)
$dates = [regex]::Matches($content, $datePattern)

Write-Host "Found $($dates.Count) timestamp entries in $MxlFile"
Write-Host ""

# Simple approach: extract all text values in order
$allTexts = [regex]::Matches($content, $textPattern)

$currentRecord = @{}
$recordCount = 0
$records = @()

foreach ($match in $allTexts) {
    $value = $match.Groups[1].Value

    # Skip header values (using byte comparison to avoid encoding issues)
    $headers = @('Дата, время', 'Разделение данных', 'Пользователь', 'Компьютер', 'Приложение', 'Событие', 'Комментарий', 'Метаданные', 'Данные', 'Представление данных', 'Сеанс', 'Транзакция', 'Статус транзакции', 'Рабочий сервер', 'Основной IP порт', 'Вспомогательный IP порт')
    $isHeader = $false
    foreach ($header in $headers) {
        if ($value -eq $header) {
            $isHeader = $true
            break
        }
    }
    if ($isHeader) {
        continue
    }

    # Check if this is a timestamp (starts a new record)
    if ($value -match '^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}:\d{2}$') {
        if ($currentRecord.Count -gt 0) {
            $records += [PSCustomObject]$currentRecord
            $recordCount++
        }
        $currentRecord = @{
            'DateTime' = $value
            'Fields' = @()
        }
    } else {
        # Add value to current record
        if ($currentRecord.ContainsKey('DateTime')) {
            $currentRecord['Fields'] += $value
        }
    }
}

# Add last record
if ($currentRecord.Count -gt 0) {
    $records += [PSCustomObject]$currentRecord
    $recordCount++
}

Write-Host "Extracted $recordCount records:"
Write-Host ""

foreach ($rec in $records) {
    Write-Host "[$($rec.DateTime)]"
    Write-Host "  Fields: $($rec.Fields -join ' | ')"
    Write-Host ""
}

# Output as JSON for easier parsing
$records | ConvertTo-Json -Depth 10 | Out-File "$MxlFile.json" -Encoding UTF8
Write-Host "Saved to: $MxlFile.json"
