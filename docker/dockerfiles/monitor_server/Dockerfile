FROM golang:1.13-alpine

MAINTAINER Yaechan Kim

WORKDIR /app

COPY . .

RUN apk add --no-cache git

RUN go mod download

RUN go build -o ./build/monitor_server ./cmd/monitor_server/main.go 

RUN git clone https://github.com/eficode/wait-for.git

CMD ["/bin/sh","-c", "./wait-for/wait-for redis_three:8002 redis_one:8000 redis_two:8001 -- ./build/monitor_server"]
