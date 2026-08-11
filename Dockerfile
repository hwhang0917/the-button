# build stages run on the builder's native arch; only the final image is per-platform
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY *.go ./
COPY --from=web /app/web/dist ./web/dist
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /thebutton .

FROM alpine:3.22
RUN adduser -D app && mkdir /data && chown app /data
USER app
ENV DB_PATH=/data/thebutton.db
EXPOSE 8080
VOLUME /data
COPY --from=build /thebutton /usr/local/bin/thebutton
ENTRYPOINT ["thebutton"]
