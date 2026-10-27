FROM golang:1.26 AS build

WORKDIR /app
COPY go.mod ./
COPY . .
RUN go build -o order-service .

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app/order-service .
EXPOSE 8082
CMD ["./order-service"]