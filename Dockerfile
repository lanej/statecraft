FROM node:22.22.0-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY assets/ /src/assets/
RUN npm run build

FROM golang:1.26.0-bookworm AS api
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY gen/ gen/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /statecraft ./cmd/statecraft-readonly

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /statecraft /statecraft
COPY --from=web /src/web/dist /web
ENV PORT=8080 STATECRAFT_WEB_DIR=/web
EXPOSE 8080
ENTRYPOINT ["/statecraft"]
