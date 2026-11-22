FROM docker.io/library/golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /shortener .

FROM docker.io/library/alpine:3.21
COPY --from=build /shortener /usr/local/bin/shortener
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/shortener"]
