FROM golang:1.22.10 AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main

# FROM scratch
FROM golang:1.22.10
WORKDIR /app
COPY --from=build /app/main .
ENTRYPOINT ["/app/main"]
