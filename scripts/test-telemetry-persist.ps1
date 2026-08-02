param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$Prefix = "telemetry"
)

$ErrorActionPreference = "Stop"
$script:Base = $BaseUrl.TrimEnd("/")

function Api {
    param($Method, $Path, $Body, $Token)
    $h = @{}
    if ($Token) { $h["Authorization"] = "Bearer $Token" }
    $p = @{ Method = $Method; Uri = "$script:Base$Path"; Headers = $h }
    if ($null -ne $Body) {
        $p.ContentType = "application/json"
        $p.Body = $Body | ConvertTo-Json -Depth 10
    }
    Invoke-RestMethod @p
}

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$pw = "Password123!"
$vEmail = "${Prefix}-victim-${suffix}@test.com"
$oEmail = "${Prefix}-obs-${suffix}@test.com"

Write-Host "=== E2E: Persistencia de telemetria (WS -> telemetry_logs -> reporte) ===" -ForegroundColor Cyan

# 1. Registros
$null = Api POST "/api/v1/auth/register" @{ name = "Tel Victima"; email = $vEmail; password = $pw; phone = "5524000101" }
$null = Api POST "/api/v1/auth/register" @{ name = "Tel Obs"; email = $oEmail; password = $pw; phone = "5524000102" }
$vTok = (Api POST "/api/v1/auth/login" @{ email = $vEmail; password = $pw }).data.access_token
$oTok = (Api POST "/api/v1/auth/login" @{ email = $oEmail; password = $pw }).data.access_token
Write-Host "[1] Usuarios registrados"

# 2. Victima crea contacto -> observador acepta
$contact = Api POST "/api/v1/contacts/" -Body @{ name = "Apoyo"; phone = "5524000102"; email = $oEmail; relationship = "familiar" } -Token $vTok
$null = Api POST "/api/v1/contacts/$($contact.data.contact_id)/accept" -Token $oTok
Write-Host "[2] Contacto aceptado (IsVerified)"

# 3. SOS activo
$sos = Api POST "/api/v1/alerts/sos" -Body @{ latitude = 19.4326; longitude = -99.1332; trigger_source = "tel_test" } -Token $vTok
$alertID = $sos.data.alert_id
Write-Host "[3] SOS activo: $alertID"

# 4. Emisor conecta por WS y envia telemetria
$wsBase = $script:Base -replace "^http", "ws"
$emitter = New-Object System.Net.WebSockets.ClientWebSocket
$emitUri = [Uri]("$wsBase/ws/v1/alerts/stream?token=$vTok")
$emitter.ConnectAsync($emitUri, [Threading.CancellationToken]::None).GetAwaiter().GetResult() | Out-Null

for ($i = 1; $i -le 3; $i++) {
    $packet = @{
        alert_id      = $alertID
        latitude      = 19.4 + ($i * 0.01)
        longitude     = -99.13 + ($i * 0.01)
        speed         = 10 + $i
        battery_level = 90 - $i
        timestamp     = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    } | ConvertTo-Json -Compress
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($packet)
    $seg = [ArraySegment[byte]]::new($bytes)
    $emitter.SendAsync($seg, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, [Threading.CancellationToken]::None).GetAwaiter().GetResult() | Out-Null
    Start-Sleep -Milliseconds 300
}
Write-Host "[4] Emisor envio 3 paquetes de telemetria por WS"

$emitter.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "bye", [Threading.CancellationToken]::None).GetAwaiter().GetResult() | Out-Null

# 5. Resolver y esperar persistencia
$null = Api PUT "/api/v1/alerts/$alertID/resolved" -Token $vTok
Start-Sleep -Seconds 2

# 6. Reporte forense debe incluir la telemetria persistida
$report = Api GET "/api/v1/evidence/report/$alertID" -Token $vTok
$data = $report.data
Write-Host "[5] Reporte forense:"
Write-Host "      telemetry_count: $($data.telemetry_count)"
Write-Host "      telemetry_start: $($data.telemetry_start) | telemetry_end: $($data.telemetry_end)"
if ($data.path) {
    Write-Host "      path points: $($data.path.Count) | primero: lat=$($data.path[0].latitude) lng=$($data.path[0].longitude)"
}

if ($data.telemetry_count -ge 3 -and $data.path.Count -ge 3) {
    Write-Host "`n=== TELEMETRIA PERSISTIDA OK: 3 paquetes en el historial del reporte ===" -ForegroundColor Green
} else {
    Write-Host "`n=== FALLO: telemetria no persistida en el reporte ===" -ForegroundColor Red
}
