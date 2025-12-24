FROM golang:alpine AS builder

WORKDIR /build
COPY ["go.sum", "go.mod", "./"]
RUN go mod download
COPY . .
RUN go build -o api cmd/main.go 

FROM alpine

WORKDIR /app
COPY --from=builder /build/api .
CMD ["./api"]