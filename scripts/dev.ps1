# GoSplit task runner (PowerShell). Mirror of dev.sh. Run from anywhere.
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Tasks)

$ErrorActionPreference = 'Continue'

# Decode native-command stdout as UTF-8. Windows PowerShell 5.1 otherwise decodes
# it via the OEM code page (CP437), turning UTF-8 output (Trivy's box-drawing
# tables, emoji, accented text) into mojibake before we can log it.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent $ScriptDir
$LogDir = Join-Path $ScriptDir 'logs'
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
Set-Location $Root

# Image name/tag (override with $env:IMAGE_NAME).
$Image = if ($env:IMAGE_NAME) { $env:IMAGE_NAME } else { 'gosplit:dev' }
# Trivy runs as a container image (no local binary needed); override the tag.
$TrivyImage = if ($env:TRIVY_IMAGE) { $env:TRIVY_IMAGE } else { 'aquasec/trivy:latest' }
# Python for the graphify update (override with $env:PYTHON).
$Python = if ($env:PYTHON) { $env:PYTHON }
  elseif (Get-Command python -ErrorAction SilentlyContinue) { 'python' }
  elseif (Get-Command python3 -ErrorAction SilentlyContinue) { 'python3' }
  else { 'python' }

function Step($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Ok($m)   { Write-Host "ok: $m" -ForegroundColor Green }
function Warn($m) { Write-Host "warn: $m" -ForegroundColor Yellow }
function Die($m)  { Write-Host "FAIL: $m" -ForegroundColor Red; exit 1 }

function Run($name, [scriptblock]$cmd) {
  $log = Join-Path $LogDir "$name.log"
  # Windows PowerShell 5.1 quirks this guards against:
  #  - `2>&1` wraps each stderr line (go build/vet/test errors) in a verbose
  #    multi-line ErrorRecord -> `"$_"` flattens each to its plain text.
  #  - Out-String wraps at the console width -> `-Width 4096` keeps lines whole.
  #  - Tee-Object passes objects through AND mixes UTF-16 into a UTF-8 file ->
  #    capture once, echo live, and write the file once with a single encoding.
  $out = (& $cmd 2>&1 | ForEach-Object { "$_" } | Out-String -Width 4096)
  $code = $LASTEXITCODE
  Set-Content -Path $log -Value "== $name ==`r`n$out" -Encoding utf8
  Write-Host -NoNewline $out
  return $code
}

function Task-Build { Step 'build'; if ((Run 'build' { go build ./... }) -eq 0) { Ok 'build' } else { Die 'build' } }
function Task-Vet   { Step 'vet';   if ((Run 'vet'   { go vet ./... })   -eq 0) { Ok 'vet' }   else { Die 'vet' } }
function Task-Test  { Step 'test';  if ((Run 'test'  { go test ./... -count=1  })  -eq 0) { Ok 'test' }  else { Die 'test' } }
function Task-Cov {
  Step 'cov'
  $prof = Join-Path $ScriptDir 'coverage.out'
  # `-p 1` serializes package test binaries: on Windows, parallel runs race to
  # exec the shared covdata.exe and hit "file in use" locks (often Defender).
  if ((Run 'cov' { go test -covermode=atomic -coverprofile=$prof -p 1 -count=1 ./... }) -ne 0) { Die 'cov' }
  go tool cover -html=$prof -o (Join-Path $ScriptDir 'coverage.html')
  go tool cover -func=$prof | Select-Object -Last 1
  Ok 'cov'
}
function Task-Vuln { Step 'vuln'; if ((Run 'vuln' { govulncheck ./... }) -ne 0) { Warn 'vuln (report-only)' } }
# Build the distroless container image from the repo Dockerfile.
function Task-Image { Step "image ($Image)"; if ((Run 'image' { docker build -t $Image $Root }) -eq 0) { Ok 'image' } else { Die 'image' } }
# Scan the built image for CVEs (report-only) using the Trivy container image --
# mounts the Docker socket to read the local image and a named volume to cache
# the vulnerability DB between runs. Requires the image to exist.
function Task-Trivy {
  Step "trivy ($Image)"
  $code = Run 'trivy' {
    docker run --rm `
      -e NO_COLOR=1 `
      -v /var/run/docker.sock:/var/run/docker.sock `
      -v trivy-cache:/root/.cache/ `
      $TrivyImage image --quiet --scanners vuln --exit-code 0 --ignore-unfixed $Image
  }
  if ($code -ne 0) { Warn 'trivy (report-only)' }
}

# Incrementally update the graphify knowledge graph (AST-only, no API cost).
# Report-only: a missing python/graphify or absent graph warns, never aborts.
function Task-Graphify {
  Step 'graphify'
  $env:NO_COLOR = '1'
  if ((Run 'graphify' { & $Python -m graphify update . }) -eq 0) { Ok 'graphify' } else { Warn 'graphify (report-only)' }
}

function Task-All  { Task-Build; Task-Vet; Task-Test; Task-Cov; Task-Graphify }
function Task-Scan { Task-Vuln; Task-Image; Task-Trivy }
function Task-Full { Task-All; Task-Scan }

function Usage { Write-Host "Usage: .\scripts\dev.ps1 <task> [task...]`n  build  vet  test  cov  graphify  vuln  image  trivy  scan  all  full" }

if (-not $Tasks -or $Tasks.Count -eq 0) { Usage; exit 0 }
foreach ($t in $Tasks) {
  switch ($t) {
    'build' { Task-Build }
    'vet'   { Task-Vet }
    'test'  { Task-Test }
    'cov'   { Task-Cov }
    'graphify' { Task-Graphify }
    'vuln'  { Task-Vuln }
    'image' { Task-Image }
    'trivy' { Task-Trivy }
    'scan'  { Task-Scan }
    'all'   { Task-All }
    'full'  { Task-Full }
    default { if ($t -in @('-h','--help','help')) { Usage } else { Die "unknown task: $t" } }
  }
}
