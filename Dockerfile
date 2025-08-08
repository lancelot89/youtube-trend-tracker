# Stage 1: ビルド用のステージ
FROM golang:1.22-alpine AS builder

# ca-certificatesをインストール（GCP APIへのHTTPS通信に必要）
RUN apk --no-cache add ca-certificates

WORKDIR /app

# 依存関係のレイヤーをキャッシュさせるために、先にgo.modとgo.sumをコピー
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

# アプリケーションをビルド
# CGO_ENABLED=0: CGOを無効化し、静的バイナリを生成
# GOOS=linux: Cloud Runの実行環境であるLinux向けのバイナリを生成
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/fetcher ./cmd/fetcher

# Stage 2: 実行用のステージ
FROM alpine:latest

# Stage 1からca-certificatesをコピー
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 非rootユーザーで実行するための設定
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

WORKDIR /home/appuser

# ビルドしたバイナリをコピー
COPY --from=builder /app/fetcher .

# Cloud RunがPORT環境変数を自動で設定するが、デフォルトポートをドキュメント化
EXPOSE 8080

# コンテナ起動時に実行するコマンド
ENTRYPOINT ["./fetcher"]
