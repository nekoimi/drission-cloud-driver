FROM golang:1.25 AS modules

WORKDIR /modules

COPY go.mod go.sum ./
RUN go mod download

FROM golang:1.25 AS builder

WORKDIR /app

COPY --from=modules /go/pkg /go/pkg

COPY . .

RUN PWGO_VER=$(awk '/github.com\/playwright-community\/playwright-go/ {print $2; exit}' go.mod) \
    && go install github.com/playwright-community/playwright-go/cmd/playwright@${PWGO_VER}
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server cmd/server/main.go

FROM ubuntu:noble

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

COPY --from=builder /go/bin/playwright /usr/local/bin/playwright

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && playwright install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/configs /app/configs

EXPOSE 8091

ENTRYPOINT ["/app/server"]
CMD ["--config", "configs/config.prod.yaml"]
