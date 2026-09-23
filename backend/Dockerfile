# Один образ на все процессы: команда запуска выбирается в compose.yaml.
FROM golang:1.25-alpine AS build
WORKDIR /src

# Зависимости отдельным слоем: при правке кода они не качаются заново.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

FROM alpine:3.22
# Сертификаты — для HTTPS к platform-api.max.ru.
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY testdata ./testdata
USER nobody
CMD ["bot"]
