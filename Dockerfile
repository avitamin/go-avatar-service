FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -o /out/avatars-service ./cmd/avatars-service

FROM debian:bookworm-slim
WORKDIR /app
RUN groupadd --gid 10001 avatars && \
    useradd --uid 10001 --gid avatars --home-dir /app --shell /usr/sbin/nologin avatars
COPY --from=build /out/avatars-service /usr/local/bin/avatars-service
COPY migrations ./migrations
RUN chown -R avatars:avatars /app /usr/local/bin/avatars-service
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["avatars-service"]
