FROM golang:1.24-alpine AS build

WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o main src/main.go

FROM alpine:latest
RUN apk add --no-cache curl tzdata ca-certificates

WORKDIR /app
COPY --from=build /app/main .

EXPOSE 8000
CMD ["./main"]
