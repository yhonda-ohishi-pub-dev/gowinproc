# GoWinProc Backend Stop Script

Write-Host "Stopping gowinproc-gui.exe..." -ForegroundColor Yellow
Write-Host ""

# Send graceful shutdown request
try {
    $response = Invoke-WebRequest -Uri "http://127.0.0.1:8080/shutdown" -Method POST -ErrorAction Stop
    Write-Host "Server shutdown request sent successfully!" -ForegroundColor Green
    Write-Host "Response: $($response.Content)" -ForegroundColor Cyan
    Write-Host "Waiting for graceful shutdown..." -ForegroundColor Yellow
    Start-Sleep -Seconds 3
} catch {
    Write-Host "Warning: Could not connect to server on port 8080" -ForegroundColor Yellow
    Write-Host "The server may already be stopped or running on a different port" -ForegroundColor Gray
}

Write-Host ""
Write-Host "Checking for running processes..." -ForegroundColor Cyan

# Check if process is still running
$processes = Get-Process -Name "gowinproc-gui" -ErrorAction SilentlyContinue

if ($processes) {
    Write-Host "gowinproc-gui.exe is still running (PID: $($processes.Id -join ', '))" -ForegroundColor Yellow
    Write-Host "Use Task Manager or 'Stop-Process -Id <PID>' to force stop if needed" -ForegroundColor Gray
} else {
    Write-Host "No gowinproc-gui.exe processes found - server stopped successfully!" -ForegroundColor Green
}

Write-Host ""
