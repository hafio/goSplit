#requires -Version 5.1
# GoSplit dev tasks. Behaviourally identical to dev.sh -- same task names, same
# gating, same footer format.
[CmdletBinding()]
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Tasks)

$ErrorActionPreference = 'Continue'

# Decode native-command stdout as UTF-8. Windows PowerShell 5.1 otherwise
# decodes it via the OEM code page (CP437), turning UTF-8 output (Trivy's
# box-drawing tables, accented text) into mojibake before we can log it.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = Split-Path -Parent $ScriptDir
$LogDir    = Join-Path $ScriptDir 'logs'
$Dist      = Join-Path $RepoRoot 'dist'
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
Set-Location $RepoRoot
$env:NO_COLOR = '1'

# Toolchain parity with CI: go.mod's `toolchain` directive is the single pin,
# and GOTOOLCHAIN makes any go binary honour it exactly. Set only when unset,
# so an exported value wins.
$goMod = Join-Path $RepoRoot 'go.mod'
if (-not $env:GOTOOLCHAIN -and (Test-Path $goMod)) {
  $m = Select-String -Path $goMod -Pattern '^toolchain (\S+)' | Select-Object -First 1
  if ($m) { $env:GOTOOLCHAIN = $m.Matches[0].Groups[1].Value }
}

# --- output helpers ---------------------------------------------------------
function Step { param($m) Write-Host "==> $m" -ForegroundColor Cyan }
function Ok   { param($m) Write-Host "ok: $m"    -ForegroundColor Green }
function Warn { param($m) Write-Host "warn: $m"  -ForegroundColor Yellow }
function Die  { param($m) Write-Host "error: $m" -ForegroundColor Red; exit 1 }

function Get-Now { (Get-Date).ToString('yyyy-MM-ddTHH:mm:sszzz') }
function Get-Log { param($Task) Join-Path $LogDir "$Task.log" }

function Start-TaskLog {
  param($Task)
  Set-Content -Path (Get-Log $Task) -Encoding utf8 `
    -Value ("=== {0} | {1} ===" -f (Get-Now), $Task)
}

function Write-Finish {
  param([string]$Task, [int]$Code, [int]$Seconds)
  $status = if ($Code -eq 0) { 'OK' } else { "FAILED (exit $Code)" }
  $line = '{0} | {1} | {2}s | {3}' -f (Get-Now), $Task, $Seconds, $status
  # Add-Content, never Tee-Object: Tee doubles lines and writes UTF-16.
  Add-Content -Path (Get-Log $Task) -Value $line -Encoding utf8
  Write-Host $line
}

# Capture once, write once. -Width stops column wrap; the CSI strip keeps the
# file readable plain text.
#
# Stderr ErrorRecords are flattened via .Exception.Message, not "$_": stringifying
# an ErrorRecord falls back to the exception TYPE NAME when the message is empty,
# so every blank stderr line docker's buildkit emits would log as the literal
# "System.Management.Automation.RemoteException".
function Invoke-Logged {
  # NB: $CmdArgs, not $Args -- $Args is a PowerShell automatic variable, so a
  # param named $Args binds empty and `& $Exe @Args` runs $Exe with no args.
  param([string]$Task, [string]$Exe, [string[]]$CmdArgs)
  $out = (& $Exe @CmdArgs 2>&1 | ForEach-Object {
            if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.Exception.Message }
            else { "$_" }
          } | Out-String -Width 4096)
  $code = $LASTEXITCODE
  $out = $out -replace "\x1b\[[0-9;?]*[a-zA-Z]", ""
  Add-Content -Path (Get-Log $Task) -Value $out -Encoding utf8
  Write-Host $out
  return $code
}

# --- target resolution ------------------------------------------------------
# CI sets TARGET_OS/TARGET_ARCH; unset means host. Translated to GOOS/GOARCH
# here and nowhere else.
function Get-HostArch {
  switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' } 'ARM64' { 'arm64' } default { 'amd64' }
  }
}
$TOs   = if ($env:TARGET_OS)   { $env:TARGET_OS }   else { 'windows' }
$TArch = if ($env:TARGET_ARCH) { $env:TARGET_ARCH } else { Get-HostArch }
$BinName = "gosplit-{0}-{1}" -f $TOs, $TArch
if ($TOs -eq 'windows') { $BinName = "$BinName.exe" }

# --- version ----------------------------------------------------------------
# The git tag is the single source of version truth. tag.yml passes VERSION to
# the image build from the tag ref, and this script passes it from `git
# describe`, so neither depends on `git describe` running inside the build
# container -- where .dockerignore's exclusions would make it report -dirty.
# Images take the tag with the leading v stripped (registry convention) -- the
# only place the v drops.
$Version = if ($env:VERSION) { $env:VERSION }
           else {
             $d = (git describe --tags --always --dirty 2>$null)
             if ($LASTEXITCODE -eq 0 -and $d) { "$d".Trim() } else { 'dev' }
           }
$ImageTag = $Version -replace '^v', ''
# Local image name/tag (override with $env:IMAGE_NAME). The published
# multi-arch image is built by tag.yml, not here.
$Image = if ($env:IMAGE_NAME) { $env:IMAGE_NAME } else { "gosplit:$ImageTag" }
# Trivy runs as a container (no local binary needed); override the tag.
$TrivyImage = if ($env:TRIVY_IMAGE) { $env:TRIVY_IMAGE } else { 'aquasec/trivy:latest' }

# --- tasks ------------------------------------------------------------------
# PowerShell returns EVERY uncaptured pipeline value, not just `return`: a bare
# external command inside a task turns $code into an array. Route commands
# through Invoke-Logged, or pipe anything you don't return to Out-Null.
function Task-build {
  New-Item -ItemType Directory -Force -Path $Dist | Out-Null
  # CGO-free (modernc.org/sqlite is pure Go) -> a static binary on every target.
  # -X main.version stamps the tag in; without it the binary reports "dev".
  $env:CGO_ENABLED = '0'; $env:GOOS = $TOs; $env:GOARCH = $TArch
  try {
    return (Invoke-Logged 'build' 'go' @(
      'build','-trimpath','-ldflags',"-s -w -X main.version=$Version",
      '-o',(Join-Path $Dist $BinName),'./cmd/gosplit'))
  } finally {
    Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
  }
}

function Task-vet  { return (Invoke-Logged 'vet'  'go' @('vet','./...')) }
function Task-test { return (Invoke-Logged 'test' 'go' @('test','./...','-count=1')) }

function Task-cov {
  $prof = Join-Path $LogDir 'coverage.out'
  $html = Join-Path $LogDir 'coverage.html'
  # -p 1 serializes package test binaries: on Windows, parallel runs race to
  # exec the shared covdata.exe and hit "file in use" locks (often Defender).
  $c = Invoke-Logged 'cov' 'go' @(
    'test','-covermode=atomic',"-coverprofile=$prof",'-p','1','-count=1','./...')
  if ($c -ne 0) { return $c }
  $c = Invoke-Logged 'cov' 'go' @('tool','cover',"-html=$prof","-o",$html)
  if ($c -ne 0) { return $c }
  # The printed total is the floor for the next run; logs/cov.log keeps it.
  $total = (& go tool cover "-func=$prof" | Select-Object -Last 1)
  Add-Content -Path (Get-Log 'cov') -Value "$total" -Encoding utf8
  Write-Host "$total"
  Ok "cov -> $html"
  return 0
}

# GitHub's windows runners cannot run linux containers, and a machine without
# Docker has nothing to build against. Both are "does not apply here", not a
# misconfiguration -- the ubuntu leg covers the image tasks.
function Test-DockerLinux {
  if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { return $false }
  $os = (& docker version -f '{{.Server.Os}}' 2>$null)
  return ($LASTEXITCODE -eq 0 -and "$os".Trim() -eq 'linux')
}

# The actual build, logged under the calling task so a standalone `scan` never
# appends to a stale image.log.
function Build-Image {
  param([string]$Task)
  return (Invoke-Logged $Task 'docker' @(
    'build','--progress=plain','--build-arg',"VERSION=$Version",'-t',$Image,$RepoRoot))
}

function Task-image {
  if (-not (Test-DockerLinux)) {
    Warn 'no docker daemon running linux containers; skipping image'; return 0
  }
  return (Build-Image 'image')
}

# One task, every applicable check. FATAL on fixable CVEs.
function Task-scan {
  # go tool, not `go run pkg@version`: the latter runs module-less and ignores
  # go.mod's toolchain pin, so it builds against the minimum Go its own module
  # accepts and then refuses to load this module's packages.
  $c = Invoke-Logged 'scan' 'go' @('tool','govulncheck','./...')
  if ($c -ne 0) { return $c }

  # Image half: never against a stale image -- build first, because `image` is
  # not in `all`.
  if (-not (Test-DockerLinux)) {
    Warn 'no docker daemon running linux containers; skipping image scan'; return 0
  }
  $c = Build-Image 'scan'; if ($c -ne 0) { return $c }
  # Trivy runs in a container, so it needs the Docker socket to read the image
  # just built -- without it it finds nothing locally and tries to PULL the
  # image from Docker Hub, which 401s. The named volume caches the vulnerability
  # DB between runs (the DB itself still refreshes; only the re-fetch is saved).
  # --pull=always matters more than the tag: without it docker reuses whatever
  # :latest resolved to months ago -- unpinned AND stale.
  # --scanners vuln keeps this gate about CVEs. Trivy also ships secret and
  # misconfiguration detectors, and with --exit-code 1 those would hard-fail
  # scan (and CI, and the release) on advisory findings this task never claimed
  # to cover.
  return (Invoke-Logged 'scan' 'docker' @(
    'run','--rm','--pull=always',
    '-e','NO_COLOR=1',
    '-v','/var/run/docker.sock:/var/run/docker.sock',
    '-v','trivy-cache:/root/.cache/',
    $TrivyImage,'image',
    '--quiet','--scanners','vuln','--exit-code','1','--severity','HIGH,CRITICAL',
    '--ignore-unfixed',$Image))
}

function Task-up   { return (Invoke-Logged 'up'   'docker' @('compose','up','-d')) }
function Task-down { return (Invoke-Logged 'down' 'docker' @('compose','down')) }

# Local only: the graph is a developer artifact, not a CI output.
function Task-graphify {
  if ($env:CI) { Warn 'graphify is local-only; skipping in CI'; return 0 }
  if (-not (Get-Command graphify -ErrorAction SilentlyContinue)) {
    Warn 'graphify not on PATH; skipping'; return 0
  }
  return (Invoke-Logged 'graphify' 'graphify' @('update','.'))
}

# --- dispatch ---------------------------------------------------------------
$All  = @('build','vet','test')
$Full = @('build','vet','test','cov','image','scan','graphify')

function Show-Usage {
  @"
usage: dev.ps1 <task>...

  build vet test cov scan image up down graphify
  all   = $($All -join ' ')            (what CI runs, as: all scan)
  full  = $($Full -join ' ')           (pre-tag sweep)

  version $Version -> image $Image
  override: VERSION, IMAGE_NAME, TRIVY_IMAGE, TARGET_OS, TARGET_ARCH
"@ | Write-Host
}

if (-not $Tasks -or $Tasks[0] -in @('-h','--help','help')) { Show-Usage; exit 0 }

$queue = @()
foreach ($t in $Tasks) {
  switch ($t) { 'all' { $queue += $All } 'full' { $queue += $Full } default { $queue += $t } }
}

$failed = 0
foreach ($task in $queue) {
  if (-not (Get-Command "Task-$task" -ErrorAction SilentlyContinue)) { Die "unknown task: $task" }
  Step $task
  Start-TaskLog $task
  $sw = [Diagnostics.Stopwatch]::StartNew()
  $code = 0
  try { $code = & "Task-$task" } catch { $code = 1 }
  if ($null -eq $code) { $code = 0 }
  $sw.Stop()
  Write-Finish -Task $task -Code $code -Seconds ([int]$sw.Elapsed.TotalSeconds)
  if ($code -ne 0) { $failed = 1; Warn "$task failed; stopping"; break }
  Ok $task
}
exit $failed
