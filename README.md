# YouTube トレンドトラッカー

> **目的**: 日本国内の YouTube チャンネルや動画のトレンド推移を定期取得し、BigQuery へ蓄積→Looker Studio ダッシュボードで可視化します。

---

## アーキテクチャ概要

```text
┌───────────────┐        ┌──────────────┐
│ Cloud Scheduler│ ─→    │  Cloud Run   │
│  (cron 1h)     │  HTTP │  (API Fetch) │
└───────────────┘        └──────┬───────┘
                                 │ Pub/Sub (raw)
                                 ▼
                          ┌──────────────┐
                          │ Cloud Tasks  │ (retry / rate‑limit)
                          └──────────────┘
                                 │
                                 ▼
                          ┌──────────────┐
                          │  BigQuery    │  (videos, channels tables)
                          └──────────────┘
```

*コンテナビルドは Cloud Build を使わず、**ローカル Docker + Artifact Registry** に直接 push する想定です。*

---

## 前提

| 項目         | バージョン例    |
| ---------- | --------- |
| gcloud CLI | ≥ 474.0.0 |
| Docker     | ≥ 24      |
| jq (任意)    | ≥ 1.6     |

* **Billing が有効化**された GCP プロジェクトを用意してください。
* YouTube Data API v3 の API キーを発行済みであること。

---

## クイックスタート (TL;DR)

```bash
# 変数を自分の環境に合わせて設定
export PROJECT_ID="your-project-id"
export REGION="us-central1"        # Cloud Run & Artifact Registry
export SERVICE="youtube-trend-tracker"
export AR_REPO="trend-tracker-repo"

# 1) API 有効化
bash scripts/enable-apis.sh  "$PROJECT_ID"

# 2) Artifact Registry 作成
bash scripts/setup-artifact-registry.sh "$PROJECT_ID" "$REGION" "$AR_REPO"

# 3) コンテナをビルドして push
bash scripts/build-and-push.sh "$PROJECT_ID" "$REGION" "$AR_REPO"

# 4) Cloud Run へデプロイ
bash scripts/deploy-cloud-run.sh "$PROJECT_ID" "$REGION" "$SERVICE" "$AR_REPO"

# 5) 1 時間ごとに実行する Cloud Scheduler を作成
bash scripts/create-scheduler.sh "$PROJECT_ID" "$REGION" "$SERVICE"
```

> **備考**: 上記スクリプトは `scripts/` ディレクトリにサンプルとして格納しています。環境変数・IAM・Secret Manager の参照はプロジェクト構成に応じて修正してください。

---

## 詳細ステップ

### 1. API を有効化

```bash
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  bigquery.googleapis.com \
  pubsub.googleapis.com \
  cloudscheduler.googleapis.com \
  secretmanager.googleapis.com
```

### 2. IAM & サービス アカウント

| SA                 | 用途                                | 付与ロール                                            |
| ------------------ | --------------------------------- | ------------------------------------------------ |
| `trend-tracker-sa` | Cloud Run 実行 & BigQuery 書込        | `roles/run.invoker`, `roles/bigquery.dataEditor` |
| `scheduler-sa`     | Cloud Scheduler → Cloud Run 呼び出し用 | `roles/run.invoker`                              |

```bash
# 例) SA 作成と役割付与
gcloud iam service-accounts create trend-tracker-sa \
  --display-name "YouTube Trend Tracker"

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:trend-tracker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.invoker"
```

### 3. Secret Manager に API キーを登録

```bash
echo -n "$YOUTUBE_API_KEY" | gcloud secrets create youtube-api-key \
  --replication-policy="automatic" \
  --data-file=-
```

### 4. Artifact Registry にコンテナを push

```bash
# 認証
gcloud auth configure-docker "$REGION"-docker.pkg.dev

# ビルド&push
docker build -t "$REGION"-docker.pkg.dev/"$PROJECT_ID"/"$AR_REPO"/tracker:latest .
docker push "$REGION"-docker.pkg.dev/"$PROJECT_ID"/"$AR_REPO"/tracker:latest
```

### 5. Cloud Run デプロイ

```bash
gcloud run deploy "$SERVICE" \
  --image="$REGION"-docker.pkg.dev/"$PROJECT_ID"/"$AR_REPO"/tracker:latest \
  --service-account="trend-tracker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --set-secrets="YOUTUBE_API_KEY=youtube-api-key:latest" \
  --region="$REGION" --platform=managed --allow-unauthenticated
```

### 6. Cloud Scheduler の作成

```bash
CRON_SVC_URL=$(gcloud run services describe "$SERVICE" --region="$REGION" --format="value(status.url)")

gcloud scheduler jobs create http trend-tracker-hourly \
  --schedule="0 * * * *" \
  --uri="$CRON_SVC_URL" \
  --http-method=POST \
  --oauth-service-account="scheduler-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --headers="Content-Type=application/json"
```

---

## ローカル開発

```bash
# .env.example をコピーして必要な環境変数を書き換え
cp .env.example .env

# BigQuery エミュレータを使用する場合 (任意)
# docker run -p 9060:9060 -p 9061:9061 --name bigquery-emulator -d goccy/bigquery-emulator
# .env ファイルに BIGQUERY_EMULATOR_HOST=localhost:9060 を設定

# 単体テストの実行
go test ./...

# ローカルでの動作確認
go run ./cmd/fetcher/main.go --once --debug
```

---

## 設定ファイル

本プロジェクトでは、設定を YAML ファイルで管理しています。

- `configs/project.yaml`: プロジェクト全体のメタデータ（モジュール構成、利用する Secret 名など）を定義します。
- `configs/channels.yaml`: トレンドを監視したい YouTube チャンネルの ID をリスト形式で指定します。

---

## データモデル (BigQuery)

取得した動画データは、以下のスキーマで BigQuery に保存されます。スキーマの詳細は `deployments/bq/schema.json` を参照してください。

| フィールド名   | 型        | 説明                               |
| :------------- | :-------- | :--------------------------------- |
| `ts`           | TIMESTAMP | データ取得タイムスタンプ (必須)    |
| `channel_id`   | STRING    | YouTube チャンネル ID (必須)       |
| `video_id`     | STRING    | YouTube 動画 ID (必須)             |
| `title`        | STRING    | 動画タイトル                       |
| `views`        | INTEGER   | 再生回数                           |
| `likes`        | INTEGER   | 高評価数                           |
| `comments`     | INTEGER   | コメント数                         |
| `published_at` | TIMESTAMP | 動画の公開日時                     |

---

## コスト試算 (2025‑08 時点, 東京リージョン)

| サービス              | 月次想定コスト        | 備考                                            |
| ----------------- | -------------- | --------------------------------------------- |
| Cloud Run         | ~¥150          | 1 h 周期, 実行 30 秒 / 回, 256 MiB, 100 万リクエスト/月 未満 |
| Artifact Registry | ~¥50           | 1 GB ストレージ                                    |
| BigQuery          | ~¥200          | 2 GB ストレージ + 5 GB クエリ/月                       |
| Cloud Scheduler   | ~¥0            | 月 730 回は無料枠内                                  |
| 合計                | **≈ ¥400 / 月** | *為替レート & 利用状況で変動*                             |

---

## リソース削除

```bash
gcloud run services delete "$SERVICE" --region="$REGION"
gcloud scheduler jobs delete trend-tracker-hourly
gcloud artifacts repositories delete "$AR_REPO" --location="$REGION" --quiet
gcloud secrets delete youtube-api-key
```

---

## ライセンス

MIT