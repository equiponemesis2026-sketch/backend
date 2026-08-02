param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$VictimEmail,
    [string]$ObserverEmail,
    [string]$Password = "Password123!",
    [string]$AlertId
)

$ErrorActionPreference = "Stop"

function Get-LoginToken {
    param([string]$Email, [string]$Password)
    $login = Invoke-RestMethod -Method POST -Uri "$BaseUrl/api/v1/auth/login" -ContentType "application/json" -Body (@{ email = $Email; password = $Password } | ConvertTo-Json)
    return $login.data.access_token
}

if (-not $VictimEmail -or -not $ObserverEmail -or -not $AlertId) {
    Write-Host "Uso: test-ws.ps1 -VictimEmail <email> -ObserverEmail <email> -AlertId <alt_id>" -ForegroundColor Red
    exit 1
}

$victimToken = Get-LoginToken $VictimEmail $Password
$obsToken    = Get-LoginToken $ObserverEmail $Password

$wsBase = $BaseUrl -replace '^http', 'ws'

Write-Host "=== Test WebSocket Telemetria ===" -ForegroundColor Cyan
Write-Host "Alert: $AlertId"

# Receptor (observador) se conecta primero usando Authorization header
$receiver = New-Object System.Net.WebSockets.ClientWebSocket
$recvUri = [Uri]("$wsBase/ws/v1/alerts/stream")
$receiver.Options.SetRequestHeader("Authorization", "Bearer $obsToken")
Write-Host "[1] Conectando receptor (observador) via header..."
$receiver.ConnectAsync($recvUri, [Threading.CancellationToken]::None).GetAwaiter().GetResult()
Write-Host "      Estado receptor: $($receiver.State)"

Start-Sleep -Milliseconds 500

# Emisor (victima) se conecta via query token (fallback) y envia telemetria
$emitter = New-Object System.Net.WebSockets.ClientWebSocket
$emitUri = [Uri]("$wsBase/ws/v1/alerts/stream?token=$victimToken")
Write-Host "[2] Conectando emisor (victima) via query fallback..."
$emitter.ConnectAsync($emitUri, [Threading.CancellationToken]::None).GetAwaiter().GetResult()
Write-Host "      Estado emisor: $($emitter.State)"

Start-Sleep -Milliseconds 500

# Enviar paquete de telemetria
$packet = @{
    alert_id      = $AlertId
    latitude      = 19.4326
    longitude     = -99.1332
    speed         = 15.5
    battery_level = 72
    timestamp     = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
} | ConvertTo-Json -Compress
$bytes = [System.Text.Encoding]::UTF8.GetBytes($packet)
$seg = [ArraySegment[byte]]::new($bytes)
Write-Host "[3] Emisor envia telemetria: $packet"
$emitter.SendAsync($seg, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, [Threading.CancellationToken]::None).GetAwaiter().GetResult()

# Receptor recibe
$buffer = New-Object byte[] 1024
$recvSeg = [ArraySegment[byte]]::new($buffer)
Write-Host "[4] Esperando que el receptor reciba..."
$recvResult = $receiver.ReceiveAsync($recvSeg, [Threading.CancellationToken]::None).GetAwaiter().GetResult()
$msg = [System.Text.Encoding]::UTF8.GetString($recvSeg.Array, 0, $recvResult.Count)
Write-Host "      Receptor recibio: $msg"

# Verificar
if ($msg.Contains($AlertId)) {
    Write-Host "`n=== WS TELEMETRIA OK: el observador recibio el paquete GPS ===`n" -ForegroundColor Green
} else {
    Write-Host "`n=== FALLO: el receptor no recibio telemetria correcta ===`n" -ForegroundColor Red
}

# Cerrar
$receiver.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "bye", [Threading.CancellationToken]::None).GetAwaiter().GetResult()
$emitter.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "bye", [Threading.CancellationToken]::None).GetAwaiter().GetResult()
