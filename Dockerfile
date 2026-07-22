FROM golang:1.25 AS modules

WORKDIR /modules

COPY go.mod go.sum ./
RUN go mod download

FROM golang:1.25 AS builder

WORKDIR /app

COPY --from=modules /go/pkg /go/pkg

COPY . .

RUN PWGO_MOD_DIR=$(go list -f '{{.Dir}}' -m github.com/playwright-community/playwright-go) \
    && awk -F'"' '/const playwrightCliVersion/ {print $2; exit}' "${PWGO_MOD_DIR}/run.go" > /app/playwright.version
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server cmd/server/main.go

FROM ubuntu:noble

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
    PLAYWRIGHT_DRIVER_PATH=/ms-playwright-driver \
    PLAYWRIGHT_NODEJS_PATH=/usr/bin/node

COPY --from=builder /app/playwright.version /tmp/playwright.version

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates tzdata wget nodejs npm \
    && PW_CORE_VER="$(cat /tmp/playwright.version)" \
    && npm install --prefix /tmp/playwright-driver --ignore-scripts --no-audit --no-fund "playwright@${PW_CORE_VER}" \
    && mkdir -p "${PLAYWRIGHT_DRIVER_PATH}/node_modules" \
    && mv /tmp/playwright-driver/node_modules/playwright "${PLAYWRIGHT_DRIVER_PATH}/package" \
    && mv /tmp/playwright-driver/node_modules/playwright-core "${PLAYWRIGHT_DRIVER_PATH}/node_modules/playwright-core" \
    && node "${PLAYWRIGHT_DRIVER_PATH}/package/cli.js" install --with-deps chromium \
    && rm -rf /tmp/playwright.version /tmp/playwright-driver /root/.npm \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/config /app/config

EXPOSE 8091

ENTRYPOINT ["/app/server"]
CMD ["--config", "config/config.prod.yaml"]
