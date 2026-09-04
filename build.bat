@echo off
gcc -std=c11 -O2 -o neurolang.exe rt\nl.c rt\main.c
if errorlevel 1 exit /b 1
echo built neurolang.exe
