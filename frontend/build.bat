@echo off
set "PATH=C:\Users\huch\AppData\Roaming\npm;C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0;C:\Program Files\nodejs"
cd /d G:\stock\blades-stock\frontend
if exist "src\wailsjs" rmdir /s /q "src\wailsjs"
npm run build
