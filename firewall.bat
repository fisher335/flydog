@echo off
setlocal
chcp 65001 >nul
title FlyDog 防火墙放行

cd /d "%~dp0"

rem 需要管理员权限
net session >nul 2>&1
if errorlevel 1 (
    echo 需要管理员权限。请右键本文件 -^> "以管理员身份运行"。
    pause
    exit /b 1
)

echo 正在为飞鸽协议端口 2425 添加入站放行规则...
netsh advfirewall firewall delete rule name="FlyDog UDP 2425" >nul 2>&1
netsh advfirewall firewall delete rule name="FlyDog TCP 2425" >nul 2>&1
netsh advfirewall firewall delete rule name="FlyDog TCP 2426" >nul 2>&1

netsh advfirewall firewall add rule name="FlyDog UDP 2425" dir=in action=allow protocol=UDP localport=2425
netsh advfirewall firewall add rule name="FlyDog TCP 2425" dir=in action=allow protocol=TCP localport=2425
netsh advfirewall firewall add rule name="FlyDog TCP 2426" dir=in action=allow protocol=TCP localport=2426

if errorlevel 1 (
    echo.
    echo [失败] 添加规则时出错。
    pause
    exit /b 1
)

echo.
echo [完成] 已放行 UDP/TCP 2425、TCP 2426（入站）。
echo 直接运行 build\bin\flydog.exe 后点"扫描"或"直连"即可。
echo.
pause
endlocal
