# Test event_log MCP tool and verify results against ClickHouse
# This script:
# 1. Calls MCP tool logc_get_event_log with parameters
# 2. Extracts query parameters from the call
# 3. Executes direct SQL query to ClickHouse with same parameters
# 4. Compares results

param(
    [string]$MCPUrl = "http://localhost:8080",
    [string]$ClickHouseHost = "localhost",
    [int]$ClickHousePort = 8123,
    [string]$ClickHouseDB = "logs",
    [string]$ClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
    [string]$InfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be",
    [string]$From = "",
    [string]$To = "",
    [string]$Level = "",
    [string]$Mode = "minimal",
    [int]$Limit = 10
)

# Set default time range (last 24 hours) if not specified
if ([string]::IsNullOrEmpty($From)) {
    $From = (Get-Date).AddDays(-1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
}
if ([string]::IsNullOrEmpty($To)) {
    $To = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
}

Write-Host ""
Write-Host "=== Event Log MCP Tool Verification Test ===" -ForegroundColor Cyan
Write-Host ""

# Display test parameters
Write-Host "Test Parameters:" -ForegroundColor Yellow
Write-Host "  Cluster GUID: $ClusterGUID"
Write-Host "  Infobase GUID: $InfobaseGUID"
Write-Host "  From: $From"
Write-Host "  To: $To"
Write-Host "  Level: $(if ([string]::IsNullOrEmpty($Level)) { '(all)' } else { $Level })"
Write-Host "  Mode: $Mode"
Write-Host "  Limit: $Limit"
Write-Host ""

# Step 1: Call MCP tool
Write-Host "Step 1: Calling MCP tool logc_get_event_log..." -ForegroundColor Cyan

$mcpBody = @{
    cluster_guid = $ClusterGUID
    infobase_guid = $InfobaseGUID
    from = $From
    to = $To
    limit = $Limit
    mode = $Mode
}

if (-not [string]::IsNullOrEmpty($Level)) {
    $mcpBody.level = $Level
}

try {
    $jsonBody = $mcpBody | ConvertTo-Json -Depth 10
    Write-Host "MCP Request:" -ForegroundColor Gray
    Write-Host $jsonBody -ForegroundColor DarkGray
    Write-Host ""

    $mcpResponseRaw = Invoke-WebRequest -Uri "$MCPUrl/tools/logc_get_event_log" -Method Post -Body $jsonBody -ContentType "application/json" -ErrorAction Stop
    
    Write-Host 'MCP call successful' -ForegroundColor Green
    
    # MCP HTTP endpoint returns JSON string directly (not wrapped)
    $mcpResponseText = $mcpResponseRaw.Content
    $mcpResults = $mcpResponseText | ConvertFrom-Json
    
    Write-Host "  MCP returned $($mcpResults.Count) records" -ForegroundColor Gray
    Write-Host ""
    
} catch {
    Write-Host 'MCP call failed:' $_.Exception.Message -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $reader.BaseStream.Position = 0
        $responseBody = $reader.ReadToEnd()
        Write-Host "Error details: $responseBody" -ForegroundColor Red
    }
    exit 1
}

# Step 2: Build SQL query for ClickHouse
Write-Host "Step 2: Building SQL query for ClickHouse..." -ForegroundColor Cyan

# Convert ISO 8601 to ClickHouse DateTime format
# Parse as UTC and keep UTC (ClickHouse stores times in UTC)
$fromDateTime = [DateTime]::Parse($From, $null, [System.Globalization.DateTimeStyles]::RoundtripKind)
$toDateTime = [DateTime]::Parse($To, $null, [System.Globalization.DateTimeStyles]::RoundtripKind)

# Ensure UTC
if ($fromDateTime.Kind -ne [System.DateTimeKind]::Utc) {
    $fromDateTime = $fromDateTime.ToUniversalTime()
}
if ($toDateTime.Kind -ne [System.DateTimeKind]::Utc) {
    $toDateTime = $toDateTime.ToUniversalTime()
}

$fromClickHouse = $fromDateTime.ToString("yyyy-MM-dd HH:mm:ss")
$toClickHouse = $toDateTime.ToString("yyyy-MM-dd HH:mm:ss")

# Build SELECT clause based on mode
if ($Mode -eq "minimal") {
    $selectClause = "SELECT event_time, level, event_presentation, user_name, comment, metadata_presentation"
} else {
    $selectClause = "SELECT event_time, event_date, cluster_guid, cluster_name, infobase_guid, infobase_name, level, event, event_presentation, user_name, user_id, computer, application, application_presentation, session_id, connection_id, transaction_status, transaction_id, data_separation, metadata_name, metadata_presentation, comment, data, data_presentation, server, primary_port, secondary_port"
}

# Build WHERE clause
$whereClause = "WHERE cluster_guid = '$ClusterGUID' AND infobase_guid = '$InfobaseGUID' AND event_time BETWEEN '$fromClickHouse' AND '$toClickHouse'"

if (-not [string]::IsNullOrEmpty($Level)) {
    $whereClause += " AND level = '$Level'"
}

$orderClause = "ORDER BY event_time DESC LIMIT $Limit"

$sqlQuery = "$selectClause FROM logs.event_log $whereClause $orderClause"

Write-Host "SQL Query:" -ForegroundColor Gray
Write-Host $sqlQuery -ForegroundColor DarkGray
Write-Host ""

# Step 3: Execute SQL query directly to ClickHouse
Write-Host "Step 3: Executing SQL query to ClickHouse..." -ForegroundColor Cyan

try {
    $clickhouseUrl = "http://${ClickHouseHost}:${ClickHousePort}/?database=${ClickHouseDB}"
    $encodedQuery = [System.Web.HttpUtility]::UrlEncode($sqlQuery)
    $amp = '&'
    $fullUrl = "$clickhouseUrl$amp" + "default_format=JSON$amp" + "query=$encodedQuery"
    
    $chResponseRaw = Invoke-WebRequest -Uri $fullUrl -Method Get -ErrorAction Stop
    $chResponseText = $chResponseRaw.Content
    $chResponse = $chResponseText | ConvertFrom-Json
    
    Write-Host 'ClickHouse query successful' -ForegroundColor Green
    
    # ClickHouse with default_format=JSON returns array of objects directly
    # Each object has property names matching column names
    if ($chResponse -is [Array]) {
        $chResults = $chResponse
    } elseif ($chResponse.data) {
        $chResults = $chResponse.data
    } else {
        $chResults = @($chResponse)
    }
    
    Write-Host "  ClickHouse returned $($chResults.Count) records" -ForegroundColor Gray
    Write-Host ""
    
} catch {
    Write-Host 'ClickHouse query failed:' $_.Exception.Message -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $reader.BaseStream.Position = 0
        $responseBody = $reader.ReadToEnd()
        Write-Host "Error details: $responseBody" -ForegroundColor Red
    }
    exit 1
}

# Step 4: Compare results
Write-Host "Step 4: Comparing results..." -ForegroundColor Cyan
Write-Host ""

# Normalize results for comparison
# MCP returns array of objects with camelCase properties
# ClickHouse returns array of arrays with column order matching SELECT

$mcpCount = $mcpResults.Count
$chCount = $chResults.Count

Write-Host "Record counts:" -ForegroundColor Yellow
Write-Host "  MCP: $mcpCount"
Write-Host "  ClickHouse: $chCount"

if ($mcpCount -ne $chCount) {
    Write-Host "WARNING: Record counts differ!" -ForegroundColor Yellow
} else {
    Write-Host 'Record counts match' -ForegroundColor Green
}

Write-Host ""

# Compare individual records
$maxCompare = [Math]::Min($mcpCount, $chCount)
$differences = @()

for ($i = 0; $i -lt $maxCompare; $i++) {
    $mcpRecord = $mcpResults[$i]
    $chRecord = $chResults[$i]
    
    # ClickHouse returns objects with property names, so use directly
    $chObj = $chRecord
    
    # Compare key fields
    $recordDiff = @()
    
    foreach ($key in $mcpRecord.PSObject.Properties.Name) {
        $mcpValue = $mcpRecord.$key
        $chValue = $chObj.$key
        
        # Normalize values for comparison (handle nulls, empty strings, date formats)
        if ($null -eq $mcpValue) { $mcpValue = "" }
        if ($null -eq $chValue) { $chValue = "" }
        
        # Special handling for event_time: normalize format
        if ($key -eq 'event_time') {
            # MCP returns ISO 8601 with Z (2025-11-16T23:05:12Z)
            # ClickHouse returns format without timezone (2025-11-16 23:05:12.000000)
            # Parse both and compare as DateTime
            try {
                $mcpTime = [DateTime]::Parse($mcpValue.ToString(), $null, [System.Globalization.DateTimeStyles]::RoundtripKind)
                $chTime = [DateTime]::Parse($chValue.ToString())
                # Compare up to seconds (ignore microseconds)
                $mcpTimeRounded = $mcpTime.AddTicks(-($mcpTime.Ticks % 10000000))
                $chTimeRounded = $chTime.AddTicks(-($chTime.Ticks % 10000000))
                if ($mcpTimeRounded -ne $chTimeRounded) {
                    $recordDiff += ('  ' + $key + ' : MCP=' + $mcpValue.ToString() + ' vs CH=' + $chValue.ToString())
                }
            } catch {
                # If parsing fails, compare as strings
                $mcpStr = $mcpValue.ToString()
                $chStr = $chValue.ToString()
                if ($mcpStr -ne $chStr) {
                    $recordDiff += ('  ' + $key + ' : MCP=' + $mcpStr + ' vs CH=' + $chStr)
                }
            }
        } else {
            # Convert to string for comparison
            $mcpStr = $mcpValue.ToString()
            $chStr = $chValue.ToString()
            
            if ($mcpStr -ne $chStr) {
                $recordDiff += ('  ' + $key + ' : MCP=' + $mcpStr + ' vs CH=' + $chStr)
            }
        }
    }
    
    if ($recordDiff.Count -gt 0) {
        $recNum = $i + 1
        $evtTime = $mcpRecord.event_time
        $differences += ('Record ' + $recNum + ' (event_time=' + $evtTime + '):')
        $differences += $recordDiff
    }
}

if ($differences.Count -eq 0) {
    Write-Host 'All records match perfectly!' -ForegroundColor Green
} else {
    $diffCount = $differences.Count
    Write-Host ('Found differences in ' + $diffCount + ' field(s):') -ForegroundColor Yellow
    Write-Host ""
    foreach ($diff in $differences) {
        Write-Host $diff -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host '=== Test Summary ===' -ForegroundColor Cyan
Write-Host ('  MCP records: ' + $mcpCount)
Write-Host ('  ClickHouse records: ' + $chCount)
Write-Host ('  Compared: ' + $maxCompare)
Write-Host ('  Differences: ' + $differences.Count)
Write-Host ""

if ($mcpCount -eq $chCount -and $differences.Count -eq 0) {
    Write-Host 'TEST PASSED: Results match perfectly!' -ForegroundColor Green
    exit 0
} else {
    Write-Host 'TEST FAILED: Results do not match' -ForegroundColor Red
    exit 1
}

