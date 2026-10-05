FROM node:22-alpine AS web
WORKDIR /src/tools/fpbpack/web
COPY tools/fpbpack/web/package.json ./
RUN npm install --no-audit --no-fund
COPY tools/fpbpack/web/ ./
RUN npm run typecheck && npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY tools/fpbpack/ ./tools/fpbpack/
COPY --from=web /src/tools/fpbpack/web/out/ ./tools/fpbpack/internal/webui/dist/
WORKDIR /src/tools/fpbpack
ARG FPBPACK_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${FPBPACK_VERSION}" \
    -o /out/fpbpack \
    ./cmd/fpbpack

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/fpbpack /usr/local/bin/fpbpack
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/fpbpack"]
