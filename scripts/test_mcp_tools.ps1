# Test MCP tools
# Quick script to test individual MCP server tools

param(
    [string]$MCPUrl = "http://localhost:8080",
    [string]$ClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
    [string]$InfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"
)

function Test-Tool {
    param(
        [string]$ToolName,
        [hashtable]$Body
    )

    Write-Host ""
    Write-Host "=== Testing $ToolName ===" -ForegroundColor Cyan

    try {
        $jsonBody = $Body | ConvertTo-Json -Depth 10
        Write-Host "Request body:" -ForegroundColor Gray
        Write-Host $jsonBody -ForegroundColor DarkGray

        $response = Invoke-RestMethod -Uri "$MCPUrl/tools/$ToolName" -Method Post -Body $jsonBody -ContentType "application/json" -ErrorAction Stop

        Write-Host "✓ Success" -ForegroundColor Green
        Write-Host "Response:" -ForegroundColor Gray
        Write-Host ($response | ConvertTo-Json -Depth 10) -ForegroundColor DarkGray

        return $true
    } catch {
        Write-Host "✗ Failed: $($_.Exception.Message)" -ForegroundColor Red

        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $reader.BaseStream.Position = 0
            $responseBody = $reader.ReadToEnd()
            Write-Host "Error details: $responseBody" -ForegroundColor Red
        }

        return $false
    }
}

# Test 1: configure_techlog
$configBody = @{
    cluster_guid = $ClusterGUID
    infobase_guid = $InfobaseGUID
    location = "D:\My Projects\FrameWork 1C\1c-log-checker\tech_logs\$ClusterGUID\$InfobaseGUID"
    format = "json"
    history = 24
    events = @("EXCP", "EXCPCNTX")
    properties = @("all")
}
Test-Tool -ToolName "configure_techlog" -Body $configBody

# Test 2: get_techlog_config
$getConfigBody = @{}
Test-Tool -ToolName "get_techlog_config" -Body $getConfigBody

# Test 3: get_tech_log (query recent tech log events)
$getTechLogBody = @{
    cluster_guid = $ClusterGUID
    infobase_guid = $InfobaseGUID
    from = (Get-Date).AddHours(-1).ToString("yyyy-MM-ddTHH:mm:ss")
    to = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss")
    event_types = @("EXCP")
    limit = 10
}
Test-Tool -ToolName "get_tech_log" -Body $getTechLogBody

# Test 4: get_event_log (query recent event log)
$getEventLogBody = @{
    cluster_guid = $ClusterGUID
    infobase_guid = $InfobaseGUID
    from = (Get-Date).AddDays(-1).ToString("yyyy-MM-ddTHH:mm:ss")
    to = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss")
    event_types = @("_$" + "PerformError" + "$" + "_")
    limit = 10
}
Test-Tool -ToolName "get_event_log" -Body $getEventLogBody

# Test 5: disable_techlog
Write-Host ""
Write-Host "Skipping disable_techlog test (would disable tech log)" -ForegroundColor Yellow
# Uncomment to test:
# $disableBody = @{}
# Test-Tool -ToolName "disable_techlog" -Body $disableBody

Write-Host ""
Write-Host "=== All Tests Completed ===" -ForegroundColor Cyan
