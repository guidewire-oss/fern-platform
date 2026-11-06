FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY main.go .
COPY go.mod .

RUN go build -o mock-jira main.go

FROM alpine:3.24
# Drop the eMudhra (emSign) root from the trust store. Alpine's Mozilla
# bundle still ships it.
RUN apk --no-cache add ca-certificates && \
    rm -f /usr/share/ca-certificates/mozilla/emSign_Root_CA_-_G1.crt && \
    sed -i '/emSign_Root_CA_-_G1/d' /etc/ca-certificates.conf && \
    update-ca-certificates --fresh
WORKDIR /root/
COPY --from=builder /app/mock-jira .

EXPOSE 8080
CMD ["./mock-jira"]