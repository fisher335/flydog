@echo off
setlocal
chcp 65001 >nul
title FlyDog 打包 (Wails v3)

cd /d "%~dp0"

rem ---- 查找 wails3 命令 ----
set "WAILS=wails3"
where wails3 >nul 2>nul
if errorlevel 1 (
    if exist "%USERPROFILE%\go\bin\wails3.exe" (
        set "WAILS=%USERPROFILE%\go\bin\wails3.exe"
    ) else (
        echo [错误] 找不到 wails3 命令。
        echo 请先安装:  go install github.com/wailsapp/wails/v3/cmd/wails3@latest
        echo 并确保 %%USERPROFILE%%\go\bin 已加入 PATH。
        pause
        exit /b 1
    )
)

where go >nul 2>nul
if errorlevel 1 (
    echo [错误] 找不到 go 命令，请先安装 Go 并加入 PATH。
    pause
    exit /b 1
)

set "MODE=%~1"
if "%MODE%"=="" set "MODE=release"

echo ============================================
echo    FlyDog  [%MODE%]   Wails v3
echo    %WAILS%
echo ============================================
echo.

if /i "%MODE%"=="release" goto release
if /i "%MODE%"=="debug"   goto debug
if /i "%MODE%"=="package" goto pkg
if /i "%MODE%"=="dev"     goto dev
if /i "%MODE%"=="clean"   goto clean
goto help

:release
echo 正式构建...
"%WAILS%" build
goto done

:debug
echo 调试构建...
"%WAILS%" build DEV=true
goto done

:pkg
echo 构建并打包安装程序（需要 NSIS）...
"%WAILS%" package
goto done

:dev
echo 启动开发模式（热重载，Ctrl+C 退出）...
"%WAILS%" dev -config ./build/config.yml -port 9245
goto end

:clean
echo 清理构建产物...
if exist "bin" rd /s /q "bin"
if exist "frontend\dist" rd /s /q "frontend\dist"
if exist "frontend\bindings" rd /s /q "frontend\bindings"
echo 已清理 bin / frontend\dist / frontend\bindings。
goto end

:help
echo 用法:  build.bat [release ^| debug ^| package ^| dev ^| clean]
echo.
echo   release   正式单文件 exe (bin\flydog.exe)
echo   debug     调试构建
echo   package   构建并打包安装程序
echo   dev       开发模式（前端热重载）
echo   clean     清理构建产物
goto end

:done
if errorlevel 1 (
    echo.
    echo [失败] 构建出错，请查看上面的日志。
    pause
    exit /b 1
)
echo.
echo [完成] 产物 (bin):
for %%F in ("bin\*.exe") do echo     %%~nxF   %%~zF 字节
echo.
echo 提示: 目标机若为旧版 Win10 需安装 WebView2 运行时；首次运行请允许防火墙访问 UDP/TCP 2425、TCP 2426。
echo.
pause
goto :eof

:end
endlocal
