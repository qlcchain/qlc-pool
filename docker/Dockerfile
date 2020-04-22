# Build gqlc-pool in a stock Go builder container
FROM qlcchain/go-qlc-builder:latest as builder

ENV GOPROXY https://goproxy.cn

ARG BUILD_ACT=build

COPY . /qlcchain/qlc-pool
RUN cd /qlcchain/qlc-pool && make clean ${BUILD_ACT}

# Pull gqlc-pool into a second stage deploy alpine container
FROM alpine:3.11.3

COPY --from=builder /qlcchain/qlc-pool/build/gqlc-pool .

# EXPOSE 3333

ENTRYPOINT [ "/gqlc-pool"]
