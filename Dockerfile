# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -o /bin/aichatplayers ./cmd/server

FROM alpine:3.20

RUN adduser -D -H app
USER app

WORKDIR /app

COPY --from=build /bin/aichatplayers /app/aichatplayers

EXPOSE 8090

ENTRYPOINT ["/app/aichatplayers"]
CMD ["-listen", ":8090"]
