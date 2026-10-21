# NetOps 多平台打包脚本（Windows 交叉编译）
# 产物：Windows-amd64 + Linux(麒麟/统信) amd64/arm64，输出到 dist-release
param(
    [string]$Version = "2.2.2"
)
$ErrorActionPreference = "Stop"
$b    = "E:\IDEA\netops-platform\backend"
$dist = "E:\IDEA\netops-platform\dist-release"
$readmeWin = "E:\IDEA\netops-platform\deploy\README-Windows.md"
$readmeLin = "E:\IDEA\netops-platform\deploy\README-Linux.md"

if (-not (Test-Path $dist)) { [void](New-Item -ItemType Directory -Force $dist) }

# 0) 删除旧部署包
Get-ChildItem $dist -Filter "netops-*.zip" -ErrorAction SilentlyContinue | Remove-Item -Force
Write-Output "old packs removed"

# 1) Windows amd64（本机部署）
$t = Join-Path $env:TEMP "netops_pack_win"
if (Test-Path $t) { Remove-Item $t -Recurse -Force }
[void](New-Item -ItemType Directory -Force $t)
Copy-Item (Join-Path $b "netops-server.exe") (Join-Path $t "netops-server.exe") -Force
Copy-Item (Join-Path $b "web") (Join-Path $t "web") -Recurse -Force
Copy-Item (Join-Path $b "start.bat") (Join-Path $t "start.bat") -Force
Copy-Item $readmeWin (Join-Path $t "README.md") -Force
Compress-Archive -Path (Join-Path $t "*") -DestinationPath (Join-Path $dist ("netops-windows-amd64-v" + $Version + ".zip")) -CompressionLevel Optimal
Write-Output ("packed: netops-windows-amd64-v" + $Version + ".zip  " + (Get-Item (Join-Path $dist ("netops-windows-amd64-v" + $Version + ".zip"))).Length)
Remove-Item $t -Recurse -Force

# 2) Linux amd64（云服务器 / 麒麟 / 统信）
$t = Join-Path $env:TEMP "netops_pack_linux_amd64"
if (Test-Path $t) { Remove-Item $t -Recurse -Force }
[void](New-Item -ItemType Directory -Force $t)
Copy-Item (Join-Path $b "netops-linux-amd64") (Join-Path $t "netops-server") -Force
Copy-Item (Join-Path $b "web") (Join-Path $t "web") -Recurse -Force
Copy-Item $readmeLin (Join-Path $t "README.md") -Force
Copy-Item "E:\IDEA\netops-platform\install.sh" (Join-Path $t "install.sh") -Force
Compress-Archive -Path (Join-Path $t "*") -DestinationPath (Join-Path $dist ("netops-kylin-amd64-v" + $Version + ".zip")) -CompressionLevel Optimal
Compress-Archive -Path (Join-Path $t "*") -DestinationPath (Join-Path $dist ("netops-uos-amd64-v" + $Version + ".zip")) -CompressionLevel Optimal
Write-Output ("packed: netops-kylin-amd64-v" + $Version + ".zip + netops-uos-amd64-v" + $Version + ".zip")
Remove-Item $t -Recurse -Force

# 3) Linux arm64（麒麟 / 统信）
$t = Join-Path $env:TEMP "netops_pack_linux_arm64"
if (Test-Path $t) { Remove-Item $t -Recurse -Force }
[void](New-Item -ItemType Directory -Force $t)
Copy-Item (Join-Path $b "netops-linux-arm64") (Join-Path $t "netops-server") -Force
Copy-Item (Join-Path $b "web") (Join-Path $t "web") -Recurse -Force
Copy-Item $readmeLin (Join-Path $t "README.md") -Force
Copy-Item "E:\IDEA\netops-platform\install.sh" (Join-Path $t "install.sh") -Force
Compress-Archive -Path (Join-Path $t "*") -DestinationPath (Join-Path $dist ("netops-kylin-arm64-v" + $Version + ".zip")) -CompressionLevel Optimal
Compress-Archive -Path (Join-Path $t "*") -DestinationPath (Join-Path $dist ("netops-uos-arm64-v" + $Version + ".zip")) -CompressionLevel Optimal
Write-Output ("packed: netops-kylin-arm64-v" + $Version + ".zip + netops-uos-arm64-v" + $Version + ".zip")
Remove-Item $t -Recurse -Force

Write-Output "DONE"
