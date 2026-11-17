# Build context: the server/ directory.
FROM golang:1.27 AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/airmon-server ./cmd/airmon-server

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/airmon-server /airmon-server
USER nonroot
EXPOSE 8790
# The AAI key is not part of the image: mount it as a file and point
# aai.key_file at it. Mount the config at /etc/airmon-server/config.yaml
# (set listen: 0.0.0.0:8790 in it).
ENTRYPOINT ["/airmon-server"]
CMD ["-config", "/etc/airmon-server/config.yaml"]
