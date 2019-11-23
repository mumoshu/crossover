FROM golang:1.12

WORKDIR /workspace

ADD . /workspace

RUN go build -o /usr/local/bin/crossover .

FROM frolvlad/alpine-glibc:alpine-3.10_glibc-2.30

COPY --from=0 /usr/local/bin/crossover /usr/local/bin
