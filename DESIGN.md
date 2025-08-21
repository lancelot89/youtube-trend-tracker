# YouTube トレンドトラッカー — GeminiCLI 設計書

> **目的**  複数の YouTube チャンネルの統計情報を収集し、最新動画ごとの成長速度を計算して GCP 上に保存・可視化する。
> 本ドキュメントは **GeminiCLI** がプロジェクト一式を自動生成できるように記述されています。

---

## 1  プロジェクトメタデータ（`project.yaml`）

```yaml
name: youtube-trend-tracker
language: go
modules:
  - fetcher        # YouTube Data API を呼び出す Cloud Run サービス
  - scheduler      # fetcher をトリガーする Cloud Scheduler（Cron）
  - storage        # BigQuery のデータセット & テーブル定義
  - analytics      # 成長率計算用 BigQuery ビュー
  - dashboard      # Looker Studio 用データソースの初期化
secrets:
  - YOUTUBE_API_KEY   # Data API v3 用の API キー
  - PROJECT_ID        # GCP プロジェクト ID（デプロイ時に注入）
  - REGION            # Cloud Run を配置する GCP リージョン
```

---

## 2  チャンネル設定（`channels.yaml`）

```yaml
channels:
  # Google Developers
  - id: UC_x5XG1OV2P6uZZ5FSM9Ttw
  # 監視したいチャンネルを追加
  - id: <CHANNEL_ID_2>
```

* GeminiCLI はこのファイルを ConfigMap／Secret としてマウントし、fetcher コンテナに渡します。

---

## 3  アーキテクチャ概要

```
Cloud Scheduler (cron) ─▶ Cloud Run: fetcher
                               │
                               ▼
              BigQuery テーブル : youtube.video_stats_snapshot
                               │
                               ▼
              BigQuery ビュー   : youtube.video_growth
                               │
                               ▼
                     Looker Studio ダッシュボード
```

### 3.1  取得サイクル

| Step | API                                       | 最大件数        | 備考                         |
| ---- | ----------------------------------------- | ----------- | -------------------------- |
| 1    | `channels.list` (part=`contentDetails`)   | 50 ch/リクエスト | `uploads` プレイリスト ID を取得    |
| 2    | `playlistItems.list`                      | 50 本/リクエスト  | 新→旧へページング。`maxPages`=2 が既定 |
| 3    | `videos.list` (part=`statistics,snippet`) | 50 本/リクエスト  | 各動画の統計スナップショット             |
| 4    | テーブル `video_stats_snapshot` へ書き込み         | —           | 追記のみ                       |

### 3.2  成長率ビュー（`video_growth.sql`）

```sql
CREATE OR REPLACE VIEW `${project}.youtube.video_growth` AS
SELECT
  video_id,
  channel_id,
  title,
  published_at,
  CURRENT_TIMESTAMP()          AS calc_ts,
  views,
  views - LAG(views)  OVER w   AS delta_views,
  likes,
  likes - LAG(likes)  OVER w   AS delta_likes,
  SAFE_DIVIDE(delta_views, TIMESTAMP_DIFF(ts, LAG(ts) OVER w, HOUR)) AS view_velocity_per_h
FROM `${project}.youtube.video_stats_snapshot`
WINDOW w AS (
  PARTITION BY video_id ORDER BY ts
);
```

---

## 4  BigQuery スキーマ（`schema.json`）

```json
[{
  "name": "ts",           "type": "TIMESTAMP", "mode": "REQUIRED"},
 {"name": "channel_id",   "type": "STRING",    "mode": "REQUIRED"},
 {"name": "video_id",     "type": "STRING",    "mode": "REQUIRED"},
 {"name": "title",        "type": "STRING",    "mode": "NULLABLE"},
 {"name": "views",        "type": "INTEGER",   "mode": "NULLABLE"},
 {"name": "likes",        "type": "INTEGER",   "mode": "NULLABLE"},
 {"name": "comments",     "type": "INTEGER",   "mode": "NULLABLE"},
 {"name": "published_at", "type": "TIMESTAMP", "mode": "NULLABLE"}
]
```

---

## 5  レポジトリ構成

```
.
├── Dockerfile
├── cmd/
│   └── fetcher/
│       └── main.go
├── internal/
│   ├── youtube/
│   │   ├── client.go
│   │   └── client_test.go
│   ├── storage/
│   │   └── bq.go
│   └── scheduler/
│       └── cron.go
├── configs/
│   ├── project.yaml
│   └── channels.yaml
├── deployments/
│   ├── cloudrun/
│   │   └── service.yaml
│   ├── scheduler/
│   │   └── job.yaml
│   └── bq/
│       ├── schema.json
│       └── video_growth.sql
├── scripts/
│   ├── enable-apis.sh
│   ├── setup-artifact-registry.sh
│   ├── build-and-push.sh
│   ├── deploy-cloud-run.sh
│   └── create-scheduler.sh
└── README.md
```

### 5.1 フォルダ詳細

| ディレクトリ                   | 役割                                                        | 主なファイル                            |
| ------------------------ | --------------------------------------------------------- | --------------------------------- |
| `cmd/`                   | バイナリのエントリーポイントを配置。Cloud Run では `cmd/fetcher/main.go` が起動。 | `main.go`                         |
| `internal/youtube/`      | YouTube Data API 用のラッパー層。ユニットテストもここに配置。      | `client.go`, `client_test.go`     |
| `internal/storage/`      | BigQuery への書き込み、テーブル存在チェックなど永続化処理。                        | `bq.go`                           |
| `internal/scheduler/`    | Cloud Scheduler の Cron 表現生成やバックオフ計算関数。                    | `cron.go`                         |
| `configs/`               | YAML, JSON など静的設定。CI/CD で ConfigMap として展開。                | `project.yaml`, `channels.yaml`   |
| `deployments/cloudrun/`  | Service Manifest。CPU/メモリ、IAM、環境変数を定義。                     | `service.yaml`                    |
| `deployments/scheduler/` | Cloud Scheduler Job Manifest。HTTP ターゲット URL はデプロイ時に補完。    | `job.yaml`                        |
| `deployments/bq/`        | BigQuery スキーマとビュー定義。                                      | `schema.json`, `video_growth.sql` |
| `scripts/`               | ローカル検証やCI/CDで使う補助スクリプト。                                 | `*.sh`                            |

---

## 6  Fetcher サービス仕様

```yaml
entrypoint: go run cmd/fetcher/main.go
env:
  - name: YOUTUBE_API_KEY   # Secret Manager で提供
  - name: CHANNEL_CONFIG    # channels.yaml の内容をインライン渡し
runtime:
  timeout: 300s
  memory: 512Mi
```

* YouTube API への 5xx レスポンス時は指数バックオフで再試行。
* Cloud Logging へ JSON 形式で出力: `{ts, level, channelId, apiCall, latencyMs}`。

---

## 7  アラート & ダッシュボード

* **Looker Studio テンプレート**

  * 過去 24 時間で `view_velocity_per_h` 上位 10 本をカード表示
  * チャンネル別成長ヒートマップ（スパークライン）
* **BigQuery スケジュールクエリ** で `view_velocity_per_h > 10 000` を検知しメール通知

---

## 8  クオータ & 上限

| リソース                 | 想定使用量              | メモ                     |
| -------------------- | ------------------ | ---------------------- |
| YouTube Data API 単位数 | ≤ 250 units / 30 分 | デフォルト 10 000 / 日の範囲内   |
| Cloud Run CPU        | ≤ 0.1 vCPU 時間 / 日  | 軽量コンテナ                 |
| BigQuery ストレージ       | +2 MB / 日          | 5 000 本 × 日次スナップショット想定 |

---

## 9  GeminiCLI 次のアクション

1. `gemini init -f project.yaml`
2. `gemini generate module fetcher --template cloud-run-go --output services/fetcher`
3. `gemini deploy all --env prod`

---

*以上で設計書は完了です*
