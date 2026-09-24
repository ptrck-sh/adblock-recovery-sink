FROM gcr.io/distroless/static:nonroot
ARG TARGETOS=linux
ARG TARGETARCH
COPY dist/sink_${TARGETOS}_${TARGETARCH}*/sink /sink
USER nonroot:nonroot
ENTRYPOINT ["/sink"]
CMD ["serve"]
