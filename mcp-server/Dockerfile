FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go vet ./... && go test ./... && CGO_ENABLED=0 go build -o /out/mcp-server .
FROM alpine:3.20
RUN adduser -D -u 10002 mcpuser
USER mcpuser
COPY --from=build /out/mcp-server /app/mcp-server
EXPOSE 8090
CMD ["/app/mcp-server"]
