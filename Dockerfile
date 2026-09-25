FROM gcr.io/distroless/static:nonroot
ARG TARGETOS=linux
ARG TARGETARCH
COPY dist/sink_${TARGETOS}_${TARGETARCH}*/sink /sink
USER 65532:65532
ENTRYPOINT ["/sink"]
CMD ["serve"]
