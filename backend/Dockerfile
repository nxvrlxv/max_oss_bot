# Один образ на все процессы: команда запуска выбирается в compose.yaml.
FROM golang:1.25-alpine AS build
WORKDIR /src

# Зависимости отдельным слоем: при правке кода они не качаются заново.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

FROM alpine:3.22
# Сертификаты — для HTTPS к API MAX. platform-api2.max.ru подписан
# корневым УЦ Минцифры, которого нет в стандартном наборе Alpine.
RUN apk add --no-cache ca-certificates
COPY certs/russian_trusted_root_ca.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY testdata ./testdata
USER nobody
CMD ["bot"]
