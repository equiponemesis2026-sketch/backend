param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$Prefix = "test-sos"
)

$ErrorActionPreference = "Stop"
$script:Base = $BaseUrl.TrimEnd("/")

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        $Body,
        [string]$Token
    )
    $headers = @{}
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }
    $params = @{
        Method = $Method
        Uri    = "$script:Base$Path"
        Headers = $headers
    }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = $Body | ConvertTo-Json -Depth 10
    }
    $resp = Invoke-RestMethod @params
    return $resp
}

function Get-LoginToken {
    param([string]$Email, [string]$Password)
    $login = Invoke-Api -Method POST -Path "/api/v1/auth/login" -Body @{ email = $Email; password = $Password }
    return $login.data.access_token
}

Write-Host "=== TEST E2E: Motor de Alertas SOS (dry-run) ===" -ForegroundColor Cyan
Write-Host "Base: $BaseUrl`n"

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()

# 1. Registrar víctima y observador
$victimEmail = "${Prefix}-victim-${suffix}@test.com"
$obsEmail    = "${Prefix}-obs-${suffix}@test.com"
$password    = "Password123!"

Write-Host "[1/7] Registrando víctima: $victimEmail"
$null = Invoke-Api -Method POST -Path "/api/v1/auth/register" -Body @{
    name = "Víctima test"; email = $victimEmail; password = $password; phone = "5520000001"
}

Write-Host "[2/7] Registrando observador: $obsEmail"
$obs = Invoke-Api -Method POST -Path "/api/v1/auth/register" -Body @{
    name = "Observador test"; email = $obsEmail; password = $password; phone = "5520000002"
}
$observerID = $obs.data.user_id
Write-Host "      observer id: $observerID"

# 2. Logins
$victimToken = Get-LoginToken $victimEmail $password
$obsToken    = Get-LoginToken $obsEmail $password
Write-Host "[3/7] Logins OK"

# 3. Víctima crea contacto con el email del observador (auto-link)
Write-Host "[4/7] Víctima crea contacto con email del observador (auto-link)"
$contact = Invoke-Api -Method POST -Path "/api/v1/contacts/" -Token $victimToken -Body @{
    name = "Mi apoyo"; phone = "5520000002"; email = $obsEmail; relationship = "familiar"
}
$contactID = $contact.data.contact_id
Write-Host "      contact id: $contactID"
Write-Host "      linked_user_id: $($contact.data.linked_user_id)"

if (-not $contact.data.linked_user_id) {
    Write-Host "      ADVERTENCIA: no se auto-vinculó. Probando POST /api/v1/contacts/{id}/link" -ForegroundColor Yellow
    $null = Invoke-Api -Method POST -Path "/api/v1/contacts/$contactID/link" -Token $victimToken -Body @{ linked_user_id = $observerID }
}

# 4. Observador genera código de vinculación y empareja un dispositivo
Write-Host "[5/7] Observador vincula dispositivo"
$codeResp = Invoke-Api -Method POST -Path "/api/v1/devices/tokens/generate" -Token $obsToken -Body @{ platform = "android" }
$code = $codeResp.data.code
Write-Host "      pairing code: $code"

$device = Invoke-Api -Method POST -Path "/api/v1/devices/pair" -Token $obsToken -Body @{
    pairing_code = $code; device_model = "Pixel 7 test"; device_os = "Android 14"; serial = "SN-TEST-1"; platform = "android"
}
$deviceID = $device.data.id
Write-Host "      device id: $deviceID"

# 5. Registrar FCM token (dummy: dry-run omite el envío real)
Write-Host "[6/7] Registrando FCM token dummy"
$null = Invoke-Api -Method POST -Path "/api/v1/devices/fcm-token" -Token $obsToken -Body @{
    device_id = $deviceID; fcm_token = "dummy-fcm-token-${suffix}"; platform = "android"
}

# 6. Víctima dispara SOS
Write-Host "[7/7] Víctima dispara POST /api/v1/alerts/sos"
$alert = Invoke-Api -Method POST -Path "/api/v1/alerts/sos" -Token $victimToken -Body @{
    type = "sos"; latitude = 19.4326; longitude = -99.1332; trigger_source = "wearable_test"
}
Write-Host "      alert_id: $($alert.data.alert_id)"
Write-Host "      status:   $($alert.data.status)"

# 7. Verificaciones
Write-Host "`n=== VERIFICACIONES ===" -ForegroundColor Green
$obsAlerts = Invoke-Api -Method GET -Path "/api/v1/alerts/observing" -Token $obsToken
Write-Host "Observador ve emergencias activas: $($obsAlerts.data.Count)"

$alertDetail = Invoke-Api -Method GET -Path "/api/v1/alerts/$($alert.data.alert_id)" -Token $obsToken
Write-Host "Observador ve detalle de la alerta: $($alertDetail.data.alert_id)"
Write-Host "      lat/lng: $($alertDetail.data.latitude), $($alertDetail.data.longitude)"

Write-Host "`n=== RESULTADO ===" -ForegroundColor Cyan
if ($obsAlerts.data.Count -ge 1) {
    Write-Host "FLUJO COMPLETO OK. Ahora revisa los logs del servidor:" -ForegroundColor Green
    Write-Host '  - "SOS: resolving observers" con linked_observers=1 y push_targets=1' -ForegroundColor Yellow
    Write-Host '  - "FCM credentials missing: Push skipped" (dry-run, sin credenciales)' -ForegroundColor Yellow
    Write-Host '  - Con FIREBASE_SERVICE_ACCOUNT real + token de app, verás la notificación en el teléfono.'
} else {
    Write-Host "El observador no recibió alertas. Revisa linked_user_id y el device." -ForegroundColor Red
}
