# YouTube Trend Tracker - Project Guidelines

## プロジェクト概要
YouTube Trend Trackerは、指定したYouTubeチャンネルの動画データを定期的に収集し、BigQueryに保存するGo言語製のアプリケーションです。

## アーキテクチャ
- **言語**: Go 1.25.0
- **クラウド**: Google Cloud Platform
  - Cloud Run (アプリケーション実行)
  - BigQuery (データストレージ)
  - Cloud Scheduler (定期実行)
  - Secret Manager (APIキー管理)

## ディレクトリ構造
```
.
├── cmd/fetcher/        # メインアプリケーション
├── internal/           # 内部パッケージ
│   ├── config/        # 設定管理
│   ├── errors/        # エラーハンドリング
│   ├── fetcher/       # データ取得ロジック
│   ├── logger/        # ロギング
│   ├── retry/         # リトライロジック
│   ├── storage/       # BigQueryストレージ
│   └── youtube/       # YouTube API クライアント
├── configs/           # 設定ファイル
├── deployments/       # デプロイメント設定
└── scripts/           # ユーティリティスクリプト
```

## 開発ガイドライン

### コーディング規約
- Go標準のフォーマッティング（`go fmt`）を使用
- エラーハンドリングは`internal/errors`パッケージを使用
- ロギングは`internal/logger`パッケージを使用
- 設定は`internal/config`パッケージで一元管理

### テスト
- 単体テストは`*_test.go`ファイルに記述
- `make test`で全テストを実行
- `make coverage`でカバレッジレポートを生成

### CI/CD
- GitHub Actionsを使用
- mainブランチへのプッシュで自動テスト実行
- PRには必ずテストを含める

## 設定管理
- `configs/config.yaml`: デフォルト設定
- 環境変数で上書き可能
- 必須環境変数:
  - `YOUTUBE_API_KEY`: YouTube Data API v3のAPIキー
  - `GOOGLE_CLOUD_PROJECT`: GCPプロジェクトID

## デプロイメント
```bash
# 初期セットアップ
make setup

# Cloud Runへのデプロイ
make deploy

# ローカル実行
make run
```

## トラブルシューティング
- ログは構造化JSON形式で出力
- `LOG_LEVEL`環境変数でログレベルを調整可能（debug, info, warning, error）
- BigQueryエミュレータを使用したローカルテストが可能

## セキュリティ
- APIキーはSecret Managerで管理
- 最小権限の原則に従ったIAM設定
- 定期的な依存関係の更新

## メンテナンス
- 依存関係の更新: `go mod tidy`
- セキュリティスキャン: `make security-scan`
- コードフォーマット: `make fmt`