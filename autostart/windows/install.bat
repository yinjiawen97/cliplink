@echo off
setlocal

set "PROJECT_DIR=%~dp0..\.."
for %%F in ("%PROJECT_DIR%") do set "PROJECT_DIR=%%~fF"
set "BINARY=%PROJECT_DIR%\cliplink.exe"
set "REG_KEY=HKCU\Software\Microsoft\Windows\CurrentVersion\Run"

if not exist "%BINARY%" (
    echo Error: cliplink.exe not found at %BINARY%
    echo Run "go build -o cliplink.exe ." in the project directory first.
    pause
    exit /b 1
)

reg add "%REG_KEY%" /v "cliplink" /t REG_SZ /d "\"%BINARY%\"" /f
echo cliplink autostart enabled. It will run at next login.
pause
