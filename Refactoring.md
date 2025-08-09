# 修正をおすすめするポイント（重要度順）

## A. すぐ直す/確認する系

1. **Go バージョンの扱い**

   * 現在は **Go 1.24.6 で問題なし**。そのままで OK。
   * **CI/ビルダー/Buildpacks** の Go バージョンピン留めを確認（GitHub Actions・Cloud Build の `setup-go` / `go` バージョン一致）。
   * Docker ベースイメージ（`golang:1.24.6-alpine` など）は **digest ピン留め**を検討（サプライチェーン安定化）。

2. **モジュールパスの実リポジトリ化**

   * `go.mod` の `module` を実パスへ（例: `github.com/lancelop89/youtube-trend-tracker`）。
   * 依存整理: `go mod tidy`、相対 import の残骸を排除。

3. **環境変数名の統一**

   * `GOOGLE_CLOUD_PROJECT` に統一（Cloud Run が自動注入）。
   * フォールバックで `PROJECT_ID` を読んでもよいが、**どちらが正として優先か**をコードに明記。

4. **`CHANNEL_CONFIG_PATH`**\*\* の配布方法\*\*

   * 本番は **ボリュームマウント不可** ⇒ **コンテナ同梱**（推奨）または **`CHANNELS_JSON`**\*\* を ENV 供給\*\*。
   * Dockerfile に `COPY configs/channels.yaml /srv/channels.yaml`、`ENV CHANNEL_CONFIG_PATH=/srv/channels.yaml`。

5. **`godotenv`**\*\* の取り扱い\*\*

   * 本番で `.env` は使わない。
   * `if isLocal()` のように **ローカル限定で Load** する条件分岐に。

## B. 品質/運用向上

6. **BigQuery スキーマ/パーティション設計**

   * 追記テーブルは **`snapshot_date`**\*\* パーティション\*\*＋**`channel_id, video_id`**\*\*\*\*\*\*\*\* クラスタ**。
   * **`insertId`** を `video_id||'-'||FORMAT_DATE('%F', snapshot_date)` 等で指定し重複防止。
   * データセット/リージョンは Cloud Run と同系。

7. **レート制御とリトライ**（YouTube Data API）

   * **指数バックオフ**（429/5xx、`Retry-After` 尊重）。
   * **取得上限**（1 日の最大件数・ページ数）を ENV で外出し。
   * ETag を活用できる API では `If-None-Match` による節約も検討。

8. **構造化ログ**

   * JSON で `severity` / `message` / `labels`（`video_id`/`channel_id`/`job`）を出力。
   * 失敗時は `error` レベルに stack/原因を含める。

9. **ヘルスチェック/メタ情報エンドポイント**

   * `GET /healthz`（200）と `GET /info`（version, commit, build\_ts）。
   * Cloud Run の起動確認・運用トラブルシュートが楽になる。

10. **テスト戦略の分離**

* `-tags integration` で結合テスト（実 API）を分離。
* ユニットはモック/レコーダーで安定化。CI は基本ユニットのみ。

11. **Dockerfile の微調整**

* `ENV PORT=8080` を明示、アプリ側で可変 PORT に対応。
* マルチステージで **最終イメージを distroless**（既に採用なら維持）。
* 不要ツールを削ぎ、`USER nonroot` を検討。

12. **スクリプトの堅牢化**

* `set -euo pipefail` を先頭に。
* 既存確認 → 作成の分岐で `jq` を使用し JSON を厳密に扱う。
* 失敗時に明確な exit code とメッセージ。

## C. あったら嬉しい（中期）

* **Terraform/IaC**：Cloud Run / Scheduler / SA / Secret / IAM / BQ をコード化。
* **Looker Studio 用 SQL/ビュー**：`/deployments/bq/` に保存。
* **セキュリティ強化**：Workload Identity Federation（必要に応じて）、Secret 自動ローテ。
* **監視**：Error Reporting、アラート（エラー率・429 連発・BQ 失敗）。

---

# 目的 / スコープ

* 収集対象は **動画・チャンネルのメタ情報＋集計値（コメントは“数のみ”）**。
* BigQuery は **追記型スナップショット**を基本にし、\*\*最新ビュー（or ディメンション）\*\*を別途維持。
* テーブルは **日次パーティション**＋**クラスタリング**でコスト最適化。
* Cloud Run からの実行・Cloud Scheduler で定期実行・Secret Manager で鍵管理。

---

# データモデル（BigQuery）

## 共通ポリシー

* **パーティション列**: `snapshot_date` (DATE)
* **クラスタ列**: `channel_id, video_id`（テーブルに応じて片方のみ）
* **重複排除**: `insertId` を `(resource_id || '-' || snapshot_date)` などで指定
* **タイムスタンプ**: `snapshot_ts TIMESTAMP` を格納（可視化/最新抽出用）

## 1) 動画スナップショット（追記）

```sql
CREATE TABLE IF NOT EXISTS yt.fact_video_stats_snapshots (
  snapshot_date   DATE       NOT NULL,
  snapshot_ts     TIMESTAMP  NOT NULL,
  video_id        STRING     NOT NULL,
  channel_id      STRING     NOT NULL,
  title           STRING,
  published_at    TIMESTAMP,
  tags            ARRAY<STRING>,
  category_id     STRING,
  duration        STRING,
  definition      STRING,
  licensed        BOOL,
  view_count      INT64,
  like_count      INT64,
  comment_count   INT64,
  etag            STRING,
  source          STRING
)
PARTITION BY snapshot_date
CLUSTER BY channel_id, video_id;
```

## 2) チャンネルスナップショット（追記）

```sql
CREATE TABLE IF NOT EXISTS yt.fact_channel_stats_snapshots (
  snapshot_date   DATE       NOT NULL,
  snapshot_ts     TIMESTAMP  NOT NULL,
  channel_id      STRING     NOT NULL,
  title           STRING,
  custom_url      STRING,
  country         STRING,
  published_at    TIMESTAMP,
  subscriber_count INT64,
  view_count      INT64,
  video_count     INT64,
  etag            STRING,
  source          STRING
)
PARTITION BY snapshot_date
CLUSTER BY channel_id;
```

## 3) 参照テーブル

```sql
CREATE TABLE IF NOT EXISTS yt.lu_video_categories (
  category_id STRING NOT NULL,
  title       STRING,
  assignable  BOOL,
  updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP()
);
```

## 4) 最新ビュー/ディメンション

```sql
CREATE OR REPLACE VIEW yt.vw_latest_video_stats AS
SELECT * EXCEPT(rn)
FROM (
  SELECT t.*, ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY snapshot_ts DESC) AS rn
  FROM yt.fact_video_stats_snapshots t
)
WHERE rn = 1;
```

---

# 収集 API と上限

* **videos.list**: `snippet,contentDetails,statistics,topicDetails`
* **channels.list**: `snippet,statistics,brandingSettings`
* **videoCategories.list**: 参照用

**除外**

* コメント本文は保存せず `comment_count` のみ
* `search/playlistItems`: 必要になってから導入

**取得制御 ENV 例**

```yaml
FETCH_LIMITS:
  max_videos_per_day: 2000
  max_pages_per_call: 5
  request_interval_ms: 100
RETRY:
  max_retries: 5
  initial_backoff_ms: 500
```

---

# Cloud Run / インフラ

## ENV

* `GOOGLE_CLOUD_PROJECT`
* `YOUTUBE_API_KEY`（Secret Manager）
* `CHANNEL_CONFIG_PATH=/srv/channels.yaml`

## Secret / 権限

* `roles/bigquery.dataEditor`
* `roles/secretmanager.secretAccessor`

## 配置

* `configs/channels.yaml` をコンテナに同梱（`COPY` + `ENV`）

## スケジューラ

* Cloud Scheduler → Cloud Run (POST, OIDC)

## ログ/ヘルスチェック

* JSON 構造化ログ
* `/healthz` `/info`

---

# タスクリスト

*

---

# リファクタリング進捗チェックリスト

## A. すぐ直す/確認する系
- [x] 1. Go バージョンの扱い (Go 1.24.6 に統一)
  - [ ] CI/ビルダー/Buildpacks の Go バージョンピン留め (スキップ)
  - [ ] Docker ベースイメージの digest ピン留め (スキップ)
- [x] 2. モジュールパスの実リポジトリ化
- [x] 3. 環境変数名の統一
- [x] 4. `CHANNEL_CONFIG_PATH` の配布方法
- [x] 5. `godotenv` の取り扱い

## B. 品質/運用向上
- [x] 6. BigQuery スキーマ/パーティション設計
- [x] 7. レート制御とリトライ (指数バックオフは実装済み)
  - [x] 取得上限の外出し
  - [ ] ETag の活用 (スキップ)
- [x] 8. 構造化ログ
- [x] 9. ヘルスチェック/メタ情報エンドポイント
- [ ] 10. テスト戦略の分離 (スキップ)
- [x] 11. Dockerfile の微調整
- [ ] 12. スクリプトの堅牢化 (スキップ)
