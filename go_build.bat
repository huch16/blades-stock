@echo off
set "GOPATH=G:\stock\Go"
set "PATH=G:\stock\Go\bin;C:\Users\huch\go\bin;%PATH%"
cd /d G:\stock\blades-stock
go build -ldflags "-s -w -H=windowsgui" -o build/bin/blades-stock.exe . 2>&1
