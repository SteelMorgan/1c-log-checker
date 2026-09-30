# Full test for configure_techlog with file creation check
# Tests validation, file creation and deletion

param(
    [string]$MCPHost = "localhost",
    [int]$MCPPort = 8080,
    [string]$TestConfigDir = "$env:TEMP\test_techlog_config",  # Directory on host (will be mounted to container)
    [string]$TestConfigPath = "$env:TEMP\test_techlog_config\test_logcfg.xml"  # Full path on host
)

$baseUrl = "http://${MCPHost}:${MCPPort}"

# Test GUIDs
$testClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
$testInfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be"

Write-Host "=== Full configure_techlog test ===" -ForegroundColor Cyan
Write-Host "URL: $baseUrl" -ForegroundColor Gray
Write-Host "Test config dir: $TestConfigDir" -ForegroundColor Gray
Write-Host "Test config path: $TestConfigPath" -ForegroundColor Gray
Write-Host ""
Write-Host "NOTE: Make sure TECHLOG_CONFIG_DIR in docker-compose.yml points to: $TestConfigDir" -ForegroundColor Yellow
Write-Host ""

# Function to execute HTTP request
function Invoke-MCPRequest {
    param(
        [string]$Endpoint,
        [string]$Method = "POST",
        [object]$Body = $null
    )
    
    $url = "$baseUrl$Endpoint"
    
    try {
        $headers = @{
            "Content-Type" = "application/json"
        }
        
        $params = @{
            Uri = $url
            Method = $Method
            Headers = $headers
            ErrorAction = "Stop"
        }
        
        if ($Body) {
            $jsonBody = $Body | ConvertTo-Json -Depth 10
            $params.Body = $jsonBody
        }
        
        $response = Invoke-RestMethod @params
        return @{
            Success = $true
            Data = $response
            Error = $null
        }
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        $errorDetails = $null
        if ($_.ErrorDetails.Message) {
            try {
                $errorDetails = $_.ErrorDetails.Message | ConvertFrom-Json
            } catch {
                $errorDetails = @{ message = $_.ErrorDetails.Message }
            }
        }
        
        return @{
            Success = $false
            StatusCode = $statusCode
            Error = $errorDetails
            Exception = $_.Exception.Message
        }
    }
}

# Test 1: Invalid path (should return validation error)
Write-Host "=== Test 1: Invalid path (should return error) ===" -ForegroundColor Yellow
$invalidLocation = "D:\TechLogs"
$invalidBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $invalidLocation
    history = 24
    format = "json"
    events = @("EXCP")
    properties = @("all")
}

$result1 = Invoke-MCPRequest -Endpoint "/tools/configure_techlog" -Body $invalidBody

if (-not $result1.Success -and $result1.StatusCode -eq 400) {
    Write-Host "OK: Validation works! Error returned:" -ForegroundColor Green
    if ($result1.Error) {
        Write-Host "  Error: $($result1.Error.error)" -ForegroundColor Yellow
        Write-Host "  Message: $($result1.Error.message)" -ForegroundColor Yellow
    } else {
        Write-Host "  $($result1.Exception)" -ForegroundColor Yellow
    }
} else {
    Write-Host "FAIL: Validation did not work!" -ForegroundColor Red
    Write-Host "  Response: $($result1 | ConvertTo-Json)" -ForegroundColor Red
}

Write-Host ""

# Test 2: Valid path with file saving
Write-Host "=== Test 2: Valid path with file saving ===" -ForegroundColor Yellow
$validLocation = "D:\TechLogs\$testClusterGUID\$testInfobaseGUID"
# Use path inside container (mounted from host)
$containerConfigPath = "/app/techlog-config/test_logcfg.xml"
$validBody = @{
    cluster_guid = $testClusterGUID
    infobase_guid = $testInfobaseGUID
    location = $validLocation
    config_path = $containerConfigPath
    history = 24
    format = "json"
    events = @("EXCP", "QERR")
    properties = @("all")
}

$result2 = Invoke-MCPRequest -Endpoint "/tools/configure_techlog" -Body $validBody

if ($result2.Success) {
    Write-Host "OK: Configuration created successfully" -ForegroundColor Green
    Write-Host "  XML length: $($result2.Data.Length) characters" -ForegroundColor Gray
    
    # Check file (on host, mounted from container)
    Write-Host ""
    Write-Host "=== File check (on host) ===" -ForegroundColor Yellow
    if (Test-Path $TestConfigPath) {
        Write-Host "OK: File created: $TestConfigPath" -ForegroundColor Green
        
        $fileContent = Get-Content $TestConfigPath -Raw
        $fileInfo = Get-Item $TestConfigPath
        
        Write-Host "  Size: $($fileInfo.Length) bytes" -ForegroundColor Gray
        Write-Host "  Created: $($fileInfo.CreationTime)" -ForegroundColor Gray
        
        # Check content
        Write-Host ""
        Write-Host "=== File content ===" -ForegroundColor Yellow
        Write-Host $fileContent -ForegroundColor White
        
        # Check GUIDs
        if ($fileContent -match [regex]::Escape($testClusterGUID) -and 
            $fileContent -match [regex]::Escape($testInfobaseGUID)) {
            Write-Host ""
            Write-Host "OK: File contains correct GUIDs" -ForegroundColor Green
        } else {
            Write-Host ""
            Write-Host "FAIL: File does not contain correct GUIDs" -ForegroundColor Red
        }
        
        # Check location (normalized - backslashes converted to forward slashes)
        $normalizedLocation = $validLocation.Replace('\', '/')
        if ($fileContent -match [regex]::Escape($normalizedLocation)) {
            Write-Host "OK: File contains correct location (normalized)" -ForegroundColor Green
        } else {
            Write-Host "FAIL: File does not contain correct location" -ForegroundColor Red
            Write-Host "  Expected: $normalizedLocation" -ForegroundColor Yellow
        }
        
        # Check format
        if ($fileContent -match 'format="json"') {
            Write-Host "OK: File contains format=`"json`"" -ForegroundColor Green
        } else {
            Write-Host "FAIL: File does not contain format=`"json`"" -ForegroundColor Red
        }
        
        # Check events
        if ($fileContent -match 'value="EXCP"' -and $fileContent -match 'value="QERR"') {
            Write-Host "OK: File contains events EXCP and QERR" -ForegroundColor Green
        } else {
            Write-Host "FAIL: File does not contain required events" -ForegroundColor Red
        }
        
    } else {
        Write-Host "FAIL: File was NOT created!" -ForegroundColor Red
    }
} else {
    Write-Host "FAIL: Error creating configuration:" -ForegroundColor Red
    Write-Host "  $($result2 | ConvertTo-Json)" -ForegroundColor Red
}

Write-Host ""

# Test 3: Disable tech log
Write-Host "=== Test 3: Disable tech log ===" -ForegroundColor Yellow
$disableBody = @{
    config_path = $containerConfigPath
}

$result3 = Invoke-MCPRequest -Endpoint "/tools/disable_techlog" -Body $disableBody

if ($result3.Success) {
    Write-Host "OK: disable_techlog command executed successfully" -ForegroundColor Green
    
    # Check that file was modified (should contain empty config)
    if (Test-Path $TestConfigPath) {
        $disabledContent = Get-Content $TestConfigPath -Raw
        Write-Host ""
        Write-Host "=== Content after disable ===" -ForegroundColor Yellow
        Write-Host $disabledContent -ForegroundColor White
        
        # Check that there are no log elements
        if ($disabledContent -notmatch '<log') {
            Write-Host ""
            Write-Host "OK: File disabled (no log elements)" -ForegroundColor Green
        } else {
            Write-Host ""
            Write-Host "FAIL: File not disabled (has log elements)" -ForegroundColor Red
        }
        
        # Delete test file (on host)
        Write-Host ""
        Write-Host "=== Deleting test file ===" -ForegroundColor Yellow
        Remove-Item $TestConfigPath -Force
        if (-not (Test-Path $TestConfigPath)) {
            Write-Host "OK: Test file deleted" -ForegroundColor Green
        } else {
            Write-Host "FAIL: Could not delete test file" -ForegroundColor Red
        }
    } else {
        Write-Host "OK: File does not exist (already deleted or not created)" -ForegroundColor Green
    }
} else {
    Write-Host "FAIL: Error disabling tech log:" -ForegroundColor Red
    Write-Host "  $($result3 | ConvertTo-Json)" -ForegroundColor Red
}

Write-Host ""
Write-Host "=== Testing completed ===" -ForegroundColor Cyan
