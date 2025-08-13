FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY main.go .
COPY go.mod .

RUN go build -o mock-jira main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/mock-jira .

EXPOSE 8080
CMD ["./mock-jira"]