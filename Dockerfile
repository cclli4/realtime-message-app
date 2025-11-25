FROM golang:alpine

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod tidy

COPY . .

RUN go build -o message-app .
RUN go build -o persistence-worker ./cmd/persistence-worker
RUN go build -o notification-worker ./cmd/notification-worker
RUN go build -o query-service ./cmd/query-service
RUN chmod +x message-app persistence-worker notification-worker query-service

EXPOSE 9000

EXPOSE 8080

CMD ["./message-app"]
