# Full reset script: stops parser, cleans offsets (local + Docker volume), cleans tables, rebuilds and restarts

$ErrorActionPreference = "Stop"

Write-Host "Stopping parser container first..." -ForegroundColor Yellow
cd deploy/docker
docker-compose stop log-parser
cd ../..

Write-Host "Cleaning offsets..." -ForegroundColor Cyan

# Clean local offsets file (if exists)
$offsetsFile = "offsets\parser.db"
if (Test-Path $offsetsFile) {
    Remove-Item $offsetsFile -Force -ErrorAction SilentlyContinue
    Write-Host "Deleted local offsets\parser.db" -ForegroundColor Green
} else {
    Write-Host "Local offsets\parser.db not found (already cleaned)" -ForegroundColor Gray
}

# Clean offsets in Docker volume (CRITICAL: this is where the actual data is stored)
Write-Host "Cleaning offsets in Docker volume..." -ForegroundColor Cyan
$parserContainer = "1c-log-parser"

# Try to delete file from container (only if container is running)
$containerRunning = docker ps --filter "name=$parserContainer" --format "{{.Names}}" | Select-String -Pattern $parserContainer
if ($containerRunning) {
    $result = docker exec $parserContainer rm -f /app/offsets/parser.db 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Deleted parser.db from Docker container" -ForegroundColor Green
    } else {
        Write-Host "Warning: Failed to delete from running container, will clean volume after containers are down" -ForegroundColor Yellow
    }
} else {
    Write-Host "Parser container is stopped, will clean volume after containers are down" -ForegroundColor Yellow
}

# Verify local offsets file is deleted
Write-Host "Verifying local offsets file is deleted..." -ForegroundColor Gray
if (Test-Path $offsetsFile) {
    Write-Host "ERROR: Local offsets\parser.db still exists!" -ForegroundColor Red
    exit 1
} else {
    Write-Host "Verified: Local offsets\parser.db is deleted" -ForegroundColor Green
}

Write-Host "Cleaning all ClickHouse tables (while ClickHouse is still running)..." -ForegroundColor Cyan
$containerName = "1c-log-clickhouse"
$containerRunning = docker ps --filter "name=$containerName" --format "{{.Names}}" | Select-String -Pattern $containerName

if ($containerRunning) {
    $cleanupSql = @"
TRUNCATE TABLE IF EXISTS logs.event_log;
TRUNCATE TABLE IF EXISTS logs.tech_log;
TRUNCATE TABLE IF EXISTS logs.parser_metrics;
TRUNCATE TABLE IF EXISTS logs.file_reading_progress;
"@
    
    Write-Host "Cleaning tables via docker exec..." -ForegroundColor Gray
    $cleanupSql | docker exec -i $containerName clickhouse-client --multiquery --database=logs 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "All tables cleaned successfully" -ForegroundColor Green
    } else {
        Write-Host "Warning: Failed to clean tables via docker exec" -ForegroundColor Yellow
    }
} else {
    Write-Host "ClickHouse container is not running, will clean tables after restart" -ForegroundColor Yellow
}

Write-Host "Stopping all containers..." -ForegroundColor Yellow
cd deploy/docker
docker-compose down
cd ../..

# Clean Docker volume for offsets (CRITICAL: this is where offsets are persisted)
Write-Host "Cleaning Docker volume for offsets..." -ForegroundColor Cyan
$volumeName = "docker_parser_offsets"
$volumeExists = docker volume ls --filter "name=$volumeName" --format "{{.Name}}" | Select-String -Pattern $volumeName

if ($volumeExists) {
    Write-Host "Removing Docker volume: $volumeName" -ForegroundColor Gray
    docker volume rm $volumeName 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Docker volume removed successfully" -ForegroundColor Green
    } else {
        Write-Host "Warning: Failed to remove volume (may be in use, will be recreated on next start)" -ForegroundColor Yellow
    }
} else {
    Write-Host "Docker volume not found (already cleaned or not created yet)" -ForegroundColor Gray
}

Write-Host "Rebuilding Docker images..." -ForegroundColor Cyan
cd deploy/docker
docker-compose build --no-cache log-parser
docker-compose build --no-cache mcp-server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error building images!" -ForegroundColor Red
    cd ../..
    exit 1
}
cd ../..

Write-Host "Starting containers..." -ForegroundColor Cyan
cd deploy/docker
docker-compose up -d
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error starting containers!" -ForegroundColor Red
    cd ../..
    exit 1
}
cd ../..

# Wait for ClickHouse to be ready and verify tables are clean
Write-Host "Waiting for ClickHouse to initialize..." -ForegroundColor Gray
Start-Sleep -Seconds 5

$containerName = "1c-log-clickhouse"
$maxRetries = 10
$retryCount = 0
$clickhouseReady = $false

while ($retryCount -lt $maxRetries -and -not $clickhouseReady) {
    $result = docker exec $containerName clickhouse-client --query "SELECT 1" 2>&1
    if ($LASTEXITCODE -eq 0) {
        $clickhouseReady = $true
    } else {
        $retryCount++
        Start-Sleep -Seconds 2
    }
}

if ($clickhouseReady) {
    Write-Host "Verifying tables are clean..." -ForegroundColor Gray
    $verifyQuery = "SELECT 'event_log' as table_name, count() as count FROM logs.event_log UNION ALL SELECT 'tech_log', count() FROM logs.tech_log UNION ALL SELECT 'parser_metrics', count() FROM logs.parser_metrics UNION ALL SELECT 'file_reading_progress', count() FROM logs.file_reading_progress FORMAT CSV"
    $result = docker exec $containerName clickhouse-client --query $verifyQuery 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Tables verification:" -ForegroundColor Green
        $allClean = $true
        $result | ForEach-Object {
            if ($_ -match '^"([^"]+)",(\d+)$') {
                $tableName = $matches[1]
                $count = $matches[2]
                if ($count -eq "0") {
                    Write-Host "  $tableName : $count records" -ForegroundColor Green
                } else {
                    Write-Host "  $tableName : $count records" -ForegroundColor Yellow
                    $allClean = $false
                }
            }
        }
        
        if (-not $allClean) {
            Write-Host "Warning: Some tables are not empty!" -ForegroundColor Yellow
        }
    }
    
    # Verify offsets in file_reading_progress table
    Write-Host "Verifying offsets in file_reading_progress table..." -ForegroundColor Gray
    $offsetsQuery = "SELECT count() as count FROM logs.file_reading_progress"
    $offsetsResult = docker exec $containerName clickhouse-client --query $offsetsQuery 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        $offsetsCount = [int]$offsetsResult.Trim()
        if ($offsetsCount -eq 0) {
            Write-Host "Verified: file_reading_progress table is empty (0 offsets)" -ForegroundColor Green
        } else {
            Write-Host "Warning: file_reading_progress table contains $offsetsCount records" -ForegroundColor Yellow
        }
    }
} else {
    Write-Host "Warning: Could not verify tables (ClickHouse may still be initializing)" -ForegroundColor Yellow
}

# Final verification: check offsets file and Docker volume after restart
Write-Host "Final verification: checking offsets..." -ForegroundColor Gray

# Check local file
if (Test-Path $offsetsFile) {
    Write-Host "Warning: Local offsets\parser.db exists after restart" -ForegroundColor Yellow
} else {
    Write-Host "Verified: Local offsets\parser.db does not exist" -ForegroundColor Green
}

# Check Docker volume
$parserContainer = "1c-log-parser"
Start-Sleep -Seconds 3  # Wait for parser to start
$parserRunning = docker ps --filter "name=$parserContainer" --format "{{.Names}}" | Select-String -Pattern $parserContainer

if ($parserRunning) {
    $dbExists = docker exec $parserContainer test -f /app/offsets/parser.db 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Info: parser.db exists in Docker volume (created by parser on first run - this is normal)" -ForegroundColor Cyan
        Write-Host "Note: This file will be empty initially, offsets will be created as files are processed" -ForegroundColor Gray
    } else {
        Write-Host "Verified: parser.db does not exist in Docker volume (will be created by parser on first run)" -ForegroundColor Green
    }
} else {
    Write-Host "Parser container is not running, cannot verify Docker volume" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Done! System restarted" -ForegroundColor Green
Write-Host ""
Write-Host "Services:" -ForegroundColor Cyan
Write-Host "  - ClickHouse: http://localhost:8123" -ForegroundColor White
Write-Host "  - Grafana: http://localhost:3000" -ForegroundColor White
Write-Host "  - MCP Server: http://localhost:8080" -ForegroundColor White
Write-Host ""
Write-Host "Check status:" -ForegroundColor Cyan
Write-Host "  docker ps" -ForegroundColor Gray

