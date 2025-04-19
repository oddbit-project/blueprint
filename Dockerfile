FROM golang:1.23

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=1

CMD ["go", "test", "-v", "-race","-tags=integration,race", "./..."]
