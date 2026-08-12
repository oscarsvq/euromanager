# Despliegue del parser a Google Cloud Run.
#
# El despliegue vive aqui como codigo y no como una lista de comandos en un chat:
# la version del parser se graba en analysis_runs.parser_version y forma parte de
# la cadena de custodia, asi que tiene que poder repetirse igual dentro de un ano.
#
# Requisitos previos (una sola vez):
#   - Cuenta de Google Cloud con facturacion vinculada al proyecto.
#   - gcloud instalado y autenticado:  gcloud auth login
#
# Uso:
#   .\scripts\deploy-cloudrun.ps1                      # despliega
#   .\scripts\deploy-cloudrun.ps1 -RotarApiKey         # ademas genera clave nueva
#
# Tras desplegar hay que cargar en Supabase los secretos PARSER_SERVICE_URL y
# PARSER_API_KEY que el script imprime al final.
#
# NOTA sobre el manejo de errores: NO se usa $ErrorActionPreference='Stop'. En
# PowerShell 5.1 eso convierte cualquier linea de stderr de un ejecutable nativo
# en excepcion terminante, y gcloud escribe en stderr cosas que no son fallos
# (por ejemplo el NOT_FOUND de "¿existe ya este repositorio?", que la primera vez
# es la respuesta correcta). Se comprueba $LASTEXITCODE explicitamente.

param(
    [string]$ProjectId = "eurotacho-parser",
    # Madrid: los TGD llevan datos personales de conductores (jornada, GNSS) y no
    # hay motivo para sacarlos de Espana. El parser no almacena, pero procesa.
    [string]$Region    = "europe-southwest1",
    [string]$Service   = "eurotacho-parser",
    [string]$Repo      = "containers",
    [switch]$RotarApiKey
)

$ErrorActionPreference = "Continue"

function Paso($texto) { Write-Host "`n=== $texto ===" -ForegroundColor Cyan }

# Ejecuta un paso que DEBE salir bien; aborta el despliegue si no.
function Exigir($descripcion, [scriptblock]$bloque) {
    & $bloque
    if ($LASTEXITCODE -ne 0) { throw "$descripcion fallo (exit $LASTEXITCODE)" }
}

# Consulta cuyo fallo es informacion, no error (p. ej. "¿existe ya?").
function Consultar([scriptblock]$bloque) {
    $salida = (& $bloque 2>&1 | Out-String).Trim()
    return [pscustomobject]@{ Ok = ($LASTEXITCODE -eq 0); Salida = $salida }
}

# --- 0. Contexto -------------------------------------------------------------
Paso "Comprobando entorno"
Exigir "gcloud config set project" { gcloud config set project $ProjectId 2>&1 | Out-Null }

$cuenta = (gcloud config get-value account 2>$null | Out-String).Trim()
if (-not $cuenta -or $cuenta -eq "(unset)") {
    throw "gcloud no esta autenticado. Ejecuta: gcloud auth login"
}
Write-Host "Proyecto: $ProjectId   Cuenta: $cuenta   Region: $Region"

# La version identifica el binario exacto. --dirty marca un arbol con cambios sin
# commitear: es preferible desplegar algo etiquetado como sucio a desplegar algo
# que dice ser un commit limpio y no lo es.
$version = (git describe --tags --always --dirty | Out-String).Trim()
if (-not $version) { $version = "desconocida" }
if ($version -like "*dirty*") {
    Write-Warning "El arbol tiene cambios sin commitear: la imagen no correspondera a ningun commit."
}
Write-Host "Version a desplegar: $version"

# --- 1. APIs necesarias ------------------------------------------------------
Paso "Habilitando APIs (idempotente)"
Exigir "habilitar APIs" {
    gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com
}

# --- 2. Repositorio de imagenes ---------------------------------------------
Paso "Repositorio de Artifact Registry"
$existentes = Consultar { gcloud artifacts repositories list --location=$Region --format="value(name)" }
if ($existentes.Salida -match "(?m)(^|/)$([regex]::Escape($Repo))\s*$") {
    Write-Host "Ya existe; no se crea."
} else {
    Exigir "crear repositorio" {
        gcloud artifacts repositories create $Repo `
            --repository-format=docker `
            --location=$Region `
            --description="Imagenes del parser de EuroTacho"
    }
}

# La politica se (re)aplica SIEMPRE, no solo al crear el repositorio: si falla el
# dia de la creacion —paso la primera vez, por el BOM— el repositorio se queda
# sin limpieza para siempre y el coste crece en silencio.
if ($true) {
    # Sin politica de limpieza, cada despliegue deja ~30 MB y el nivel gratuito
    # (0,5 GB) se agota en unas quince versiones sin que nadie se entere.
    $politica = @'
[
  {
    "name": "conservar-recientes",
    "action": {"type": "Keep"},
    "mostRecentVersions": {"keepCount": 5}
  },
  {
    "name": "borrar-antiguas",
    "action": {"type": "Delete"},
    "condition": {"olderThan": "30d"}
  }
]
'@
    # WriteAllText escribe UTF-8 SIN BOM. Set-Content -Encoding utf8 en
    # PowerShell 5.1 lo escribe CON BOM y gcloud rechaza el JSON con
    # "Unexpected UTF-8 BOM (decode using utf-8-sig)".
    $tmp = [System.IO.Path]::GetTempFileName()
    [System.IO.File]::WriteAllText($tmp, $politica)
    $pol = Consultar {
        gcloud artifacts repositories set-cleanup-policies $Repo --location=$Region --policy=$tmp
    }
    Remove-Item $tmp -Force
    if (-not $pol.Ok) {
        # No aborta: el despliegue es valido igualmente, pero hay que saberlo
        # porque el coste crece en silencio.
        Write-Warning "No se pudo aplicar la politica de limpieza. Revisa el consumo de Artifact Registry a mano.`n$($pol.Salida)"
    } else {
        Write-Host "Politica de limpieza aplicada (5 versiones recientes)."
    }
}

# --- 3. Clave de API ---------------------------------------------------------
Paso "Clave de API del parser"
$actual = Consultar {
    gcloud run services describe $Service --region=$Region `
        --format="value(spec.template.spec.containers[0].env)"
}
$claveExistente = $null
if ($actual.Ok -and $actual.Salida -match 'PARSER_API_KEY[^A-Za-z0-9_-]+([A-Za-z0-9_-]{20,})') {
    $claveExistente = $Matches[1]
}

if ($RotarApiKey -or -not $claveExistente) {
    # 32 bytes aleatorios en base64url. Generada aqui y nunca versionada.
    $bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $apiKey = [Convert]::ToBase64String($bytes).Replace('+','-').Replace('/','_').TrimEnd('=')
    Write-Host "Clave NUEVA generada (guardala: no vuelve a mostrarse)." -ForegroundColor Yellow
} else {
    $apiKey = $claveExistente
    Write-Host "Reutilizando la clave ya configurada en el servicio."
}

# --- 4. Build ----------------------------------------------------------------
Paso "Construyendo la imagen (Cloud Build, sin Docker local)"
$imagen = "$Region-docker.pkg.dev/$ProjectId/$Repo/$Service" + ":" + $version
Exigir "build" {
    gcloud builds submit --config cloudbuild.yaml --substitutions="_IMAGE=$imagen,_VERSION=$version"
}

# --- 5. Deploy ---------------------------------------------------------------
Paso "Desplegando en Cloud Run"
# --allow-unauthenticated: el control de acceso lo hace nuestra X-Api-Key, no IAM
#   de Google. La Edge Function corre en Deno y firmar tokens de servicio desde
#   alli anadiria complejidad sin ganar mucho: la clave ya se compara en tiempo
#   constante y el servicio no expone nada sin ella.
# --max-instances 5: techo de gasto y de dano. Un bucle en el frontend no puede
#   escalar a cien instancias.
# --min-instances 0: escala a cero. El precio es un arranque en frio de 1-2 s en
#   la primera subida tras un rato inactivo.
# --memory 1Gi: un VU con velocidad detallada genera decenas de miles de puntos
#   y el JSON de salida se hincha; 512Mi se queda justo.
Exigir "deploy" {
    gcloud run deploy $Service `
        --image=$imagen `
        --region=$Region `
        --platform=managed `
        --allow-unauthenticated `
        --memory=1Gi `
        --cpu=1 `
        --min-instances=0 `
        --max-instances=5 `
        --concurrency=4 `
        --timeout=300 `
        --set-env-vars="PARSER_API_KEY=$apiKey" `
        --labels="app=eurotacho,componente=parser"
}

# --- 6. Comprobacion ---------------------------------------------------------
Paso "Verificando el despliegue"
$url = (gcloud run services describe $Service --region=$Region --format="value(status.url)" | Out-String).Trim()
Write-Host "URL: $url"

$health = Invoke-RestMethod -Uri "$url/api/v1/health" -TimeoutSec 90
Write-Host "health -> status=$($health.status)  parser_version=$($health.parser_version)"
if ($health.parser_version -ne $version) {
    Write-Warning "La version que reporta el servicio ($($health.parser_version)) no coincide con la desplegada ($version)."
}

# /parse sin clave debe rechazar: si esto devuelve 200, el parser esta abierto.
$abierto = $false
try {
    Invoke-WebRequest -Uri "$url/api/v1/parse" -Method POST -Body ([byte[]]@(0x76,0x00)) -TimeoutSec 90 | Out-Null
    $abierto = $true
} catch {
    $codigo = $_.Exception.Response.StatusCode.value__
    if ($codigo -eq 401) {
        Write-Host "/parse sin clave -> 401 (correcto)" -ForegroundColor Green
    } else {
        Write-Warning "/parse sin clave -> $codigo (esperaba 401)"
    }
}
if ($abierto) { Write-Warning "ATENCION: /parse respondio SIN clave. El parser esta abierto: revisa PARSER_API_KEY." }

Paso "Listo"
Write-Host "Carga estos dos secretos en Supabase (Edge Functions):"
Write-Host "  PARSER_SERVICE_URL = $url"
Write-Host "  PARSER_API_KEY     = $apiKey"
