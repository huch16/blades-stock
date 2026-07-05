@echo off
set "GOPATH=G:\stock\Go"
set "PATH=G:\stock\Go\bin;C:\Users\huch\go\bin;%PATH%"
cd /d G:\stock\blades-stock
wails build 2>&1
