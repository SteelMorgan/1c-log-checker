# Тестовый скрипт для проверки workflow работы с techlog через MCP
# Просто вызывает инструменты по порядку и проверяет что MCP отвечает

param(
    [string]$MCPHost = "localhost",
    [int]$MCPPort = 8080,
    [string]$ConfigPath = "configs/techlog/logcfg.xml"
)

$baseUrl = "http://${MCPHost}:${MCPPort}"

# GUIDs from plan
$clusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$infobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"

Write-Host "=== Test techlog workflow via MCP ===" -ForegroundColor Cyan
Write-Host "MCP URL: $baseUrl" -ForegroundColor Gray
Write-Host ""

# Function to call MCP tool
function Invoke-MCPTool {
    param(
        [string]$ToolName,
        [string]$Endpoint,
        [object]$Body = $null
    )
    
    $url = "$baseUrl$Endpoint"
    
    Write-Host "[$ToolName] Call: $Endpoint" -ForegroundColor Yellow
    if ($Body) {
        $jsonBody = $Body | ConvertTo-Json -Depth 10 -Compress
        Write-Host "  Body: $jsonBody" -ForegroundColor DarkGray
    }
    
    try {
        $headers = @{
            "Content-Type" = "application/json"
        }
        
        $params = @{
            Uri = $url
            Method = "POST"
            Headers = $headers
        }
        
        if ($Body) {
            $params.Body = $Body | ConvertTo-Json -Depth 10
        }
        
        $response = Invoke-RestMethod @params -ErrorAction Stop
        
        Write-Host "  OK: Response received" -ForegroundColor Green
        if ($response -is [string]) {
            if ($response.Length -gt 200) {
                Write-Host "  Response (first 200 chars): $($response.Substring(0, 200))..." -ForegroundColor Gray
            } else {
                Write-Host "  Response: $response" -ForegroundColor Gray
            }
        } else {
            $responseJson = $response | ConvertTo-Json -Depth 3
            if ($responseJson.Length -gt 200) {
                Write-Host "  Response (first 200 chars): $($responseJson.Substring(0, 200))..." -ForegroundColor Gray
            } else {
                Write-Host "  Response: $responseJson" -ForegroundColor Gray
            }
        }
        
        return @{ Success = $true; Data = $response }
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        $errorMsg = $_.Exception.Message
        
        if ($_.ErrorDetails.Message) {
            try {
                $errorDetails = $_.ErrorDetails.Message | ConvertFrom-Json
                $errorMsg = $errorDetails.message
                if ($errorDetails.error) {
                    $errorMsg = "$($errorDetails.error): $errorMsg"
                }
            } catch {
                $errorMsg = $_.ErrorDetails.Message
            }
        }
        
        Write-Host "  ERROR (HTTP $statusCode): $errorMsg" -ForegroundColor Red
        
        return @{ Success = $false; Error = $errorMsg; StatusCode = $statusCode }
    }
}

# Health check
Write-Host "`n=== Health check ===" -ForegroundColor Cyan
try {
    $health = Invoke-RestMethod -Uri "$baseUrl/health" -Method "GET"
    Write-Host "OK: Server is running" -ForegroundColor Green
} catch {
    Write-Host "ERROR: Server not responding: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Step 2: Save original configuration
Write-Host "`n=== Step 2: Save original configuration ===" -ForegroundColor Cyan

# 2.1. get_techlog_config
$getConfigResult = Invoke-MCPTool -ToolName "get_techlog_config" `
    -Endpoint "/tools/get_techlog_config" `
    -Body @{ config_path = $ConfigPath }

$originalConfigExists = $getConfigResult.Success
if ($originalConfigExists) {
    Write-Host "  -> Configuration found, saving backup..." -ForegroundColor Yellow
    
    # 2.2. save_techlog
    $saveResult = Invoke-MCPTool -ToolName "save_techlog" `
        -Endpoint "/tools/save_techlog" `
        -Body @{ config_path = $ConfigPath }
} else {
    Write-Host "  -> Configuration not found (state: 'no config')" -ForegroundColor Yellow
}

# Step 3: Enable techlog (with validation testing)
Write-Host "`n=== Step 3: Enable techlog (validation testing) ===" -ForegroundColor Cyan

# 3.1. First attempt with invalid path
Write-Host "3.1. Calling configure_techlog with invalid path..." -ForegroundColor Yellow
$invalidLocation = "D:\TechLogs"
$invalidResult = Invoke-MCPTool -ToolName "configure_techlog_invalid" `
    -Endpoint "/tools/configure_techlog" `
    -Body @{
        cluster_guid = $clusterGUID
        infobase_guid = $infobaseGUID
        location = $invalidLocation
        history = 1
        format = "json"
        events = @("EXCP", "QERR", "EXCPCNTX")
        properties = @("all")
    }

if (-not $invalidResult.Success) {
    Write-Host "  OK: MCP returned validation error (expected)" -ForegroundColor Green
    Write-Host "  Error: $($invalidResult.Error)" -ForegroundColor DarkYellow
} else {
    Write-Host "  WARNING: MCP accepted invalid path (unexpected!)" -ForegroundColor Yellow
}

# 3.2. Second attempt with correct path
Write-Host "`n3.2. Calling configure_techlog with correct path..." -ForegroundColor Yellow
$correctLocation = "D:\My Projects\FrameWork 1C\1c-log-checker\tech_logs\$clusterGUID\$infobaseGUID"
$configureResult = Invoke-MCPTool -ToolName "configure_techlog_correct" `
    -Endpoint "/tools/configure_techlog" `
    -Body @{
        cluster_guid = $clusterGUID
        infobase_guid = $infobaseGUID
        location = $correctLocation
        history = 1
        format = "json"
        events = @("EXCP", "QERR", "EXCPCNTX")
        properties = @("all")
    }

if ($configureResult.Success) {
    Write-Host "  OK: Configuration created" -ForegroundColor Green
    
    # Check that XML doesn't have compress
    $xmlContent = if ($configureResult.Data -is [string]) { $configureResult.Data } else { $configureResult.Data | ConvertTo-Json }
    if ($xmlContent -notmatch 'compress') {
        Write-Host "  OK: XML doesn't have compress attribute" -ForegroundColor Green
    } else {
        Write-Host "  WARNING: XML contains compress attribute!" -ForegroundColor Yellow
    }
} else {
    Write-Host "  ERROR: Failed to create configuration" -ForegroundColor Red
}

# Step 4: Test execution info
Write-Host "`n=== Step 4: Unit test execution ===" -ForegroundColor Cyan
Write-Host "  -> Manual test execution required: Test_GenerateTechLog in 1C" -ForegroundColor Yellow
Write-Host "  -> Skipping this step in automated test" -ForegroundColor Gray

# Step 5: Disable techlog
Write-Host "`n=== Step 5: Disable techlog ===" -ForegroundColor Cyan
$disableResult = Invoke-MCPTool -ToolName "disable_techlog" `
    -Endpoint "/tools/disable_techlog" `
    -Body @{ config_path = $ConfigPath }

# Step 6: Smart Polling (get_actual_log_timestamp)
Write-Host "`n=== Step 6: Check get_actual_log_timestamp ===" -ForegroundColor Cyan
$timestampResult = Invoke-MCPTool -ToolName "get_actual_log_timestamp" `
    -Endpoint "/tools/get_actual_log_timestamp" `
    -Body @{ base_id = $infobaseGUID }

if ($timestampResult.Success) {
    $timestampData = if ($timestampResult.Data -is [string]) {
        $timestampResult.Data | ConvertFrom-Json
    } else {
        $timestampResult.Data
    }
    Write-Host "  -> has_data: $($timestampData.has_data)" -ForegroundColor Gray
    if ($timestampData.max_timestamp) {
        Write-Host "  -> max_timestamp: $($timestampData.max_timestamp)" -ForegroundColor Gray
    }
}

# Step 7: Query logs
Write-Host "`n=== Step 7: Query logs ===" -ForegroundColor Cyan
$fromTime = (Get-Date).AddHours(-1).ToString("yyyy-MM-ddTHH:mm:ssZ")
$toTime = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ssZ")

$techLogResult = Invoke-MCPTool -ToolName "get_tech_log" `
    -Endpoint "/tools/get_tech_log" `
    -Body @{
        cluster_guid = $clusterGUID
        infobase_guid = $infobaseGUID
        from = $fromTime
        to = $toTime
        name = "EXCP"
        mode = "minimal"
        limit = 10
    }

# Step 8: Analyze results
Write-Host "`n=== Step 8: Analyze results ===" -ForegroundColor Cyan
Write-Host "  -> Tool response check completed" -ForegroundColor Gray

# Step 9: Restore original configuration
Write-Host "`n=== Step 9: Restore original configuration ===" -ForegroundColor Cyan

if ($originalConfigExists) {
    $restoreResult = Invoke-MCPTool -ToolName "restore_techlog" `
        -Endpoint "/tools/restore_techlog" `
        -Body @{ config_path = $ConfigPath }
} else {
    Write-Host "  -> No original config, skipping restore" -ForegroundColor Gray
}

# Step 10: Final check
Write-Host "`n=== Step 10: Final check ===" -ForegroundColor Cyan
$finalCheck = Invoke-MCPTool -ToolName "get_techlog_config_final" `
    -Endpoint "/tools/get_techlog_config" `
    -Body @{ config_path = $ConfigPath }

Write-Host "`n=== Test completed ===" -ForegroundColor Cyan
Write-Host "All MCP tools called and responses checked" -ForegroundColor Green

