FROM golang:1.24 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /fakeapi ./e2e/compose/fakeapi

FROM scratch
COPY --from=build /fakeapi /fakeapi
ENTRYPOINT ["/fakeapi"]
