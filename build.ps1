#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Build script for the Text-Based Boss Raid Game.
.DESCRIPTION
    This script builds the game for various operating systems and architectures.
    It creates binaries for Windows, macOS, and Linux in both 32-bit and 64-bit architectures.
.EXAMPLE
    ./build.ps1
    Builds all supported platforms and architectures.
.EXAMPLE
    ./build.ps1 -os windows -arch amd64
    Builds only for Windows 64-bit.
.PARAMETER os
    The operating system to build for. Can be "windows", "darwin", "linux", or "all".
.PARAMETER arch
    The architecture to build for. Can be "386", "amd64", "arm", "arm64", or "all".
.PARAMETER output
    The output directory for the binaries. Default is "./bin".
#>

param (
    [string]$os = "all",
    [string]$arch = "all",
    [string]$output = "./bin"
)

# Application name
$appName = "boss-raid-game"

# Supported platforms
$platforms = @{
    "windows" = @{
        "extension" = ".exe"
        "architectures" = @("386", "amd64", "arm", "arm64")
    }
    "darwin" = @{
        "extension" = ""
        "architectures" = @("amd64", "arm64")
    }
    "linux" = @{
        "extension" = ""
        "architectures" = @("386", "amd64", "arm", "arm64")
    }
}

# Create output directory if it doesn't exist
if (-not (Test-Path $output)) {
    New-Item -ItemType Directory -Path $output | Out-Null
    Write-Host "Created output directory: $output"
}

# Function to build for a specific OS and architecture
function Build-Binary {
    param (
        [string]$targetOS,
        [string]$targetArch
    )
    
    $extension = $platforms[$targetOS].extension
    $outputFile = "${output}/${appName}_${targetOS}_${targetArch}${extension}"
    
    Write-Host "Building for $targetOS/$targetArch..."
    
    $env:GOOS = $targetOS
    $env:GOARCH = $targetArch
    
    # For ARM, set GOARM to 7 (ARMv7) as a default
    if ($targetArch -eq "arm") {
        $env:GOARM = "7"
    }
    
    go build -o $outputFile -ldflags="-s -w" .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Successfully built: $outputFile" -ForegroundColor Green
    } else {
        Write-Host "Failed to build for $targetOS/$targetArch" -ForegroundColor Red
    }
}

# Build for specified platforms
if ($os -eq "all") {
    foreach ($targetOS in $platforms.Keys) {
        $architectures = $platforms[$targetOS].architectures
        
        if ($arch -eq "all") {
            foreach ($targetArch in $architectures) {
                Build-Binary -targetOS $targetOS -targetArch $targetArch
            }
        } else {
            if ($architectures -contains $arch) {
                Build-Binary -targetOS $targetOS -targetArch $arch
            } else {
                Write-Host "Architecture $arch is not supported for $targetOS" -ForegroundColor Yellow
            }
        }
    }
} else {
    if ($platforms.ContainsKey($os)) {
        $architectures = $platforms[$os].architectures
        
        if ($arch -eq "all") {
            foreach ($targetArch in $architectures) {
                Build-Binary -targetOS $os -targetArch $targetArch
            }
        } else {
            if ($architectures -contains $arch) {
                Build-Binary -targetOS $os -targetArch $arch
            } else {
                Write-Host "Architecture $arch is not supported for $os" -ForegroundColor Yellow
            }
        }
    } else {
        Write-Host "Operating system $os is not supported" -ForegroundColor Red
    }
}

Write-Host "Build process completed!" -ForegroundColor Green
