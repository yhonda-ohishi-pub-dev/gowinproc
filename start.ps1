# GoWinProc Backend Startup Script
# Usage: .\start.ps1 [config_file]
# Default: config.db_service.yaml

param(
    [string]$ConfigFile = "config.db_service.yaml"
)

Write-Host "Starting gowinproc-gui.exe with $ConfigFile..." -ForegroundColor Green
Write-Host ""

# Check if binary exists
if (-not (Test-Path "gowinproc-gui.exe")) {
    Write-Host "Error: gowinproc-gui.exe not found!" -ForegroundColor Red
    Write-Host "Please build the binary first: go build -o gowinproc-gui.exe ./src/cmd/gowinproc" -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

# Check if config file exists
if (-not (Test-Path $ConfigFile)) {
    Write-Host "Error: Configuration file $ConfigFile not found!" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Start the server
Start-Process -FilePath "gowinproc-gui.exe" -ArgumentList "-config", $ConfigFile

Write-Host ""
Write-Host "Server started successfully!" -ForegroundColor Green
Write-Host "Config: $ConfigFile" -ForegroundColor Cyan
Write-Host ""
Write-Host "To stop the server, use: .\stop.ps1" -ForegroundColor Yellow
