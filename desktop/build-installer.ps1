[CmdletBinding()]
param(
    [switch]$SkipBuild,
    [string]$NsisPath,
    [string]$StageDirectory
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = Split-Path $PSScriptRoot -Parent
$config = Get-Content (Join-Path $PSScriptRoot 'wails.json') -Raw | ConvertFrom-Json
$version = $config.info.productVersion
if ($version -notmatch '^\d+\.\d+\.\d+$') { throw '版本号必须为 major.minor.patch' }
$tools = Join-Path $PSScriptRoot '.tools'
New-Item -ItemType Directory -Path $tools -Force | Out-Null

# Use a pinned portable compiler; do not install or change system tools.
$nsisHash = 'c7d27f780ddb6cffb4730138cd1591e841f4b7edb155856901cdf5f214394fa1'
if (-not $NsisPath) {
    $archive = Join-Path $tools 'nsis-3.11.zip'
    $verified = (Test-Path -LiteralPath $archive) -and ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -eq $nsisHash)
    if (-not $verified) {
        $download = $archive + '.download'
        $sources = @(
            'https://github.com/tauri-apps/binary-releases/releases/download/nsis-3.11/nsis-3.11.zip',
            'https://downloads.sourceforge.net/project/nsis/NSIS%203/3.11/nsis-3.11.zip',
            'https://master.dl.sourceforge.net/project/nsis/NSIS%203/3.11/nsis-3.11.zip',
            'https://netix.dl.sourceforge.net/project/nsis/NSIS%203/3.11/nsis-3.11.zip'
        )
        foreach ($source in $sources) {
            $hostName = ([uri]$source).Host
            try {
                Invoke-WebRequest -UseBasicParsing -Uri $source -OutFile $download -TimeoutSec 90
                $actualHash = (Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash.ToLowerInvariant()
                if ($actualHash -eq $nsisHash) {
                    Move-Item -LiteralPath $download -Destination $archive -Force
                    $verified = $true
                    break
                }
                Write-Warning "NSIS download rejected from $hostName (SHA-256: $actualHash)"
            } catch {
                Write-Warning "NSIS download failed from ${hostName}: $($_.Exception.Message)"
            } finally {
                if (Test-Path -LiteralPath $download) { Remove-Item -LiteralPath $download }
            }
        }
    }
    if (-not $verified) { throw '所有 NSIS 下载源均未通过校验，停止构建' }
    Expand-Archive -LiteralPath $archive -DestinationPath $tools -Force
    $NsisPath = Join-Path $tools 'nsis-3.11\makensis.exe'
}
if (-not (Test-Path -LiteralPath $NsisPath -PathType Leaf)) { throw '找不到 makensis.exe' }
$NsisPath = (Resolve-Path -LiteralPath $NsisPath).Path

$bootstrapper = Join-Path $tools 'MicrosoftEdgeWebview2Setup.exe'
if (-not (Test-Path -LiteralPath $bootstrapper)) {
    Invoke-WebRequest -UseBasicParsing -Uri 'https://go.microsoft.com/fwlink/p/?LinkId=2124703' -OutFile $bootstrapper
}
$signature = Get-AuthenticodeSignature -LiteralPath $bootstrapper
if ($signature.Status -ne 'Valid' -or $signature.SignerCertificate.Subject -notmatch 'O=Microsoft Corporation(?:,|$)') {
    throw 'WebView2 引导程序未通过微软签名校验'
}

Push-Location $PSScriptRoot
try {
    if (-not $SkipBuild) { & (Join-Path $PSScriptRoot 'build-release.ps1') }
    $binary = Join-Path $PSScriptRoot 'build\bin\GameNest.exe'
    if (-not (Test-Path -LiteralPath $binary)) { throw '请先构建应用，或移除 -SkipBuild' }
    $bytes = [IO.File]::ReadAllBytes($binary)
    $peOffset = [BitConverter]::ToInt32($bytes, 0x3c)
    if ([BitConverter]::ToUInt16($bytes, $peOffset + 4) -ne 0x8664) { throw '安装包只支持 Windows amd64 应用' }

    # Always stage a new allowlisted payload; never copy either local data directory.
    $stage = if ($StageDirectory) { [IO.Path]::GetFullPath($StageDirectory) } else { Join-Path $PSScriptRoot ('.installer-stage\' + [guid]::NewGuid().ToString('N')) }
    if (Test-Path -LiteralPath $stage) { throw '安装暂存目录必须为全新目录' }
    $payload = Join-Path $stage 'payload'
    New-Item -ItemType Directory -Path (Join-Path $payload 'data') -Force | Out-Null
    Copy-Item -LiteralPath $binary -Destination (Join-Path $payload '游栖.exe')
    foreach ($name in @('LICENSE', 'THIRD_PARTY_NOTICES.md', 'licenses')) {
        Copy-Item -LiteralPath (Join-Path $root $name) -Destination $payload -Recurse
    }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'data.example\games.json') -Destination (Join-Path $payload 'data\games.json')
    Copy-Item -LiteralPath (Join-Path $root 'docs\USAGE.md') -Destination (Join-Path $payload '使用指南.md')
    $nsisLicense = Join-Path (Split-Path $NsisPath -Parent) 'COPYING'
    New-Item -ItemType Directory -Path (Join-Path $payload 'licenses\nsis') -Force | Out-Null
    Copy-Item -LiteralPath $nsisLicense -Destination (Join-Path $payload 'licenses\nsis\COPYING') -Force
    $dataFiles = @(Get-ChildItem -LiteralPath (Join-Path $payload 'data') -File -Recurse)
    $emptyLibrary = Get-Content $dataFiles[0].FullName -Raw | ConvertFrom-Json
    if ($dataFiles.Count -ne 1 -or $dataFiles[0].Name -ne 'games.json' -or $emptyLibrary.games.Count -ne 0) {
        throw '安装包数据目录必须只包含空 games.json'
    }

    # Remove shipped files only during uninstall. Preserve data and unknown files.
    $uninstallLines = @()
    foreach ($file in Get-ChildItem -LiteralPath $payload -File -Recurse) {
        $relative = $file.FullName.Substring($payload.Length + 1)
        if ($relative.StartsWith('data\')) { continue }
        if ($relative -match '["$\r\n]') { throw '不支持的安装文件名' }
        $uninstallLines += 'Delete "$INSTDIR\' + $relative + '"'
    }
    foreach ($dir in (Get-ChildItem -LiteralPath (Join-Path $payload 'licenses') -Directory -Recurse | Sort-Object { $_.FullName.Length } -Descending)) {
        $uninstallLines += 'RMDir "$INSTDIR\' + $dir.FullName.Substring($payload.Length + 1) + '"'
    }
    $uninstallLines += 'RMDir "$INSTDIR\licenses"'
    $uninstallInclude = Join-Path $stage 'uninstall-files.nsh'
    [IO.File]::WriteAllLines($uninstallInclude, $uninstallLines, [Text.UTF8Encoding]::new($true))
    $installer = Join-Path $PSScriptRoot "build\bin\GameNest-v$version-windows-amd64-setup.exe"
    & $NsisPath '/INPUTCHARSET' 'UTF8' '/V3' "/DAPP_VERSION=$version" "/DPAYLOAD=$payload" "/DBOOTSTRAPPER=$bootstrapper" "/DUNINSTALL_FILES=$uninstallInclude" "/DOUTPUT_FILE=$installer" (Join-Path $PSScriptRoot 'build\windows\installer\gamenest.nsi')
    if ($LASTEXITCODE -ne 0) { throw 'NSIS 安装程序构建失败' }
    $checksum = (Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText(($installer + '.sha256'), "$checksum  $([IO.Path]::GetFileName($installer))`n", [Text.Encoding]::ASCII)
    Write-Output "安装程序：$installer"
    Write-Output "SHA-256：$checksum"
} finally { Pop-Location }
