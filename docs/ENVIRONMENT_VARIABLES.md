# 環境変数リファレンス

このドキュメントは YouTube Trend Tracker で使用される全ての環境変数を説明します。

## 必須環境変数

### GCP関連

| 変数名 | 説明 | 例 | 使用場所 |
|--------|------|-----|----------|
| `PROJECT_ID` | GCPプロジェクトID | `my-project-123` | 全スクリプト |
| `REGION` | GCPリージョン | `asia-northeast1` | Cloud Run, Artifact Registry |
| `AR_REPO` | Artifact Registryリポジトリ名 | `youtube-trend-repo` | Docker push/pull |
| `SERVICE_NAME` | Cloud Runサービス名 | `youtube-trend-tracker` | デプロイ、スケジューラー |

### 認証関連

| 変数名 | 説明 | 例 | 使用場所 |
|--------|------|-----|----------|
| `YOUTUBE_API_KEY` | YouTube Data API v3のキー | `AIza...` | ローカル開発、Secret作成時 |
| `SECRET_NAME` | Secret Manager上のシークレット名 | `youtube-api-key` | Cloud Run実行時 |

### BigQuery関連

| 変数名 | 説明 | 例 | デフォルト値 |
|--------|------|-----|-------------|
| `BQ_DATASET` | BigQueryデータセット名 | `youtube` | `youtube` |
| `BQ_TABLE_VIDEOS` | 動画データテーブル名 | `videos` | `videos` |
| `BQ_TABLE_CHANNELS` | チャンネルデータテーブル名 | `channels` | `channels` |

### アプリケーション設定

| 変数名 | 説明 | 例 | デフォルト値 |
|--------|------|-----|-------------|
| `GOOGLE_CLOUD_PROJECT` | GCPプロジェクトID（実行時） | `my-project-123` | `PROJECT_ID`と同じ |
| `GO_ENV` | 実行環境 | `local`, `production` | `local` |
| `MAX_VIDEOS_PER_CHANNEL` | チャンネルごとの最大動画取得数 | `200` | `200` |
| `LOG_LEVEL` | ログレベル | `debug`, `info`, `warn`, `error` | `info` |
| `PORT` | HTTPサーバーポート | `8080` | `8080` |

## オプション環境変数

### 開発環境用

| 変数名 | 説明 | 例 | 用途 |
|--------|------|-----|------|
| `BIGQUERY_EMULATOR_HOST` | BigQueryエミュレータのホスト | `localhost:9060` | ローカルテスト |
| `GOOGLE_APPLICATION_CREDENTIALS` | サービスアカウントキーファイルパス | `/path/to/key.json` | ローカル認証（ADC推奨） |

## 設定ファイルの使い方

### 1. 初期設定

```bash
# .env.example をコピー
cp .env.example .env

# 必要な値を編集
vim .env
```

### 2. 環境変数の読み込み

```bash
# Bashの場合
export $(grep -v '^#' .env | xargs)

# Makefileを使う場合（自動読み込み）
make run
```

### 3. スクリプト実行時の優先順位

1. コマンドライン引数（最優先）
2. 環境変数
3. デフォルト値

## トラブルシューティング

### よくあるエラー

| エラー | 原因 | 解決方法 |
|--------|------|----------|
| `YOUTUBE_API_KEY environment variable is not set` | APIキーが未設定 | `.env`ファイルに`YOUTUBE_API_KEY`を設定 |
| `PROJECT_ID is not set` | プロジェクトIDが未設定 | `export PROJECT_ID=your-project-id` |
| `invalid region` | リージョンが無効 | 有効なリージョン（例：`asia-northeast1`）を設定 |

### 環境変数の確認方法

```bash
# 現在の環境変数を確認
env | grep -E "PROJECT_ID|REGION|AR_REPO|SERVICE_NAME"

# .envファイルの内容を確認（秘密情報に注意）
grep -v '^#' .env | grep -v '^$'
```

## セキュリティ注意事項

- **`.env`ファイルはGitにコミットしない**（`.gitignore`に登録済み）
- **本番環境ではSecret Managerを使用**（`YOUTUBE_API_KEY`を直接環境変数に設定しない）
- **サービスアカウントキーは最小権限で作成**