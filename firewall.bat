@echo off
setlocal
chcp 65001 >nul
title 内网通 防火墙放行

cd /d "%~dp0"

rem 需要管理员权限
net session >nul 2>&1
if errorlevel 1 (
    echo 需要管理员权限。请右键本文件 -^> "以管理员身份运行"。
    pause
    exit /b 1
)

echo 正在为 2425 / 2426 / 2427 端口添加入站放行规则...
netsh advfirewall firewall delete rule name="FlyDog UDP 2425" >nul 2>&1
netsh advfirewall firewall delete rule name="FlyDog TCP 2425" >nul 2>&1
netsh advfirewall firewall delete rule name="FlyDog TCP 2426" >nul 2>&1
netsh advfirewall firewall delete rule name="NeiWangTong UDP 2425" >nul 2>&1
netsh advfirewall firewall delete rule name="NeiWangTong TCP 2425" >nul 2>&1
netsh advfirewall firewall delete rule name="NeiWangTong TCP 2426" >nul 2>&1
netsh advfirewall firewall delete rule name="NeiWangTong TCP 2427" >nul 2>&1

netsh advfirewall firewall add rule name="NeiWangTong UDP 2425" dir=in action=allow protocol=UDP localport=2425
if errorlevel 1 goto failed
netsh advfirewall firewall add rule name="NeiWangTong TCP 2425" dir=in action=allow protocol=TCP localport=2425
if errorlevel 1 goto failed
netsh advfirewall firewall add rule name="NeiWangTong TCP 2426" dir=in action=allow protocol=TCP localport=2426
if errorlevel 1 goto failed
netsh advfirewall firewall add rule name="NeiWangTong TCP 2427" dir=in action=allow protocol=TCP localport=2427
if errorlevel 1 goto failed

goto success

:failed
echo.
echo [失败] 添加规则时出错。
pause
exit /b 1

:success

echo.
echo [完成] 已放行 UDP/TCP 2425、TCP 2426、TCP 2427（入站）。
echo 直接运行 bin\neiwangtong.exe 后点"扫描"或"直连"即可。
echo.
pause
endlocal
