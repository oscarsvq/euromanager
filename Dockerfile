# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Etapa 1 — compila eurotacho-api (API: /api/v1/parse + /api/v1/health).
#
# Los certificados ERCA (pks1 = Gen1, pks2 = Gen2/v2) estan VERSIONADOS en el
# repo desde 15b9b91 y se embeben con go:embed. Antes habia una etapa previa que
# los descargaba del JRC con scripts Python en CADA build y los copiaba encima:
# eso hacia que la cadena de confianza con la que se validan firmas periciales
# dependiera de lo que hubiera en un servidor externo el dia del build, y no de
# lo auditado en el repo. Los scripts siguen en scripts/pks{1,2} para
# actualizarlos a mano de forma deliberada, no como paso de build.
#
# Imagenes fijadas por digest: un tag flotante puede cambiar de contenido entre
# dos builds del mismo commit.
# ---------------------------------------------------------------------------
FROM golang:1.26.5-bookworm@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd AS gobuilder

WORKDIR /src

# Descarga de dependencias en su propia capa: solo se rehace si cambian go.mod
# o go.sum, no en cada cambio de codigo.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# VERSION se graba en analysis_runs.parser_version y forma parte de la cadena de
# custodia: identifica el binario exacto que produjo cada analisis. Pasarla con:
#   --build-arg VERSION=$(git describe --tags --always --dirty)
# Si no se pasa, el binario declara "desconocida" en vez de inventarse un numero.
ARG VERSION=desconocida

RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X github.com/traconiq/tachoparser/internal/api.buildVersion=${VERSION}" \
      -o /eurotacho-api ./cmd/eurotacho-api

# ---------------------------------------------------------------------------
# Etapa 2 — imagen minima: sin shell ni gestor de paquetes, usuario no root.
# eurotacho-api escucha en $PORT (Cloud Run lo inyecta; fallback 8080).
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a

COPY --from=gobuilder /eurotacho-api /eurotacho-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/eurotacho-api"]
