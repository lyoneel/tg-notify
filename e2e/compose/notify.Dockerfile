FROM golang:1.24 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /tg-notify ./cmd/tg-notify

FROM alpine:3.20
COPY --from=build /tg-notify /tg-notify
COPY e2e/compose/notify-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
