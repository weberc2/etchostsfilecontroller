FROM  --platform=${BUILDPLATFORM} golang

ENV CGO_ENABLED=0

ARG TARGETOS

ARG TARGETARCH

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go mod download

COPY main.go .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /bin/etc-hosts-file-controller main.go

FROM alpine

COPY --from=0 /bin/etc-hosts-file-controller /bin/etc-hosts-file-controller
