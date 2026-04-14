# Build Script for Windows (PowerShell)
# Usage: .\scripts\build.ps1 [version]

param (
    [string]$Version = "dev"
)

$DistDir = "dist"
if (!(Test-Path $DistDir)) {
    New-Item -ItemType Directory -Path $DistDir
}

$Platforms = @(
    "linux/amd64",
    "linux/arm64",
    "darwin/amd64",
    "darwin/arm64",
    "windows/amd64"
)

$BuildTime = Get-Date -Format "yyyy-MM-dd_HH:mm:ss"
$LdFlags = "-s -w -X 'main.Version=$Version'"

Write-Host "🚀 Starting Litebin Monitor Build (Version: $Version)" -ForegroundColor Cyan

foreach ($Platform in $Platforms) {
    $Parts = $Platform.Split("/")
    $GOOS = $Parts[0]
    $GOARCH = $Parts[1]
    
    $BinaryName = "litebin-monitor-$GOOS-$GOARCH"
    if ($GOOS -eq "windows") { $BinaryName += ".exe" }
    
    $OutputPath = Join-Path $DistDir $BinaryName
    
    Write-Host "Building for $GOOS/$GOARCH..." -NoNewline
    
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    $env:CGO_ENABLED = 0
    
    go build -ldflags $LdFlags -o $OutputPath .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host " [OK]" -ForegroundColor Green
        
        # Try UPX compression if available
        if (Get-Command upx -ErrorAction SilentlyContinue) {
            Write-Host "  Compressing with UPX..." -NoNewline
            upx --best $OutputPath | Out-Null
            Write-Host " [Done]" -ForegroundColor Yellow
        }
    } else {
        Write-Host " [FAILED]" -ForegroundColor Red
    }
}

Write-Host "Build complete! Binaries are in the '$DistDir' folder." -ForegroundColor Green
