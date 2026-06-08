param(
    [string]$ClientId  # optional — auto-discovered from role assignments if omitted
)

$ErrorActionPreference = "Stop"
$ok = $true

function Section($title) { Write-Host "`n=== $title ===" -ForegroundColor Cyan }
function Pass($msg)       { Write-Host "  OK   $msg" -ForegroundColor Green }
function Fail($msg)       { Write-Host "  FAIL $msg" -ForegroundColor Red; $script:ok = $false }
function Info($msg)       { Write-Host "       $msg" -ForegroundColor Gray }

function Pick($prompt, $items, $labelProp) {
    if ($items.Count -eq 1) { return $items[0] }
    Write-Host "`n  $prompt" -ForegroundColor Yellow
    for ($i = 0; $i -lt $items.Count; $i++) {
        Write-Host "  [$($i+1)] $($items[$i].$labelProp)"
    }
    $choice = Read-Host "  Enter number"
    return $items[[int]$choice - 1]
}

# ── 1. Login / subscription ────────────────────────────────────────────────
Section "Subscription"
$account = az account show 2>$null | ConvertFrom-Json
if (-not $account) { Write-Host "Not logged in — run 'az login' first" -ForegroundColor Red; exit 1 }
Pass "Logged in"
Info "Subscription : $($account.name)"
Info "Tenant       : $($account.tenantId)"
$subId = $account.id

# ── 2. Discover Trusted Signing account ───────────────────────────────────
Section "Trusted Signing Account"
$accounts = az resource list `
    --resource-type Microsoft.CodeSigning/codeSigningAccounts `
    --query "[].{name:name, resourceGroup:resourceGroup, location:location}" `
    2>$null | ConvertFrom-Json

if (-not $accounts -or $accounts.Count -eq 0) {
    Fail "No Trusted Signing accounts found in subscription '$($account.name)'"
    exit 1
}

$chosen = Pick "Multiple accounts found — pick one:" $accounts "name"
$AccountName   = $chosen.name
$ResourceGroup = $chosen.resourceGroup

$resource = az resource show `
    --resource-group $ResourceGroup `
    --resource-type Microsoft.CodeSigning/codeSigningAccounts `
    --name $AccountName | ConvertFrom-Json

Pass "Account found"
$endpoint = $resource.properties.accountUri
Info "Account        : $AccountName"
Info "Resource group : $ResourceGroup"
Info "Endpoint       : $endpoint"

# ── 3. Discover certificate profile ──────────────────────────────────────
Section "Certificate Profile"
$profilesUrl = "https://management.azure.com/subscriptions/$subId/resourceGroups/$ResourceGroup" +
               "/providers/Microsoft.CodeSigning/codeSigningAccounts/$AccountName" +
               "/certificateProfiles?api-version=2024-09-30-preview"
$profiles = az rest --method GET --url $profilesUrl | ConvertFrom-Json

if (-not $profiles.value -or $profiles.value.Count -eq 0) {
    Fail "No certificate profiles found under account '$AccountName'"
    exit 1
}

$chosenProfile = Pick "Multiple profiles found — pick one:" $profiles.value "name"
$ProfileName   = $chosenProfile.name
$profileStatus = $chosenProfile.properties.status

Pass "Profile found: $ProfileName"
if ($profileStatus -eq "Active") {
    Pass "Profile status: Active"
} else {
    Fail "Profile status is '$profileStatus' — must be Active to sign"
}

# ── 4. Discover service principal from role assignments ───────────────────
Section "Service Principal"
$profileScope = "/subscriptions/$subId/resourceGroups/$ResourceGroup" +
                "/providers/Microsoft.CodeSigning/codeSigningAccounts/$AccountName" +
                "/certificateProfiles/$ProfileName"

$assignments = az role assignment list --scope $profileScope `
    --role "Trusted Signing Certificate Profile Signer" `
    --query "[].{principalId:principalId, principalName:principalName, principalType:principalType}" `
    2>$null | ConvertFrom-Json

if (-not $ClientId) {
    $spAssignments = $assignments | Where-Object { $_.principalType -eq "ServicePrincipal" }
    if (-not $spAssignments -or $spAssignments.Count -eq 0) {
        Fail "No service principal with 'Trusted Signing Certificate Profile Signer' found on profile '$ProfileName'"
        $ClientId = $null
    } elseif ($spAssignments.Count -eq 1) {
        $ClientId = $spAssignments[0].principalId
        Info "Auto-discovered service principal: $($spAssignments[0].principalName) ($ClientId)"
    } else {
        $chosenSp = Pick "Multiple service principals found — pick one:" $spAssignments "principalName"
        $ClientId = $chosenSp.principalId
    }
}

if ($ClientId) {
    $sp = az ad sp show --id $ClientId 2>$null | ConvertFrom-Json
    if (-not $sp) {
        Fail "Service principal '$ClientId' not found in this tenant"
        $ClientId = $null
    } else {
        Pass "Service principal found"
        Info "Display name : $($sp.displayName)"
        Info "App ID       : $($sp.appId)"
        $ClientId = $sp.appId  # ensure we use the appId (not object ID) for the summary
    }
}

# ── 5. Role assignment check ──────────────────────────────────────────────
Section "Role Assignment at Certificate Profile Level"
$accountScope = "/subscriptions/$subId/resourceGroups/$ResourceGroup" +
                "/providers/Microsoft.CodeSigning/codeSigningAccounts/$AccountName"

if ($assignments -and $assignments.Count -gt 0) {
    Pass "Trusted Signing Certificate Profile Signer assigned to:"
    $assignments | ForEach-Object { Info "$($_.principalName) [$($_.principalType)]" }
} else {
    Fail "No assignments for 'Trusted Signing Certificate Profile Signer' on profile '$ProfileName'"
    Write-Host "`n  To fix, run:" -ForegroundColor Yellow
    Write-Host "  az role assignment create ``" -ForegroundColor Yellow
    Write-Host "    --role `"Trusted Signing Certificate Profile Signer`" ``" -ForegroundColor Yellow
    Write-Host "    --assignee <YOUR_CLIENT_ID> ``" -ForegroundColor Yellow
    Write-Host "    --scope `"$profileScope`"" -ForegroundColor Yellow
}

# ── Summary ───────────────────────────────────────────────────────────────
Section "GitHub Secrets Checklist"
if ($ok) {
    Write-Host "  All checks passed." -ForegroundColor Green
}
$secrets = [ordered]@{
    AZURE_TENANT_ID                = $account.tenantId
    AZURE_CLIENT_ID                = if ($ClientId) { $ClientId } else { "(not found)" }
    AZURE_CLIENT_SECRET            = "(your app registration secret)"
    AZURE_TRUSTED_SIGNING_ENDPOINT = $endpoint
    AZURE_TRUSTED_SIGNING_ACCOUNT  = $AccountName
    AZURE_TRUSTED_SIGNING_PROFILE  = $ProfileName
}
Write-Host ""
$secrets.GetEnumerator() | ForEach-Object {
    Write-Host ("  {0,-40} = {1}" -f $_.Key, $_.Value)
}
if (-not $ok) {
    Write-Host "`n  Fix the failures above before updating secrets." -ForegroundColor Red
}
