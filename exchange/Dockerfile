# 多阶段构建：编译出小体积镜像（学习 Docker 部署）
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/exchange .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ranking ./cmd/ranking

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/exchange /app/exchange
COPY --from=build /out/ranking /app/ranking
# embed 已打进二进制，无需再拷 static/migrations
EXPOSE 8080
ENTRYPOINT ["/app/exchange"]
