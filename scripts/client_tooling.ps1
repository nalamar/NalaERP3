[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('format', 'test', 'analyze', 'build-web')]
    [string]$Action,

    [string[]]$Paths = @(),

    [string]$TestTarget = 'test/sales_order_context_pages_test.dart'
)

$ErrorActionPreference = 'Stop'

function Resolve-FlutterRoot {
    if ($env:FLUTTER_ROOT -and (Test-Path (Join-Path $env:FLUTTER_ROOT 'bin\cache\flutter_tools.snapshot'))) {
        return $env:FLUTTER_ROOT
    }

    $candidates = @(
        'C:\Projekte\flutter',
        'C:\src\flutter'
    )

    foreach ($candidate in $candidates) {
        if (Test-Path (Join-Path $candidate 'bin\cache\flutter_tools.snapshot')) {
            return $candidate
        }
    }

    $flutterCmd = Get-Command flutter -ErrorAction SilentlyContinue
    if ($flutterCmd) {
        $binDir = Split-Path -Parent $flutterCmd.Source
        $root = Split-Path -Parent $binDir
        if (Test-Path (Join-Path $root 'bin\cache\flutter_tools.snapshot')) {
            return $root
        }
    }

    throw 'Flutter SDK konnte nicht gefunden werden. Bitte FLUTTER_ROOT setzen oder das SDK unter C:\Projekte\flutter bereitstellen.'
}

function Invoke-Dart {
    param(
        [string[]]$Arguments
    )

    & $script:dartExe @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Dart-Aufruf fehlgeschlagen: $($Arguments -join ' ')"
    }
}

function Invoke-FlutterTool {
    param(
        [string[]]$Arguments
    )

    & $script:dartExe $script:flutterSnapshot @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Flutter-Tool-Aufruf fehlgeschlagen: $($Arguments -join ' ')"
    }
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$clientRoot = Join-Path $repoRoot 'client'
$flutterRoot = Resolve-FlutterRoot
$script:dartExe = Join-Path $flutterRoot 'bin\cache\dart-sdk\bin\dart.exe'
$script:flutterSnapshot = Join-Path $flutterRoot 'bin\cache\flutter_tools.snapshot'

if ($Paths.Count -eq 1 -and $Paths[0].Contains(',')) {
    $Paths = $Paths[0].Split(',', [System.StringSplitOptions]::RemoveEmptyEntries) |
        ForEach-Object { $_.Trim() }
}

if (-not (Test-Path $script:dartExe)) {
    throw "dart.exe wurde nicht gefunden unter $script:dartExe"
}

if (-not (Test-Path $script:flutterSnapshot)) {
    throw "flutter_tools.snapshot wurde nicht gefunden unter $script:flutterSnapshot"
}

Push-Location $repoRoot
try {
    switch ($Action) {
        'format' {
            if ($Paths.Count -eq 0) {
                $Paths = @('client/lib', 'client/test')
            }
            Invoke-Dart -Arguments (@('format') + $Paths)
        }
        'test' {
            Push-Location $clientRoot
            try {
                Invoke-FlutterTool -Arguments @('test', $TestTarget, '--suppress-analytics')
            } finally {
                Pop-Location
            }
        }
        'analyze' {
            Push-Location $clientRoot
            try {
                Invoke-FlutterTool -Arguments @('analyze', '--suppress-analytics')
            } finally {
                Pop-Location
            }
        }
        'build-web' {
            Push-Location $clientRoot
            try {
                Invoke-FlutterTool -Arguments @('build', 'web', '--release', '--no-wasm-dry-run', '--suppress-analytics')
            } finally {
                Pop-Location
            }
        }
    }
} finally {
    Pop-Location
}
