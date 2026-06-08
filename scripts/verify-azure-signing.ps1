param(
    [Parameter(Mandatory)][string]$ResourceGroup,
    [Parameter(Mandatory)][string]$AccountName,
    [Parameter(Mandatory)][string]$ProfileName,
    [Parameter(Mandatory)][string]$ClientId
)

$ErrorActionPreference = "Stop"
$ok = $true

function Section($title) {
    Write-Host "`n=== $title ===" -ForegroundColor Cyan
}

function Pass($msg) { Write-Host "  OK  $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "  FAIL $msg" -ForegroundColor Red; $script:ok = $false }
function Info($msg) { Write-Host "       $msg" -ForegroundColor Gray }

# ── 1. Login / subscription ────────────────────────────────────────────────
Section "Subscription"
$account = az account show | ConvertFrom-Json
if (-not $account) { Fail "Not logged in — run 'az login' first"; exit 1 }
Pass "Logged in"
Info "Subscription : $($account.name)"
Info "Tenant       : $($account.tenantId)"
Info "ID           : $($account.id)"
$subId = $account.id

# ── 2. Trusted Signing account ────────────────────────────────────────────
Section "Trusted Signing Account"
$resource = az resource show `
    --resource-group $ResourceGroup `
    --resource-type Microsoft.CodeSigning/codeSigningAccounts `
    --name $AccountName 2>$null | ConvertFrom-Json

if (-not $resource) {
    Fail "Account '$AccountName' not found in resource group '$ResourceGroup'"
} else {
    Pass "Account found"
    $endpoint = $resource.properties.accountUri
    Info "Endpoint (AZURE_TRUSTED_SIGNING_ENDPOINT) : $endpoint"
}

# ── 3. Certificate profile ────────────────────────────────────────────────
Section "Certificate Profile"
$profilesUrl = "https://management.azure.com/subscriptions/$subId/resourceGroups/$ResourceGroup" +
               "/providers/Microsoft.CodeSigning/codeSigningAccounts/$AccountName" +
               "/certificateProfiles?api-version=2024-09-30-preview"
$profiles = az rest --method GET --url $profilesUrl 2>$null | ConvertFrom-Json

$profile = $profiles.value | Where-Object { $_.name -eq $ProfileName }
if (-not $profile) {
    Fail "Profile '$ProfileName' not found. Available profiles:"
    $profiles.value | ForEach-Object { Info "  $($_.name) [$($_.properties.status)]" }
} else {
    Pass "Profile found"
    Info "Status : $($profile.properties.status)"
    if ($profile.properties.status -ne "Active") {
        Fail "Profile status is '$($profile.properties.status)' — must be 'Active' to sign"
    }
}

# ── 4. Service principal ──────────────────────────────────────────────────
Section "Service Principal (App Registration)"
$sp = az ad sp show --id $ClientId 2>$null | ConvertFrom-Json
if (-not $sp) {
    Fail "Service principal '$ClientId' not found in this tenant"
} else {
    Pass "Service principal found"
    Info "Display name : $($sp.displayName)"
    Info "App ID       : $($sp.appId)"
}

# ── 5. Role at account level ──────────────────────────────────────────────
Section "Role Assignment — Account Level"
$accountScope = "/subscriptions/$subId/resourceGroups/$ResourceGroup" +
                "/providers/Microsoft.CodeSigning/codeSigningAccounts/$AccountName"
$accountRoles = az role assignment list --scope $accountScope --assignee $ClientId `
    --query "[].roleDefinitionName" 2>$null | ConvertFrom-Json

if ($accountRoles -contains "Trusted Signing Certificate Profile Signer") {
    Pass "Trusted Signing Certificate Profile Signer assigned at account level"
} else {
    Info "Role not assigned at account level (may still be OK if assigned at profile level)"
}

# ── 6. Role at certificate profile level (required) ───────────────────────
Section "Role Assignment — Certificate Profile Level (required)"
$profileScope = $accountScope + "/certificateProfiles/$ProfileName"
$profileRoles = az role assignment list --scope $profileScope --assignee $ClientId `
    --query "[].roleDefinitionName" 2>$null | ConvertFrom-Json

if ($profileRoles -contains "Trusted Signing Certificate Profile Signer") {
    Pass "Trusted Signing Certificate Profile Signer assigned at profile level"
} else {
    Fail "Role NOT assigned at profile level"
    Write-Host "`n  To fix, run:" -ForegroundColor Yellow
    Write-Host "  az role assignment create ``" -ForegroundColor Yellow
    Write-Host "    --role `"Trusted Signing Certificate Profile Signer`" ``" -ForegroundColor Yellow
    Write-Host "    --assignee $ClientId ``" -ForegroundColor Yellow
    Write-Host "    --scope `"$profileScope`"" -ForegroundColor Yellow
}

# ── Summary ───────────────────────────────────────────────────────────────
Section "Summary"
if ($ok) {
    Write-Host "  All checks passed." -ForegroundColor Green
    Write-Host "`n  GitHub secrets checklist:" -ForegroundColor Cyan
    Write-Host "    AZURE_TENANT_ID                  = $($account.tenantId)"
    Write-Host "    AZURE_CLIENT_ID                  = $ClientId"
    Write-Host "    AZURE_CLIENT_SECRET              = (your app registration secret)"
    Write-Host "    AZURE_TRUSTED_SIGNING_ENDPOINT   = $endpoint"
    Write-Host "    AZURE_TRUSTED_SIGNING_ACCOUNT    = $AccountName"
    Write-Host "    AZURE_TRUSTED_SIGNING_PROFILE    = $ProfileName"
} else {
    Write-Host "  One or more checks failed — see above." -ForegroundColor Red
}
