FROM golang:1.10.3 as builder

WORKDIR /go/src/admiralty.io/multicluster-controller
COPY . .

ARG target
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager ${target}

FROM scratch
WORKDIR /root/
COPY --from=builder /go/src/admiralty.io/multicluster-controller/manager .
ENTRYPOINT ["./manager"]
