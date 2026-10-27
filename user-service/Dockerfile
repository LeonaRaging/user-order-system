FROM golang:1.26 AS build

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o user-service .

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app/user-service .
EXPOSE 8081
CMD ["./user-service"]