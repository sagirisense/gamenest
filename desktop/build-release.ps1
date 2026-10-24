$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    npm --prefix frontend ci --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw '前端依赖安装失败' }
    npm --prefix frontend run build
    if ($LASTEXITCODE -ne 0) { throw '前端构建失败' }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go 测试失败' }
    wails build -s -trimpath
    if ($LASTEXITCODE -ne 0) { throw 'Wails 构建失败' }
    $releasePath = Join-Path (Split-Path $PSScriptRoot -Parent) '游栖'
    New-Item -ItemType Directory -Path $releasePath -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'build\bin\GameNest.exe') -Destination (Join-Path $releasePath '游栖.exe') -Force
    if (-not (Test-Path -LiteralPath (Join-Path $releasePath 'data'))) {
        New-Item -ItemType Directory -Path (Join-Path $releasePath 'data') -Force | Out-Null
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'data.example\games.json') -Destination (Join-Path $releasePath 'data\games.json')
    }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot '..\LICENSE') -Destination $releasePath -Force
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot '..\THIRD_PARTY_NOTICES.md') -Destination $releasePath -Force
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot '..\licenses') -Destination $releasePath -Recurse -Force
    Write-Output ('已构建：' + (Join-Path $releasePath '游栖.exe'))
} finally { Pop-Location }
