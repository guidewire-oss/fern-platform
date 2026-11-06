FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY main.go .
COPY go.mod .

RUN go build -o mock-jira main.go

FROM alpine:3.24
# Deselect the eMudhra (emSign) roots from the trust store (Alpine's Mozilla
# bundle still ships them). The final check fails the build if an emSign
# entry is still enabled or its cert is still in the bundle.
RUN apk --no-cache add ca-certificates && \
    sed -i 's#^mozilla/emSign#!mozilla/emSign#' /etc/ca-certificates.conf && \
    update-ca-certificates --fresh && \
    ! grep -q '^mozilla/emSign' /etc/ca-certificates.conf && \
    for f in /usr/share/ca-certificates/mozilla/emSign*.crt; do \
      [ -e "$f" ] || continue; \
      ! grep -v -- '-----' "$f" | grep -qxFf - /etc/ssl/certs/ca-certificates.crt || exit 1; \
    done
WORKDIR /root/
COPY --from=builder /app/mock-jira .

EXPOSE 8080
CMD ["./mock-jira"]