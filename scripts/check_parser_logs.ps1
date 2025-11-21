# Script to check parser logs for errors, warnings, and duplicates

$logFile = "D:\My Projects\FrameWork 1C\1c-log-checker\logs\parser.log"

Write-Host "Checking parser logs for problems..." -ForegroundColor Cyan
Write-Host ""

# Search for errors
Write-Host "=== ERRORS ===" -ForegroundColor Red
Select-String -Path $logFile -Pattern '"level":"error"' | Select-Object -Last 20 | ForEach-Object {
    $line = $_.Line
    if ($line.Length -gt 300) {
        $line = $line.Substring(0, 300) + "..."
    }
    Write-Host $line -ForegroundColor Red
}

Write-Host ""

# Search for warnings
Write-Host "=== WARNINGS ===" -ForegroundColor Yellow
Select-String -Path $logFile -Pattern '"level":"warn"' | Select-Object -Last 20 | ForEach-Object {
    $line = $_.Line
    if ($line.Length -gt 300) {
        $line = $line.Substring(0, 300) + "..."
    }
    Write-Host $line -ForegroundColor Yellow
}

Write-Host ""

# Search for duplicates
Write-Host "=== DUPLICATES ===" -ForegroundColor Magenta
Select-String -Path $logFile -Pattern 'duplicate' -CaseSensitive:$false | Select-Object -Last 20 | ForEach-Object {
    $line = $_.Line
    if ($line.Length -gt 300) {
        $line = $line.Substring(0, 300) + "..."
    }
    Write-Host $line -ForegroundColor Magenta
}

Write-Host ""

# Search for "Failed to extract GUIDs"
Write-Host "=== FAILED TO EXTRACT GUIDs ===" -ForegroundColor Red
Select-String -Path $logFile -Pattern 'Failed to extract' | Select-Object -Last 20 | ForEach-Object {
    $line = $_.Line
    if ($line.Length -gt 300) {
        $line = $line.Substring(0, 300) + "..."
    }
    Write-Host $line -ForegroundColor Red
}

Write-Host ""
Write-Host "Done checking logs." -ForegroundColor Green
