@echo off
REM GoWinProc Backend Stop Script

echo Stopping gowinproc-gui.exe...
echo.

REM Send graceful shutdown request
curl -X POST http://127.0.0.1:8080/shutdown 2>nul

IF %ERRORLEVEL% EQU 0 (
    echo.
    echo Server shutdown request sent successfully!
    echo Waiting for graceful shutdown...
    timeout /t 3 /nobreak >nul
) ELSE (
    echo.
    echo Warning: Could not connect to server on port 8080
    echo The server may already be stopped or running on a different port
)

echo.
echo Checking for running processes...
tasklist | findstr gowinproc-gui.exe >nul

IF %ERRORLEVEL% EQU 0 (
    echo gowinproc-gui.exe is still running
    echo Use Task Manager to force stop if needed
) ELSE (
    echo No gowinproc-gui.exe processes found
)

echo.
pause
