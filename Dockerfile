FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY workers ./workers
COPY *.go ./
RUN go build -o /out/swap-rithms .

FROM node:24-alpine
RUN apk add --no-cache python3
COPY --from=build /out/swap-rithms /swap-rithms
EXPOSE 8080
ENTRYPOINT ["/swap-rithms"]
