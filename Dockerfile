FROM golang:1.19-alpine

WORKDIR /app

RUN go mod tidy

RUN go build main.go


