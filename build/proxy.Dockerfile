# The product egress proxy image (cmd/cloudbox-proxy) as the release publishes
# it: built from source per target platform, so one manifest list serves every
# node architecture (design Open Question 2).
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/cloudbox-proxy ./cmd/cloudbox-proxy

FROM scratch
COPY --from=build /out/cloudbox-proxy /cloudbox-proxy
ENTRYPOINT ["/cloudbox-proxy"]
