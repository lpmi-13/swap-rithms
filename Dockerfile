FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go build -o /out/swap-rithms .

FROM alpine:3.20
COPY --from=build /out/swap-rithms /swap-rithms
EXPOSE 8080
ENTRYPOINT ["/swap-rithms"]
