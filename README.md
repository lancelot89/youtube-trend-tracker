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

最初にスクリプトへ実行権限を付与してください。
```bash
chmod +x scripts/*.sh
```

次に、ご自身の環境に合わせて変数を設定し、以下のコマンドを順番に実行します。

```bash
# 1. 環境変数の設定
export PROJECT_ID="PROJECT_ID_REDACTED"
export REGION="asia-northeast1"          # 例: us-central1, asia-northeast1
export AR_REPO="youtube-trend-repo"      # Artifact Registry のリポジトリ名
export SERVICE_NAME="youtube-trend-tracker" # Cloud Run のサービス名
export YOUTUBE_API_KEY="your-youtube-api-key" # ご自身の API キーを設定してください

# 2. デプロイスクリプトの実行
echo "Step 1/6: Google Cloud API の有効化..."
./scripts/enable-apis.sh "${PROJECT_ID}"

echo -e "\nStep 2/6: Artifact Registry リポジトリのセットアップ..."
./scripts/setup-artifact-registry.sh "${PROJECT_ID}" "${REGION}" "${AR_REPO}"

echo -e "\nStep 3/6: YouTube APIキーを Secret Manager に登録..."
./scripts/create-secret.sh

echo -e "\nStep 4/6: コンテナイメージのビルドとプッシュ..."
./scripts/build-and-push.sh "${PROJECT_ID}" "${REGION}" "${AR_REPO}" "${SERVICE_NAME}"

echo -e "\nStep 5/6: Cloud Run サービスのデプロイ..."
./scripts/deploy-cloud-run.sh "${PROJECT_ID}" "${REGION}" "${SERVICE_NAME}" "${AR_REPO}"

echo -e "\nStep 6/6: Cloud Scheduler ジョブの作成・更新..."
./scripts/create-scheduler.sh "${PROJECT_ID}" "${REGION}" "${SERVICE_NAME}"

echo -e "\n\n✅ 全てのデプロイプロセスが完了しました。"
```

### 3. 2回目以降のデプロイ (コード変更時など)

コンテナイメージの再ビルド、プッシュ、Cloud Run サービスの更新をまとめて実行します。

```bash
./scripts/redeploy.sh "${PROJECT_ID}" "${REGION}" "${AR_REPO}" "${SERVICE_NAME}"
```

> **備考**: 各スクリプトは冪等性を持つように作られていますが、IAMやSecretなど、環境に依存する値がハードコードされている場合があります。必要に応じて `scripts/` ディレクトリ内のスクリプトを直接編集してください。


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

# プロジェクトレベルのロールを付与
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:trend-tracker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.invoker"

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:trend-tracker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/bigquery.dataEditor"

# Secret へのアクセス権を付与
gcloud secrets add-iam-policy-binding youtube-api-key \
    --member="serviceAccount:trend-tracker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"


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
# 変数名はクイックスタートの例を参照
gcloud run deploy "$SERVICE_NAME" \
  --image="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}/${SERVICE_NAME}:latest" \
  --service-account="trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --region="$REGION" \
  --platform=managed \
  --no-allow-unauthenticated \
  --config-file deployments/cloudrun/service.yaml
```

### 6. Cloud Scheduler の作成

```bash
# 変数名はクイックスタートの例を参照
CRON_SVC_URL=$(gcloud run services describe "$SERVICE_NAME" --region="$REGION" --format="value(status.url)")

# 冪等性のため create の代わりに update を使用
gcloud scheduler jobs update http trend-tracker-hourly \
  --schedule="0 * * * *" \
  --uri="$CRON_SVC_URL" \
  --http-method=POST \
  --oauth-service-account-email="scheduler-sa@${PROJECT_ID}.iam.gserviceaccount.com"
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

### GCP 環境での動作確認

デプロイ済みの Cloud Run サービスをローカルからトリガーして動作を確認します。

1.  **Cloud Run サービスの URL を取得**:
    ```bash
    SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region=${REGION} --format="value(status.url)")
    echo "Cloud Run Service URL: ${SERVICE_URL}"
    ```
2.  **サービスをトリガー**:
    Cloud Run サービスは認証を必要とするため、`gcloud auth print-identity-token` で認証トークンを取得し、`Authorization` ヘッダーに含めてリクエストを送信します。
    ```bash
    AUTH_TOKEN=$(gcloud auth print-identity-token)
    curl -X POST -H "Authorization: Bearer ${AUTH_TOKEN}" ${SERVICE_URL}
    ```
    成功すると `{"status":"success"}` が返されます。
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
| `dt`           | DATE      | スナップショット日付 (必須)        |
| `channel_id`   | STRING    | YouTube チャンネル ID (必須)       |
| `video_id`     | STRING    | YouTube 動画 ID (必須)             |
| `title`        | STRING    | 動画タイトル                       |
| `channel_name` | STRING    | チャンネル名                       |
| `tags`         | STRING    | 動画タグ (繰り返し)                |
| `is_short`     | BOOLEAN   | ショート動画フラグ                 |
| `views`        | INTEGER   | 再生回数                           |
| `likes`        | INTEGER   | 高評価数                           |
| `comments`     | INTEGER   | コメント数                         |
| `published_at` | TIMESTAMP | 動画の公開日時                     |
| `created_at`   | TIMESTAMP | データ取得タイムスタンプ (必須)    |

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