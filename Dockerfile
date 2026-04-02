FROM --platform=$BUILDPLATFORM golang:1.23 AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/apollo ./cmd/apollo

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/apollo /usr/local/bin/apollo

EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/apollo"]
