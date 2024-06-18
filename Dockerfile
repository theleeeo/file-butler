FROM golang:1.22 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o file-butler .

FROM alpine:latest  

WORKDIR /app

COPY --from=builder /app/file-butler .

ENTRYPOINT ["./file-butler"]
