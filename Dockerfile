FROM golang:1.18 as builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o /bin/docker-retag cmd/docker-retag/*.go

FROM golang:1.18 as app

COPY --from=builder /bin/docker-retag /bin/docker-retag

ENTRYPOINT [ "/bin/docker-retag" ]