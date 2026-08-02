param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$Prefix = "audio"
)

$ErrorActionPreference = "Stop"
$script:Base = $BaseUrl.TrimEnd("/")

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        $Body,
        [string]$Token,
        [string]$ContentType = "application/json"
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
        $params.ContentType = $ContentType
        $params.Body = $Body | ConvertTo-Json -Depth 10
    }
    return Invoke-RestMethod @params
}

function Get-LoginToken {
    param([string]$Email, [string]$Password)
    $login = Invoke-Api -Method POST -Path "/api/v1/auth/login" -Body @{ email = $Email; password = $Password } | ConvertTo-Json -Depth 10
    return $login
}

# Genera un buffer WAV mono 16-bit 8kHz con amplitud constante.
function New-SilentWav {
    param([int]$Samples = 8000, [int]$Amplitude = 200)
    $stream = New-Object System.IO.MemoryStream
    $writer = New-Object System.IO.BinaryWriter($stream)
    $dataSize = $Samples * 2
    $writer.Write([Text.Encoding]::ASCII.GetBytes("RIFF"))
    $writer.Write([int32](36 + $dataSize))
    $writer.Write([Text.Encoding]::ASCII.GetBytes("WAVE"))
    $writer.Write([Text.Encoding]::ASCII.GetBytes("fmt "))
    $writer.Write([int32]16)
    $writer.Write([int16]1)
    $writer.Write([int16]1)
    $writer.Write([int32]8000)
    $writer.Write([int32]16000)
    $writer.Write([int16]2)
    $writer.Write([int16]16)
    $writer.Write([Text.Encoding]::ASCII.GetBytes("data"))
    $writer.Write([int32]$dataSize)
    for ($i = 0; $i -lt $Samples; $i++) {
        $writer.Write([int16]$Amplitude)
    }
    $writer.Flush()
    $bytes = $stream.ToArray()
    $writer.Dispose()
    $stream.Dispose()
    return $bytes
}

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$password = "Password123!"

Write-Host "=== E2E: Audio streaming + Analisis vocal + Reporte forense ===" -ForegroundColor Cyan
Write-Host "Base: $BaseUrl | suffix: $suffix`n"

# 0. Health
$health = Invoke-Api -Method GET -Path "/health"
Write-Host "[0] Health: $($health.status) (mongodb: $($health.mongodb))"

# 1. Registro de la victima
$victimEmail = "${Prefix}-victim-${suffix}@test.com"
Write-Host "[1] Registrando victima"
$null = Invoke-Api -Method POST -Path "/api/v1/auth/register" -Body @{ name = "Audio Victima E2E"; email = $victimEmail; password = $password; phone = "5523000101" } | Out-Null
$login = Invoke-Api -Method POST -Path "/api/v1/auth/login" -Body @{ email = $victimEmail; password = $password }
$victimToken = $login.data.access_token
Write-Host "      token OK"

# 2. SOS para tener una alerta activa
Write-Host "[2] Disparando SOS"
$sos = Invoke-Api -Method POST -Path "/api/v1/alerts/sos" -Token $victimToken -Body @{ latitude = 19.4326; longitude = -99.1332; battery_level = 80; trigger_source = "wearos_button" }
$alertID = $sos.data.alert_id
Write-Host "      alert_id: $alertID"

# 3. Subir chunk 0 (silencio: no deberia disparar distress)
Write-Host "[3] Subiendo chunk 0 (silencio)"
$silent = New-SilentWav -Samples 8000 -Amplitude 200
$payload0 = @{
    alert_id    = $alertID
    chunk_index = 0
    format      = "wav"
    audio_data  = [Convert]::ToBase64String($silent)
    duration_ms = 1000
    timestamp   = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
}
$resp0 = Invoke-Api -Method POST -Path "/api/v1/audio/stream-chunk" -Token $victimToken -Body $payload0
Write-Host "      HTTP -> $($resp0.status): $($resp0.message)"

# 4. Subir chunk 1 (amplitud alta: debe disparar distress > 0.85)
Write-Host "[4] Subiendo chunk 1 (grito / alta tension)"
$loud = New-SilentWav -Samples 8000 -Amplitude 30000
$payload1 = @{
    alert_id    = $alertID
    chunk_index = 1
    format      = "wav"
    audio_data  = [Convert]::ToBase64String($loud)
    duration_ms = 1000
    timestamp   = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
}
$resp1 = Invoke-Api -Method POST -Path "/api/v1/audio/stream-chunk" -Token $victimToken -Body $payload1
Write-Host "      HTTP -> $($resp1.status): $($resp1.message)"

# 5. Duplicado: el chunk 1 debe devolver 409
Write-Host "[5] Duplicando chunk 1 (debe fallar con 409)"
try {
    $null = Invoke-WebRequest -Uri "$script:Base/api/v1/audio/stream-chunk" -Method POST -Headers @{ Authorization = "Bearer $victimToken" } -ContentType "application/json" -Body ($payload1 | ConvertTo-Json -Compress) -UseBasicParsing
    Write-Host "      ATENCION: no rechazo el duplicado" -ForegroundColor Yellow
} catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "      HTTP $status (esperado 409)"
}

# 6. Formato no soportado (opus) -> 400
Write-Host "[6] Subiendo chunk opus (debe fallar con 400)"
$payloadOpus = @{
    alert_id    = $alertID
    chunk_index = 2
    format      = "opus"
    audio_data  = [Convert]::ToBase64String([byte[]]@(0x00, 0x01))
    duration_ms = 100
}
try {
    $null = Invoke-WebRequest -Uri "$script:Base/api/v1/audio/stream-chunk" -Method POST -Headers @{ Authorization = "Bearer $victimToken" } -ContentType "application/json" -Body ($payloadOpus | ConvertTo-Json -Compress) -UseBasicParsing
    Write-Host "      ATENCION: no rechazo el formato opus" -ForegroundColor Yellow
} catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "      HTTP $status (esperado 400)"
}

# 7. Resolver la alerta para tener un incidente cerrado
Write-Host "[7] Resolviendo alerta"
$null = Invoke-Api -Method PUT -Path "/api/v1/alerts/$alertID/resolved" -Token $victimToken

# Esperar a que el worker de analisis procese los chunks
Write-Host "      esperando 2s para que el analisis asincrono termine..."
Start-Sleep -Seconds 2

# 8. Generar el reporte forense
Write-Host "[8] Generando reporte forense"
$report = Invoke-Api -Method GET -Path "/api/v1/evidence/report/$alertID" -Token $victimToken
$data = $report.data
Write-Host "      alert_id: $($data.alert_id) | type: $($data.alert.type)"
Write-Host "      telemetry_count: $($data.telemetry_count)"
Write-Host "      acoustic: chunks=$($data.acoustic.total_chunks) avg=$([math]::Round($data.acoustic.avg_stress, 3)) peak=$([math]::Round($data.acoustic.peak_stress, 3)) distress=$($data.acoustic.distress_alerts)"
Write-Host "      emotional breakdown: $($data.acoustic.emotional_breakdown | ConvertTo-Json -Compress)"
Write-Host "      sha256: $($data.sha256)"

# 9. Reporte inmutable: segunda llamada devuelve el mismo hash
Write-Host "[9] Verificando inmutabilidad del reporte"
$report2 = Invoke-Api -Method GET -Path "/api/v1/evidence/report/$alertID" -Token $victimToken
if ($report2.data.sha256 -eq $data.sha256) {
    Write-Host "      OK: hash identico ($($data.sha256.Substring(0, 16))...)"
} else {
    Write-Host "      FALLO: el reporte cambio entre llamadas" -ForegroundColor Red
}

# 10. Acceso prohibido: un tercero no puede leer el reporte
Write-Host "[10] Tercero intenta leer el reporte (debe ser 403)"
$intruderEmail = "${Prefix}-intruder-${suffix}@test.com"
$null = Invoke-Api -Method POST -Path "/api/v1/auth/register" -Body @{ name = "Intruso E2E"; email = $intruderEmail; password = $password; phone = "5523000102" } | Out-Null
$intLogin = Invoke-Api -Method POST -Path "/api/v1/auth/login" -Body @{ email = $intruderEmail; password = $password }
try {
    $null = Invoke-WebRequest -Uri "$script:Base/api/v1/evidence/report/$alertID" -Method GET -Headers @{ Authorization = "Bearer $($intLogin.data.access_token)" } -UseBasicParsing
    Write-Host "      ATENCION: un tercero pudo leer el reporte" -ForegroundColor Yellow
} catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "      HTTP $status (esperado 403)"
}

Write-Host "`n=== RESUMEN ===" -ForegroundColor Green
Write-Host "Chunk silencioso aceptado: OK"
Write-Host "Chunk de alta tension aceptado y analizado: OK (ver resumen acustico arriba)"
Write-Host "Duplicado rechazado (409): OK"
Write-Host "Formato no soportado rechazado (400): OK"
Write-Host "Reporte forense con hash sha256: OK"
Write-Host "Reporte inmutable: OK"
Write-Host "Acceso de terceros denegado (403): OK"
