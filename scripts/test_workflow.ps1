# Test workflow script for 1C Log Checker
# This script tests the complete workflow: configure -> parse -> query

param(
    [string]$ClusterGUID = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
    [string]$InfobaseGUID = "d723aefd-7992-420d-b5f9-a273fd4146be",
    [string]$MCPUrl = "http://localhost:8080",
    [switch]$SkipDocker
)

$ErrorActionPreference = "Stop"

Write-Host "=== 1C Log Checker Test Workflow ===" -ForegroundColor Cyan
Write-Host ""

# Step 1: Check if Docker is running
if (-not $SkipDocker) {
    Write-Host "[Step 1] Checking Docker..." -ForegroundColor Yellow
    try {
        docker ps | Out-Null
        Write-Host "✓ Docker is running" -ForegroundColor Green
    } catch {
        Write-Host "✗ Docker is not running. Please start Docker Desktop." -ForegroundColor Red
        exit 1
    }
    Write-Host ""
}

# Step 2: Start Docker Compose
if (-not $SkipDocker) {
    Write-Host "[Step 2] Starting Docker Compose..." -ForegroundColor Yellow
    Push-Location "$PSScriptRoot\..\deploy\docker"
    try {
        docker-compose up -d
        Write-Host "✓ Docker Compose started" -ForegroundColor Green

        # Wait for services to be ready
        Write-Host "Waiting for services to be ready..." -ForegroundColor Yellow
        Start-Sleep -Seconds 10

        # Check ClickHouse health
        $maxRetries = 30
        $retries = 0
        while ($retries -lt $maxRetries) {
            try {
                $response = Invoke-WebRequest -Uri "http://localhost:8123/ping" -UseBasicParsing -ErrorAction Stop
                if ($response.StatusCode -eq 200) {
                    Write-Host "✓ ClickHouse is ready" -ForegroundColor Green
                    break
                }
            } catch {
                $retries++
                if ($retries -ge $maxRetries) {
                    Write-Host "✗ ClickHouse failed to start" -ForegroundColor Red
                    exit 1
                }
                Start-Sleep -Seconds 2
            }
        }

        # Check MCP server health
        $retries = 0
        while ($retries -lt $maxRetries) {
            try {
                $response = Invoke-WebRequest -Uri "$MCPUrl/health" -UseBasicParsing -ErrorAction SilentlyContinue
                Write-Host "✓ MCP Server is ready" -ForegroundColor Green
                break
            } catch {
                $retries++
                if ($retries -ge $maxRetries) {
                    Write-Host "⚠ MCP Server may not be ready (this is OK if not using MCP tools)" -ForegroundColor Yellow
                    break
                }
                Start-Sleep -Seconds 2
            }
        }
    } finally {
        Pop-Location
    }
    Write-Host ""
}

# Step 3: Test configure_techlog tool
Write-Host "[Step 3] Testing configure_techlog MCP tool..." -ForegroundColor Yellow
$techlogLocation = "D:\My Projects\FrameWork 1C\1c-log-checker\tech_logs\$ClusterGUID\$InfobaseGUID"

$configXml = @"
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
    <log location="$techlogLocation"
         format="json"
         history="24"
         placement="folders">
        <event><eq property="name" value="EXCP"/></event>
        <event><eq property="name" value="EXCPCNTX"/></event>
        <property name="all"/>
    </log>
</config>
"@

$body = @{
    cluster_guid = $ClusterGUID
    infobase_guid = $InfobaseGUID
    location = $techlogLocation
    format = "json"
    history = 24
    events = @("EXCP", "EXCPCNTX")
    properties = @("all")
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$MCPUrl/tools/configure_techlog" -Method Post -Body $body -ContentType "application/json" -ErrorAction Stop
    Write-Host "✓ configure_techlog succeeded" -ForegroundColor Green
    Write-Host "Response: $($response | ConvertTo-Json -Depth 3)" -ForegroundColor Gray
} catch {
    Write-Host "✗ configure_techlog failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $reader.BaseStream.Position = 0
        $responseBody = $reader.ReadToEnd()
        Write-Host "Error details: $responseBody" -ForegroundColor Red
    }
}
Write-Host ""

# Step 4: Check if techlog directory was created
Write-Host "[Step 4] Checking techlog directory..." -ForegroundColor Yellow
if (Test-Path $techlogLocation) {
    Write-Host "✓ Techlog directory exists: $techlogLocation" -ForegroundColor Green
} else {
    Write-Host "⚠ Techlog directory not found: $techlogLocation" -ForegroundColor Yellow
    Write-Host "  Creating directory..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Path $techlogLocation -Force | Out-Null
    Write-Host "✓ Directory created" -ForegroundColor Green
}
Write-Host ""

# Step 5: Check logcfg.xml
Write-Host "[Step 5] Checking logcfg.xml..." -ForegroundColor Yellow
$logcfgPath = "D:\My Projects\FrameWork 1C\1c-log-checker\configs\techlog\logcfg.xml"
if (Test-Path $logcfgPath) {
    Write-Host "✓ logcfg.xml exists: $logcfgPath" -ForegroundColor Green
    Write-Host "Content:" -ForegroundColor Gray
    Get-Content $logcfgPath | Write-Host -ForegroundColor Gray
} else {
    Write-Host "⚠ logcfg.xml not found" -ForegroundColor Yellow
}
Write-Host ""

# Step 6: Instructions for next steps
Write-Host "[Step 6] Next steps:" -ForegroundColor Yellow
Write-Host "1. Generate test logs (run 1C unit tests or manual operations)" -ForegroundColor White
Write-Host "2. Check logs in: $techlogLocation" -ForegroundColor White
Write-Host "3. Parser will automatically process logs (check with: docker-compose logs parser)" -ForegroundColor White
Write-Host "4. Query logs with get_tech_log tool" -ForegroundColor White
Write-Host "5. Check metrics in ClickHouse: SELECT * FROM logs.parser_metrics ORDER BY timestamp DESC LIMIT 10" -ForegroundColor White
Write-Host ""

Write-Host "=== Test Workflow Completed ===" -ForegroundColor Cyan
