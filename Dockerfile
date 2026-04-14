FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -o /out/avatars-service ./cmd/avatars-service

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /out/avatars-service /usr/local/bin/avatars-service
COPY migrations ./migrations
EXPOSE 8080
ENTRYPOINT ["avatars-service"]
