# Build Stage

FROM golang:1.21-alpine AS build

WORKDIR /shove

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o shove ./cmd/shove

FROM alpine:3.19

WORKDIR /server

COPY --from=build /shove/shove .

RUN apk --no-cache add ca-certificates tzdata

USER shover:shover

ENTRYPOINT ["/server/shove"]
