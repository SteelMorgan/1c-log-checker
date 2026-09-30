# Script to analyze duplicate records from parser logs

$logFile = "D:\My Projects\FrameWork 1C\1c-log-checker\logs\parser.log"

Write-Host "Analyzing duplicate records..." -ForegroundColor Cyan
Write-Host ""

# Find all duplicate records with full details
Write-Host "=== DUPLICATE RECORDS DETAILS ===" -ForegroundColor Yellow
Write-Host ""

# Get lines with "CRITICAL: Found duplicate records in batch" and next "CRITICAL: Duplicate record details"
$lines = Get-Content $logFile

$duplicateGroups = @()
$currentGroup = $null

for ($i = 0; $i < $lines.Count; $i++) {
    $line = $lines[$i]

    # Check if this is a "Found duplicate records in batch" line
    if ($line -match '"message":"CRITICAL: Found duplicate records in batch') {
        if ($currentGroup) {
            $duplicateGroups += $currentGroup
        }
        $currentGroup = @{
            FirstRecord = $null
            Duplicates = @()
        }

        try {
            $json = $line | ConvertFrom-Json
            $currentGroup.FirstRecord = $json
        } catch {
            Write-Host "Failed to parse JSON for first record" -ForegroundColor Red
        }
    }
    # Check if this is a "Duplicate record details" line
    elseif ($line -match '"message":"CRITICAL: Duplicate record details') {
        if ($currentGroup) {
            try {
                $json = $line | ConvertFrom-Json
                $currentGroup.Duplicates += $json
            } catch {
                Write-Host "Failed to parse JSON for duplicate record" -ForegroundColor Red
            }
        }
    }
}

if ($currentGroup) {
    $duplicateGroups += $currentGroup
}

Write-Host "Found $($duplicateGroups.Count) duplicate groups" -ForegroundColor Green
Write-Host ""

# Analyze each duplicate group
foreach ($group in $duplicateGroups) {
    $first = $group.FirstRecord
    if (-not $first) { continue }

    Write-Host "=== Duplicate Group ===" -ForegroundColor Cyan
    Write-Host "Hash: $($first.hash)" -ForegroundColor White
    Write-Host "Duplicate Count: $($first.duplicate_count)" -ForegroundColor Yellow
    Write-Host ""

    Write-Host "First Record:" -ForegroundColor Green
    Write-Host "  Event Time: $($first.first_event_time)"
    Write-Host "  Transaction DateTime: $($first.first_transaction_datetime)"
    Write-Host "  Event: $($first.first_event)"
    Write-Host "  Level: $($first.first_level)"
    Write-Host "  User: $($first.first_user)"
    Write-Host "  Computer: $($first.first_computer)"
    Write-Host "  Session ID: $($first.first_session_id)"
    Write-Host "  Connection ID: $($first.first_connection_id)"
    Write-Host "  Transaction ID: $($first.first_transaction_id)"
    Write-Host "  Transaction Number: $($first.first_transaction_number)"
    Write-Host "  Comment: $($first.first_comment)"
    Write-Host "  Data: $($first.first_data)"
    Write-Host ""

    $dupIndex = 1
    foreach ($dup in $group.Duplicates) {
        Write-Host "Duplicate #$dupIndex (index $($dup.duplicate_index)):" -ForegroundColor Red
        Write-Host "  Event Time: $($dup.event_time)"
        Write-Host "  Transaction DateTime: $($dup.transaction_datetime)"
        Write-Host "  Event: $($dup.event)"
        Write-Host "  Level: $($dup.level)"
        Write-Host "  User: $($dup.user)"
        Write-Host "  Computer: $($dup.computer)"
        Write-Host "  Session ID: $($dup.session_id)"
        Write-Host "  Connection ID: $($dup.connection_id)"
        Write-Host "  Transaction ID: $($dup.transaction_id)"
        Write-Host "  Transaction Number: $($dup.transaction_number)"
        Write-Host "  Comment: $($dup.comment)"
        Write-Host "  Data: $($dup.data)"
        Write-Host "  Data Presentation: $($dup.data_presentation)"
        Write-Host "  Event Presentation: $($dup.event_presentation)"
        Write-Host "  Metadata Name: $($dup.metadata_name)"
        Write-Host "  Metadata Presentation: $($dup.metadata_presentation)"
        Write-Host "  Server: $($dup.server)"
        Write-Host "  Primary Port: $($dup.primary_port)"
        Write-Host "  Secondary Port: $($dup.secondary_port)"
        Write-Host "  Application: $($dup.application)"
        Write-Host "  Data Separation: $($dup.data_separation)"
        Write-Host "  Transaction Status: $($dup.transaction_status)"
        Write-Host "  Properties: $($dup.properties)"
        Write-Host ""

        # Compare with first record
        Write-Host "  Comparison with first record:" -ForegroundColor Magenta
        if ($dup.event_time -ne $first.first_event_time) {
            Write-Host "    Event Time DIFFERS: '$($dup.event_time)' vs '$($first.first_event_time)'" -ForegroundColor Red
        }
        if ($dup.transaction_datetime -ne $first.first_transaction_datetime) {
            Write-Host "    Transaction DateTime DIFFERS: '$($dup.transaction_datetime)' vs '$($first.first_transaction_datetime)'" -ForegroundColor Red
        }
        if ($dup.event -ne $first.first_event) {
            Write-Host "    Event DIFFERS: '$($dup.event)' vs '$($first.first_event)'" -ForegroundColor Red
        }
        if ($dup.level -ne $first.first_level) {
            Write-Host "    Level DIFFERS: '$($dup.level)' vs '$($first.first_level)'" -ForegroundColor Red
        }
        if ($dup.user -ne $first.first_user) {
            Write-Host "    User DIFFERS: '$($dup.user)' vs '$($first.first_user)'" -ForegroundColor Red
        }
        if ($dup.comment -ne $first.first_comment) {
            Write-Host "    Comment DIFFERS: '$($dup.comment)' vs '$($first.first_comment)'" -ForegroundColor Red
        }
        if ($dup.data -ne $first.first_data) {
            Write-Host "    Data DIFFERS: '$($dup.data)' vs '$($first.first_data)'" -ForegroundColor Red
        }

        Write-Host ""
        $dupIndex++
    }

    Write-Host "===========================================" -ForegroundColor Gray
    Write-Host ""
}

Write-Host "Analysis complete." -ForegroundColor Green
