# CampusClaw api（Go）镜像
FROM golang:1.27.4-alpine AS build
WORKDIR /src
# 构建环境默认走国内模块代理（可用 --build-arg 覆盖）
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} GOMAXPROCS=2 GOMEMLIMIT=384MiB
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 单元测试不过不构建镜像（切分策略等核心逻辑先验证再打包）
RUN go vet -p 1 ./... && go test -p 1 ./...
RUN CGO_ENABLED=0 go build -p 1 -o /out/campusclaw-api ./cmd/server
RUN CGO_ENABLED=0 go build -p 1 -o /out/grade-worker ./cmd/worker && CGO_ENABLED=0 go build -p 1 -o /out/grade-relay ./cmd/relay

RUN CGO_ENABLED=0 go build -p 1 -o /out/seed-junior ./cmd/seed-junior

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/campusclaw-api /app/campusclaw-api
COPY --from=build /out/grade-worker /app/grade-worker
COPY --from=build /out/grade-relay /app/grade-relay
COPY --from=build /out/seed-junior /app/seed-junior
RUN mkdir -p /app/uploads && chown appuser:appuser /app/uploads
USER appuser
EXPOSE 8081
CMD ["/app/campusclaw-api"]
