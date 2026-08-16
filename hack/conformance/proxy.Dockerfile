# The product egress proxy image (cmd/cloudbox-proxy), built by
# hack/conformance/build-proxy-image.sh and preloaded into the Kind clusters.
FROM scratch
COPY cloudbox-proxy /cloudbox-proxy
ENTRYPOINT ["/cloudbox-proxy"]
