FROM golang:1.22.5 AS build
WORKDIR /app
COPY cmd/service-b/. .
COPY go.mod .
COPY go.sum .
RUN CGO_ENABLED=0 GOOS=linux go build -o main

FROM scratch
WORKDIR /app
COPY --from=build /app/main .
ENTRYPOINT ["/app/main"]
