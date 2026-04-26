param(
    [string]$Repo = "Ritiksuman07/quantflow",
    [string]$Description = "Agentic quantitative finance framework with SEC + Reddit agents, deterministic backtesting, and DuckDB analytics.",
    [string]$Homepage = "https://github.com/Ritiksuman07/quantflow#readme",
    [string[]]$Topics = @("quantitative-finance", "algorithmic-trading", "llm-agents", "duckdb", "backtesting")
)

if (-not $env:GITHUB_TOKEN) {
    throw "Missing GITHUB_TOKEN. Create a classic PAT with repo scope, then set `$env:GITHUB_TOKEN."
}

$headers = @{
    Authorization = "Bearer $($env:GITHUB_TOKEN)"
    Accept = "application/vnd.github+json"
    "X-GitHub-Api-Version" = "2022-11-28"
}

Write-Host "Updating description + homepage for $Repo ..."
Invoke-RestMethod `
    -Method Patch `
    -Uri "https://api.github.com/repos/$Repo" `
    -Headers $headers `
    -Body (@{
        description = $Description
        homepage = $Homepage
    } | ConvertTo-Json)

Write-Host "Updating topics for $Repo ..."
Invoke-RestMethod `
    -Method Put `
    -Uri "https://api.github.com/repos/$Repo/topics" `
    -Headers $headers `
    -Body (@{
        names = $Topics
    } | ConvertTo-Json)

Write-Host "Repository metadata updated successfully."
