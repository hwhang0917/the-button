FROM node:24-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY --from=web /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /thebutton .

FROM alpine:3.22
RUN adduser -D app && mkdir /data && chown app /data
USER app
ENV DB_PATH=/data/thebutton.db
EXPOSE 8080
VOLUME /data
COPY --from=build /thebutton /usr/local/bin/thebutton
ENTRYPOINT ["thebutton"]
