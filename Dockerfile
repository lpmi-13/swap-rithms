# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY workers ./workers
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/swap-rithms .

FROM node:24-alpine
RUN apk add --no-cache python3
COPY --from=build /out/swap-rithms /usr/local/bin/swap-rithms
EXPOSE 8080
USER node
WORKDIR /home/node
ENTRYPOINT ["swap-rithms"]
