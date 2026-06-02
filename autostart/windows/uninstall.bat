@echo off
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "cliplink" /f
echo cliplink autostart removed.
pause
