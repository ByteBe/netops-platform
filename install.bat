@echo off
rem ============================================
rem NetOps 网络运维监控平台 - Windows 部署脚本
rem 用法:
rem   install.bat           安装并启动（开机自启 + 后台运行）
rem   install.bat start     仅启动
rem   install.bat stop      停止
rem   install.bat restart   重启
rem   install.bat status    查看状态
rem   install.bat uninstall 卸载服务
rem   install.bat logs      打开日志目录
rem ============================================
setlocal enabledelayedexpansion
chcp 65001 >nul

set "APP_NAME=netops-server.exe"
set "APP_DIR=%~dp0backend"
set "APP=%APP_DIR%\%APP_NAME%"
set "LOG_DIR=%APP_DIR%\logs"
set "TASK_NAME=NetOpsServer"

if not exist "%LOG_DIR%" mkdir "%LOG_DIR%"

goto :%1

:install
echo [NetOps] 安装服务...
if not exist "%APP%" (
  echo [错误] 未找到 %APP%
  echo 请先运行 build.ps1 构建
  exit /b 1
)
schtasks /create /tn "%TASK_NAME%" /tr "\"%APP%\"" /sc onstart /ru SYSTEM /rl highest /f >nul 2>&1
if %errorlevel%==0 (
  echo [NetOps] 服务已安装，开机自启已设置
  schtasks /run /tn "%TASK_NAME%" >nul 2>&1
  echo [NetOps] 已启动: http://127.0.0.1:30821
) else (
  echo [错误] 创建服务失败，请以管理员身份运行
)
goto :eof

:start
call :status
if %errorlevel%==0 (
  echo [NetOps] 已在运行
  goto :eof
)
start "NetOps" /min "%APP%"
echo [NetOps] 已启动: http://127.0.0.1:30821
goto :eof

:stop
tasklist /fi "imagename eq %APP_NAME%" 2>nul | find /i "%APP_NAME%" >nul
if %errorlevel%==0 (
  taskkill /f /im "%APP_NAME%" >nul 2>&1
  echo [NetOps] 已停止
) else (
  echo [NetOps] 未在运行
)
goto :eof

:restart
call :stop
timeout /t 2 /nobreak >nul
call :start
goto :eof

:status
tasklist /fi "imagename eq %APP_NAME%" 2>nul | find /i "%APP_NAME%" >nul
if %errorlevel%==0 (
  echo [NetOps] 运行中
  exit /b 0
) else (
  echo [NetOps] 未运行
  exit /b 1
)
goto :eof

:uninstall
echo [NetOps] 卸载服务...
schtasks /delete /tn "%TASK_NAME%" /f >nul 2>&1
echo [NetOps] 服务已删除
goto :eof

:logs
explorer "%LOG_DIR%"
goto :eof

:default
echo 用法: %~nx0 {install^|start^|stop^|restart^|status^|uninstall^|logs}
exit /b 1
