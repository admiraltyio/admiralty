FROM golang:1.10.3 as builder

WORKDIR /go/src/admiralty.io/multicluster-service-account

COPY vendor vendor
COPY pkg pkg

ARG target
COPY ${target} ${target}

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager admiralty.io/multicluster-service-account/${target}

FROM scratch
COPY --from=builder /go/src/admiralty.io/multicluster-service-account/manager .
ENTRYPOINT ["./manager"]
