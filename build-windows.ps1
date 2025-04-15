#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Windows-specific build script for the Text-Based Boss Raid Game.
.DESCRIPTION
    This script builds the game for various operating systems and architectures.
    It creates binaries for Windows, macOS, and Linux in both 32-bit and 64-bit architectures.
.EXAMPLE
    ./build-windows.ps1
    Builds all supported platforms and architectures.
.EXAMPLE
    ./build-windows.ps1 -os windows -arch amd64
    Builds only for Windows 64-bit.
.PARAMETER os
    The operating system to build for. Can be "windows", "darwin", "linux", or "all".
.PARAMETER arch
    The architecture to build for. Can be "386", "amd64", "arm", "arm64", or "all".
.PARAMETER output
    The output directory for the binaries. Default is "./bin".
.PARAMETER clean
    Clean the output directory before building.
#>

param (
    [string]$os = "all",
    [string]$arch = "all",
    [string]$output = "./bin",
    [switch]$clean = $false
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

# Clean if requested
if ($clean) {
    Write-Host "Cleaning..." -ForegroundColor Cyan
    if (Test-Path $output) {
        Remove-Item -Path $output -Recurse -Force
        Write-Host "Removed output directory: $output" -ForegroundColor Gray
    }
    & go clean
    Write-Host "Cleaned!" -ForegroundColor Green
}

# Create output directory if it doesn't exist
if (-not (Test-Path $output)) {
    New-Item -ItemType Directory -Path $output | Out-Null
    Write-Host "Created output directory: $output" -ForegroundColor Gray
}

# Function to build for a specific OS and architecture
function Build-Binary {
    param (
        [string]$targetOS,
        [string]$targetArch
    )
    
    $extension = $platforms[$targetOS].extension
    $outputFile = Join-Path -Path $output -ChildPath "${appName}_${targetOS}_${targetArch}${extension}"
    
    Write-Host "Building for $targetOS/$targetArch..." -ForegroundColor Cyan
    
    $env:GOOS = $targetOS
    $env:GOARCH = $targetArch
    
    # For ARM, set GOARM to 7 (ARMv7) as a default
    if ($targetArch -eq "arm") {
        $env:GOARM = "7"
    }
    
    & go build -o $outputFile -ldflags="-s -w" .
    
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
