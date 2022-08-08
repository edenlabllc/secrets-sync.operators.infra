FROM golang:1.17-alpine as builder

ARG VERSION="v0.1.0"

ENV NAME=secrets-sync
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

WORKDIR /usr/src/app

COPY . .

RUN go mod tidy && \
    go build -ldflags "-w -s -X main.version=${VERSION} -X main.name=${NAME}" \
    -v -o ./"${NAME}_${GOOS}_${GOARCH}"

FROM amd64/alpine:3.16

ENV NAME=secrets-sync
ENV GOOS=linux
ENV GOARCH=amd64
ENV KUBECONFIG=""

# Create a group and user
RUN addgroup -S appgroup && adduser -HS appuser -G appgroup

COPY --from=builder /usr/src/app/${NAME}_${GOOS}_${GOARCH} /usr/local/bin/${NAME}

USER appuser

CMD ["secrets-sync"]
