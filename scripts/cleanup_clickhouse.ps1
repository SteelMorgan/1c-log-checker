# Cleanup script for ClickHouse data (PowerShell)
# Usage: .\scripts\cleanup_clickhouse.ps1 [all|event|tech]

param(
    [Parameter(Position=0)]
    [ValidateSet("all", "event", "tech")]
    [string]$Action = "all"
)

$CLICKHOUSE_HOST = if ($env:CLICKHOUSE_HOST) { $env:CLICKHOUSE_HOST } else { "localhost" }
$CLICKHOUSE_PORT = if ($env:CLICKHOUSE_PORT) { $env:CLICKHOUSE_PORT } else { "9000" }
$CLICKHOUSE_DB = if ($env:CLICKHOUSE_DB) { $env:CLICKHOUSE_DB } else { "logs" }
$CLICKHOUSE_USER = if ($env:CLICKHOUSE_USER) { $env:CLICKHOUSE_USER } else { "default" }
$CLICKHOUSE_PASSWORD = if ($env:CLICKHOUSE_PASSWORD) { $env:CLICKHOUSE_PASSWORD } else { "" }

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir

function Execute-Sql {
    param([string]$Query)
    
    $auth = ""
    if ($CLICKHOUSE_PASSWORD) {
        $auth = "--password=$CLICKHOUSE_PASSWORD"
    }
    
    $queryFile = New-TemporaryFile
    $Query | Out-File -FilePath $queryFile.FullName -Encoding UTF8
    
    try {
        $args = @(
            "--host=$CLICKHOUSE_HOST",
            "--port=$CLICKHOUSE_PORT",
            "--database=$CLICKHOUSE_DB",
            "--user=$CLICKHOUSE_USER"
        )
        
        if ($auth) {
            $args += $auth
        }
        
        $args += "--multiquery"
        $args += $queryFile.FullName
        
        & clickhouse-client $args
    }
    finally {
        Remove-Item $queryFile.FullName -Force
    }
}

switch ($Action) {
    "all" {
        Write-Host "⚠️  WARNING: This will delete ALL data from all tables!" -ForegroundColor Yellow
        $confirm = Read-Host "Are you sure? (yes/no)"
        if ($confirm -ne "yes") {
            Write-Host "Cancelled." -ForegroundColor Red
            exit 0
        }
        Write-Host "Truncating all tables..." -ForegroundColor Cyan
        $sql = Get-Content "$ProjectRoot\deploy\clickhouse\scripts\truncate_all.sql" -Raw
        Execute-Sql -Query $sql
        Write-Host "✅ All tables truncated." -ForegroundColor Green
    }
    "event" {
        Write-Host "⚠️  WARNING: This will delete ALL data from event_log table!" -ForegroundColor Yellow
        $confirm = Read-Host "Are you sure? (yes/no)"
        if ($confirm -ne "yes") {
            Write-Host "Cancelled." -ForegroundColor Red
            exit 0
        }
        Write-Host "Truncating event_log table..." -ForegroundColor Cyan
        $sql = Get-Content "$ProjectRoot\deploy\clickhouse\scripts\truncate_event_log.sql" -Raw
        Execute-Sql -Query $sql
        Write-Host "✅ event_log table truncated." -ForegroundColor Green
    }
    "tech" {
        Write-Host "⚠️  WARNING: This will delete ALL data from tech_log table!" -ForegroundColor Yellow
        $confirm = Read-Host "Are you sure? (yes/no)"
        if ($confirm -ne "yes") {
            Write-Host "Cancelled." -ForegroundColor Red
            exit 0
        }
        Write-Host "Truncating tech_log table..." -ForegroundColor Cyan
        $sql = Get-Content "$ProjectRoot\deploy\clickhouse\scripts\truncate_tech_log.sql" -Raw
        Execute-Sql -Query $sql
        Write-Host "✅ tech_log table truncated." -ForegroundColor Green
    }
}

