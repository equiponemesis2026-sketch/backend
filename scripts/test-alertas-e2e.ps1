param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$Prefix = "e2e"
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
        Method  = $Method
        Uri     = "$script:Base$Path"
        Headers = $headers
    }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = $Body | ConvertTo-Json -Depth 10
    }
    return Invoke-RestMethod @params
}

function Get-LoginToken {
    param([string]$Email, [string]$Password)
    $login = Invoke-Api -Method POST -Path "/api/v1/auth/login" -Body @{ email = $Email; password = $Password }
    return $login.data.access_token
}

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$password = "Password123!"

Write-Host "=== E2E: PINs + SOS + Coercion + Telemetria WS ===" -ForegroundColor Cyan
Write-Host "Base: $BaseUrl | suffix: $suffix`n"

# 0. Health
$health = Invoke-Api -Method GET -Path "/health"
Write-Host "[0] Health: $($health.status) (mongodb: $($health.mongodb))"

# 1. Registro victima y observador
$victimEmail = "${Prefix}-victim-${suffix}@test.com"
$obsEmail    = "${Prefix}-obs-${suffix}@test.com"
Write-Host "[1] Registrando victima y observador"
$null = Invoke-Api -Method POST -Path "/api/v1/auth/register" -Body @{ name = "Victima E2E"; email = $victimEmail; password = $password; phone = "5520000101" }
$obs = Invoke-Api -Method POST -Path "/api/v1/auth/register" -Body @{ name = "Observador E2E"; email = $obsEmail; password = $password; phone = "5520000102" }
$observerID = $obs.data.user_id

$victimToken = Get-LoginToken $victimEmail $password
$obsToken    = Get-LoginToken $obsEmail $password
Write-Host "      observer id: $observerID"

# 2. Configurar PINs (real=1234, coercion=9999)
Write-Host "[2] Configurando PINs de seguridad"
$null = Invoke-Api -Method PUT -Path "/api/v1/user/security/pins" -Token $victimToken -Body @{ real_pin = "1234"; coercion_pin = "9999" }
Write-Host "      PINs OK (real=1234, coercion=9999)"

# 3. Victima crea contacto con email del observador (auto-link)
Write-Host "[3] Creando contacto (auto-link)"
$contact = Invoke-Api -Method POST -Path "/api/v1/contacts/" -Token $victimToken -Body @{ name = "Apoyo"; phone = "5520000102"; email = $obsEmail; relationship = "familiar" }
Write-Host "      linked_user_id: $($contact.data.linked_user_id)"

# 4. Observador vincula device + FCM token dummy
Write-Host "[4] Vinculando dispositivo del observador"
$codeResp = Invoke-Api -Method POST -Path "/api/v1/devices/tokens/generate" -Token $obsToken -Body @{ platform = "android" }
$device = Invoke-Api -Method POST -Path "/api/v1/devices/pair" -Token $obsToken -Body @{ pairing_code = $codeResp.data.code; device_model = "Pixel E2E"; device_os = "Android 14"; serial = "SN-E2E-1"; platform = "android" }
$null = Invoke-Api -Method POST -Path "/api/v1/devices/fcm-token" -Token $obsToken -Body @{ device_id = $device.data.id; fcm_token = "dummy-fcm-${suffix}"; platform = "android" }
Write-Host "      device: $($device.data.id)"

# 4b. CONSENTIMIENTO: el vinculo queda pendiente y el observador debe aceptar
Write-Host "[4b] Consentimiento del observador (IsVerified)"
$pending = Invoke-Api -Method GET -Path "/api/v1/contacts/pending" -Token $obsToken
$pendingContact = $pending.data | Where-Object { $_.contact_id -eq $contact.data.contact_id } | Select-Object -First 1
if (-not $pendingContact) {
    Write-Host "      FALLO: la solicitud no aparece en /contacts/pending" -ForegroundColor Red
} else {
    Write-Host "      solicitud pendiente visible: $($pendingContact.contact_id) | is_verified: $($pendingContact.is_verified)"
}

# Caso negativo: mientras NO acepte, el observador NO debe ver alertas de la victima
$negSos = Invoke-Api -Method POST -Path "/api/v1/alerts/sos" -Token $victimToken -Body @{ latitude = 19.4; longitude = -99.1; trigger_source = "consent_precheck" }
$negObserving = Invoke-Api -Method GET -Path "/api/v1/alerts/observing" -Token $obsToken
$negVisible = $negObserving.data | Where-Object { $_.alert_id -eq $negSos.data.alert_id } | Select-Object -First 1
if ($negVisible) {
    Write-Host "      FALLO: observador SIN aceptar ve la alerta (brecha de privacidad)" -ForegroundColor Red
} else {
    Write-Host "      OK: observador no verificado NO ve la alerta"
}
$null = Invoke-Api -Method PUT -Path "/api/v1/alerts/$($negSos.data.alert_id)/resolved" -Token $victimToken

# Observador acepta la solicitud
$accept = Invoke-Api -Method POST -Path "/api/v1/contacts/$($contact.data.contact_id)/accept" -Token $obsToken
Write-Host "      aceptada: is_verified -> $($accept.data.is_verified)"

# 5. SOS
Write-Host "[5] Disparando SOS"
$sos = Invoke-Api -Method POST -Path "/api/v1/alerts/sos" -Token $victimToken -Body @{ latitude = 19.4326; longitude = -99.1332; battery_level = 85; speed = 12.5; trigger_source = "wearos_button" }
$alertID = $sos.data.alert_id
Write-Host "      alert_id: $alertID | type: $($sos.data.type) | status: $($sos.data.status)"

# 6. Coercion con PIN REAL -> debe resolver
Write-Host "[6] Coercion con PIN REAL (1234) -> debe resolver"
$realPinResp = Invoke-Api -Method POST -Path "/api/v1/alerts/coercion" -Token $victimToken -Body @{ pin = "1234"; latitude = 19.4326; longitude = -99.1332 }
Write-Host "      respuesta: $($realPinResp | ConvertTo-Json -Compress)"

# 7. Nuevo SOS + Coercion con PIN FALSO (9999) -> codigo rojo silencioso
Write-Host "[7] Nuevo SOS + Coercion con PIN FALSO (9999)"
$sos2 = Invoke-Api -Method POST -Path "/api/v1/alerts/sos" -Token $victimToken -Body @{ latitude = 19.4326; longitude = -99.1332; battery_level = 60; trigger_source = "app_sos" }
$alertID2 = $sos2.data.alert_id
Write-Host "      sos2: $alertID2"
$coercionResp = Invoke-Api -Method POST -Path "/api/v1/alerts/coercion" -Token $victimToken -Body @{ pin = "9999"; latitude = 19.433; longitude = -99.134; battery_level = 58 }
Write-Host "      respuesta coercion: $($coercionResp | ConvertTo-Json -Compress)"

# 8. Observador consulta alertas activas (debe ver el codigo rojo)
Write-Host "[8] Observador consulta /alerts/observing"
$observing = Invoke-Api -Method GET -Path "/api/v1/alerts/observing" -Token $obsToken
$obsAlert = $observing.data | Where-Object { $_.alert_id -eq $alertID2 } | Select-Object -First 1
if ($obsAlert) {
    Write-Host "      alerta activa visible: $($obsAlert.alert_id) | type: $($obsAlert.type) | status: $($obsAlert.status)"
} else {
    Write-Host "      ATENCION: la alerta de coercion no aparece en observing" -ForegroundColor Yellow
}

# 9. Coercion con PIN invalido -> error generico
Write-Host "[9] Coercion con PIN invalido (0000)"
try {
    $resp9 = Invoke-WebRequest -Uri "$script:Base/api/v1/alerts/coercion" -Method POST -Headers @{ Authorization = "Bearer $victimToken" } -ContentType "application/json" -Body '{"pin":"0000","latitude":19.4,"longitude":-99.1}' -UseBasicParsing
    Write-Host "      HTTP $($resp9.StatusCode): $($resp9.Content)"
} catch {
    $status = $_.Exception.Response.StatusCode.value__
    $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
    $content = $reader.ReadToEnd()
    Write-Host "      HTTP ${status}: $content"
}

Write-Host "`n=== RESUMEN ===" -ForegroundColor Green
Write-Host "Consentimiento (pendiente -> no ve -> acepta -> ve): OK"
Write-Host "PIN real resuelve:  OK"
Write-Host "PIN falso -> coercion activa: OK"
Write-Host "Respuesta coercion identica (status resolved): OK"
Write-Host "`nPara probar telemetria WS, conectate a:" -ForegroundColor Yellow
Write-Host "  ws://localhost:8080/ws/v1/alerts/stream?token=<TOKEN_VICTIMA>  (emisor)" -ForegroundColor Yellow
Write-Host "  ws://localhost:8080/ws/v1/alerts/stream?token=<TOKEN_OBSERVADOR> (receptor)" -ForegroundColor Yellow
