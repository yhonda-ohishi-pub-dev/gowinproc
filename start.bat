@echo off
REM GoWinProc Backend Startup Script
REM Usage: start.bat [config_file]
REM Default: config.db_service.yaml

SET CONFIG_FILE=config.db_service.yaml

REM If argument provided, use it as config file
IF NOT "%1"=="" (
    SET CONFIG_FILE=%1
)

echo Starting gowinproc-gui.exe with %CONFIG_FILE%...
echo.

REM Check if binary exists
IF NOT EXIST gowinproc-gui.exe (
    echo Error: gowinproc-gui.exe not found!
    echo Please build the binary first: go build -o gowinproc-gui.exe ./src/cmd/gowinproc
    pause
    exit /b 1
)

REM Check if config file exists
IF NOT EXIST %CONFIG_FILE% (
    echo Error: Configuration file %CONFIG_FILE% not found!
    pause
    exit /b 1
)

REM Start the server
start gowinproc-gui.exe -config %CONFIG_FILE%

echo.
echo Server started successfully!
echo Config: %CONFIG_FILE%
echo.
echo Press any key to exit...
pause >nul
