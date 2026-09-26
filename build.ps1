# 一键构建脚本（Windows）
# 用法: .\build.ps1
# 输出: backend/netops-server.exe
# 交叉编译 Linux: $env:GOOS='linux'; .\build.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$env:GOPROXY = 'https://goproxy.cn,direct'
$env:GOROOT = 'D:\go'
$env:PATH = "D:\go\bin;$env:PATH"

Write-Host "=== 1/3 构建前端 ===" -ForegroundColor Cyan
Set-Location web
npm install
npm run build
if ($LASTEXITCODE -ne 0) { exit 1 }

Write-Host "`n=== 2/3 复制前端产物到后端 ===" -ForegroundColor Cyan
Remove-Item "$root\backend\web\dist" -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item -Path "$root\web\dist" -Destination "$root\backend\web\dist" -Recurse -Force

Write-Host "`n=== 3/3 编译后端 ===" -ForegroundColor Cyan
Set-Location "$root\backend"

if ($env:GOOS -eq 'linux') {
    $env:GOARCH = 'amd64'
    go build -o netops-server .
} else {
    go build -o netops-server.exe .
}

Write-Host "`n=== 构建完成 ===" -ForegroundColor Green
if ($env:GOOS -eq 'linux') {
    Write-Host "Linux: $root\backend\netops-server"
} else {
    Write-Host "Windows: $root\backend\netops-server.exe"
}
