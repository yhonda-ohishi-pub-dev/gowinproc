# GoWinProc Backend Status Check Script

Write-Host "=== GoWinProc Server Status ===" -ForegroundColor Cyan
Write-Host ""

# Check if process is running
$processes = Get-Process -Name "gowinproc-gui" -ErrorAction SilentlyContinue

if ($processes) {
    Write-Host "[Process Status]" -ForegroundColor Green
    $processes | ForEach-Object {
        Write-Host "  PID: $($_.Id)" -ForegroundColor White
        Write-Host "  Memory: $([math]::Round($_.WorkingSet64 / 1MB, 2)) MB" -ForegroundColor White
        Write-Host "  Start Time: $($_.StartTime)" -ForegroundColor White
    }
    Write-Host ""
} else {
    Write-Host "[Process Status] Not Running" -ForegroundColor Red
    Write-Host ""
    exit
}

# Check HTTP endpoint
Write-Host "[HTTP Server]" -ForegroundColor Green
try {
    $healthResponse = Invoke-WebRequest -Uri "http://127.0.0.1:8080/health" -Method GET -TimeoutSec 5 -ErrorAction Stop
    Write-Host "  Port 8080: " -NoNewline -ForegroundColor White
    Write-Host "LISTENING" -ForegroundColor Green
    Write-Host "  Health: $($healthResponse.Content)" -ForegroundColor White
} catch {
    Write-Host "  Port 8080: " -NoNewline -ForegroundColor White
    Write-Host "NOT RESPONDING" -ForegroundColor Red
}
Write-Host ""

# Check registry endpoint
Write-Host "[Registry API]" -ForegroundColor Green
try {
    $registryResponse = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/registry" -Method GET -TimeoutSec 5 -ErrorAction Stop
    Write-Host "  Endpoint: /api/registry" -ForegroundColor White
    Write-Host "  Processes: $($registryResponse.available_processes.Count)" -ForegroundColor White

    Write-Host ""
    Write-Host "  Running Processes:" -ForegroundColor Cyan
    $registryResponse.available_processes | Where-Object { $_.status -eq "running" } | ForEach-Object {
        $servicesCount = if ($_.services) { $_.services.Count } else { 0 }
        Write-Host "    - $($_.name) (instances: $($_.instances), services: $servicesCount)" -ForegroundColor White
    }
} catch {
    Write-Host "  Endpoint: /api/registry - " -NoNewline -ForegroundColor White
    Write-Host "ERROR" -ForegroundColor Red
    Write-Host "  $($_.Exception.Message)" -ForegroundColor Gray
}

Write-Host ""
Write-Host "================================" -ForegroundColor Cyan
