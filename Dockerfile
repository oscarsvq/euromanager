# syntax=docker/dockerfile:1

# Etapa 1 — descarga las cadenas de certificados ERCA (pks1 = Gen1, pks2 = Gen2/v2)
# desde el JRC. Quedan en internal/pkg/certificates/pks{1,2}/ para el go:embed.
FROM python:3.10-slim-buster AS pythonbuilder
ENV PYTHONUNBUFFERED 1
RUN pip install requests lxml
RUN mkdir /scripts /internal
COPY ./scripts/ /scripts/
COPY ./internal/ /internal/
WORKDIR /scripts/pks1
RUN ./dl_all_pks1.py
WORKDIR /scripts/pks2
RUN ./dl_all_pks2.py

# Etapa 2 — compila eurotacho-api (nuestro API: /api/v1/parse + /api/v1/health).
# Binario estatico (sin CGO) con los certs ERCA embebidos via go:embed.
FROM golang:1.19 AS gobuilder
WORKDIR /src
COPY ./ ./
COPY --from=pythonbuilder /internal/pkg/certificates/pks1/ internal/pkg/certificates/pks1/
COPY --from=pythonbuilder /internal/pkg/certificates/pks2/ internal/pkg/certificates/pks2/
RUN go mod vendor
RUN CGO_ENABLED=0 GOOS=linux go build -o /eurotacho-api ./cmd/eurotacho-api

# Etapa 3 — imagen minima. Binario estatico + ca-certs/tzdata de distroless.
# eurotacho-api escucha en $PORT (Railway/Fly/Cloud Run lo inyectan; fallback 8080).
FROM gcr.io/distroless/static-debian12
COPY --from=gobuilder /eurotacho-api /eurotacho-api
EXPOSE 8080
ENTRYPOINT ["/eurotacho-api"]
