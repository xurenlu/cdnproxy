# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.22-alpine AS build
WORKDIR /app
RUN apk add --no-cache git ca-certificates && update-ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/cdnproxy ./

# --- run stage ---
FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /bin/cdnproxy /cdnproxy
ENV PORT=8080
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/cdnproxy"]


