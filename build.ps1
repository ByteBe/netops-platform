# 一键构建脚本（Windows）
# 用法:
#   .\build.ps1                    构建 Windows 版本
#   $env:GOOS='linux'; .\build.ps1  交叉编译 Linux 版本
# 输出: backend/netops-server[.exe] + 部署包 dist-release/

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$env:GOPROXY = 'https://goproxy.cn,direct'
$env:PATH = "D:\go\bin;$env:PATH"

Write-Host "=== 1/4 构建前端 ===" -ForegroundColor Cyan
Set-Location web
npm install
npm run build
if ($LASTEXITCODE -ne 0) { exit 1 }

Write-Host "`n=== 2/4 复制前端产物到后端 ===" -ForegroundColor Cyan
Remove-Item "$root\backend\web\dist" -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item -Path "$root\web\dist" -Destination "$root\backend\web\dist" -Recurse -Force

Write-Host "`n=== 3/4 编译后端 ===" -ForegroundColor Cyan
Set-Location "$root\backend"

if ($env:GOOS -eq 'linux') {
    $env:GOARCH = 'amd64'
    $exe = "netops-server"
    go build -o $exe .
} else {
    $exe = "netops-server.exe"
    go build -o $exe .
}

Write-Host "`n=== 4/4 生成部署包 ===" -ForegroundColor Cyan
$pkgDir = "$root\dist-release"
if (Test-Path $pkgDir) { Remove-Item $pkgDir -Recurse -Force }
New-Item -ItemType Directory -Path $pkgDir -Force | Out-Null
New-Item -ItemType Directory -Path "$pkgDir\web" -Force | Out-Null

Copy-Item "$root\backend\$exe" "$pkgDir\"
Copy-Item "$root\web\dist" "$pkgDir\web\dist" -Recurse
Copy-Item "$root\install.sh" "$pkgDir\"
Copy-Item "$root\install.bat" "$pkgDir\"
Copy-Item "$root\README.md" "$pkgDir\"

Write-Host "`n=== 构建完成 ===" -ForegroundColor Green
Write-Host "部署包: $pkgDir"
