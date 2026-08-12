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

param(
    [string]$ProjectId = "eurotacho-parser",
    # Madrid: los TGD llevan datos personales de conductores (jornada, GNSS) y no
    # hay motivo para sacarlos de Espana. El parser no almacena, pero procesa.
    [string]$Region    = "europe-southwest1",
    [string]$Service   = "eurotacho-parser",
    [string]$Repo      = "containers",
    [switch]$RotarApiKey
)

$ErrorActionPreference = "Stop"

function Paso($texto) { Write-Host "`n=== $texto ===" -ForegroundColor Cyan }

# --- 0. Contexto -------------------------------------------------------------
Paso "Comprobando entorno"
gcloud config set project $ProjectId | Out-Null
$cuenta = gcloud config get-value account 2>$null
if (-not $cuenta -or $cuenta -eq "(unset)") {
    throw "gcloud no esta autenticado. Ejecuta: gcloud auth login"
}
Write-Host "Proyecto: $ProjectId   Cuenta: $cuenta   Region: $Region"

# La version identifica el binario exacto. --dirty marca un arbol con cambios sin
# commitear: es preferible desplegar algo etiquetado como sucio a desplegar algo
# que dice ser un commit limpio y no lo es.
$version = (git describe --tags --always --dirty).Trim()
if (-not $version) { $version = "desconocida" }
Write-Host "Version a desplegar: $version"

# --- 1. APIs necesarias ------------------------------------------------------
Paso "Habilitando APIs (idempotente)"
gcloud services enable `
    run.googleapis.com `
    cloudbuild.googleapis.com `
    artifactregistry.googleapis.com

# --- 2. Repositorio de imagenes ---------------------------------------------
Paso "Repositorio de Artifact Registry"
$existe = gcloud artifacts repositories describe $Repo --location=$Region 2>$null
if (-not $existe) {
    gcloud artifacts repositories create $Repo `
        --repository-format=docker `
        --location=$Region `
        --description="Imagenes del parser de EuroTacho"

    # Sin politica de limpieza, cada despliegue deja ~30 MB y el nivel gratuito
    # (0,5 GB) se agota en unas quince versiones sin que nadie se entere.
    $politica = @'
[
  {
    "name": "conservar-ultimas-5",
    "action": {"type": "Keep"},
    "mostRecentVersions": {"keepCount": 5}
  },
  {
    "name": "borrar-el-resto",
    "action": {"type": "Delete"},
    "condition": {"olderThan": "30d"}
  }
]
'@
    $tmp = New-TemporaryFile
    Set-Content -Path $tmp -Value $politica -Encoding utf8
    gcloud artifacts repositories set-cleanup-policies $Repo --location=$Region --policy=$tmp
    Remove-Item $tmp
} else {
    Write-Host "Ya existe; no se toca."
}

# --- 3. Clave de API ---------------------------------------------------------
Paso "Clave de API del parser"
$claveExistente = $null
try {
    $claveExistente = (gcloud run services describe $Service --region=$Region `
        --format="value(spec.template.spec.containers[0].env.filter(\"name:PARSER_API_KEY\").extract(\"value\"))" 2>$null)
} catch { }

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
$imagen = "$Region-docker.pkg.dev/$ProjectId/$Repo/$Service`:$version"
gcloud builds submit --config cloudbuild.yaml `
    --substitutions="_IMAGE=$imagen,_VERSION=$version"

# --- 5. Deploy ---------------------------------------------------------------
Paso "Desplegando en Cloud Run"
# --allow-unauthenticated: el control de acceso lo hace nuestra X-Api-Key, no IAM
#   de Google. La Edge Function corre en Deno y firmar tokens de servicio de
#   Google desde alli anadiria complejidad sin ganar mucho: la clave ya se
#   compara en tiempo constante y el servicio no expone nada sin ella.
# --max-instances 5: techo de gasto y de dano. Un bucle en el frontend no puede
#   escalar a cien instancias.
# --min-instances 0: escala a cero. El precio de esto es un arranque en frio de
#   1-2 s en la primera subida tras un rato inactivo.
# --memory 1Gi: un VU con velocidad detallada genera decenas de miles de puntos
#   y el JSON de salida se hincha; 512Mi se queda justo.
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

# --- 6. Comprobacion ---------------------------------------------------------
Paso "Verificando el despliegue"
$url = gcloud run services describe $Service --region=$Region --format="value(status.url)"

$health = Invoke-RestMethod -Uri "$url/api/v1/health" -TimeoutSec 60
Write-Host "health -> status=$($health.status)  parser_version=$($health.parser_version)"
if ($health.parser_version -ne $version) {
    Write-Warning "La version que reporta el servicio ($($health.parser_version)) no coincide con la desplegada ($version)."
}

# /parse sin clave debe rechazar: si esto devuelve 200, el parser esta abierto.
try {
    Invoke-WebRequest -Uri "$url/api/v1/parse" -Method POST -Body ([byte[]]@(0x76,0x00)) -TimeoutSec 60 | Out-Null
    Write-Warning "ATENCION: /parse respondio SIN clave. Revisa PARSER_API_KEY."
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 401) {
        Write-Host "/parse sin clave -> 401 (correcto)" -ForegroundColor Green
    } else {
        Write-Host "/parse sin clave -> $($_.Exception.Response.StatusCode.value__)"
    }
}

Paso "Listo"
Write-Host "Carga estos dos secretos en Supabase (Edge Functions):"
Write-Host "  PARSER_SERVICE_URL = $url"
Write-Host "  PARSER_API_KEY     = $apiKey"
