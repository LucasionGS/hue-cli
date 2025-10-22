@echo off
echo Building hue-cli...
go build -o bin/hue.exe
if %ERRORLEVEL% == 0 (
    echo Build successful! Run .\hue.exe --help to get started.
) else (
    echo Build failed!
    exit /b 1
)