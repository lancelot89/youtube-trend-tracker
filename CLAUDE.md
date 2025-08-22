# CLAUDE.md — youtube-trend-tracker（Claude Code 運用ルール）

> このドキュメントは **Claude Code（以下「Claude」）に本リポジトリを安全に“管理・修正”させるための作業規約** です。人間レビュー前提の **小さなPR** を重ねる方針で運用します。

---

## 0. リポジトリ概要（役割・範囲）

* 目的: **YouTube Data API からデータ取得 → BigQuery へ保存 →（将来）Batch/定期実行 → 可視化（Looker Studio）**。
* 主技術: **Go (≥1.22推奨)** / GCP（BigQuery, Artifact Registry, Cloud Batch）/ Docker。
* 地域既定: `asia-northeast1`（東京）。変更が必要な場合はPR内で根拠を明記。
* 機密情報: **APIキーや認証ファイルを絶対にコミットしない**。`.env` と `gcloud` の ADC（Application Default Credentials）を利用。

---

## 1. Claude の基本ポリシー

1. **リスク最小化**: 破壊的変更は行わず、必ず **Issue → Branch → PR** の流れ。既存挙動を変える場合は **互換モード** を残す。
2. **スモールステップ**: 1PRの変更は **500行以下** を目安。大規模改修は設計PRを先行。
3. **説明責務**: すべてのPRに **目的／背景／検証手順／ロールバック手順** を記載。
4. **決め打ち禁止**: 既存仕様が不明瞭な場合は **診断結果をPRで提案**。推測実装はしない。
5. **再現可能性**: すべてのコマンドを **Makefile / scripts/** に集約。手順は README 更新をセット。

---

## 2. Git 運用・コミット規約

* ブランチ: `feat/*`, `fix/*`, `docs/*`, `chore/*`, `refactor/*`。
* コミット: **Conventional Commits** を採用（例: `feat(fetch): add trending videos fetcher`）。
* 署名・ユーザ設定（初回エラー対策）:

  ```bash
  git config user.name "Your Name"
  git config user.email "you@example.com"
  ```
* 大量差分や生成物（`/bin`, `/dist`, `.DS_Store`, `node_modules`, `__pycache__`, `*.zip`, `*.exe`, `*.sqlite` など）は `.gitignore` に追加。
* **機密の取り扱い**: `git-secrets` / `trufflehog` などでのスキャンを推奨。漏洩時は **履歴書き換えは人間承認必須**（`git filter-repo`）。

---

## 3. ディレクトリ/ファイル方針（存在しなければ作成）

```
.
├─ cmd/ytt/                # エントリポイント（main）
├─ internal/               # 業務ロジック（パッケージ分割）
│  ├─ youtube/             # API クライアント・型
│  ├─ bq/                  # BigQuery 書き込み層
│  └─ scheduler/           # Batch/スケジュール起動用
├─ pkg/                    # 外部公開の可能性がある汎用コード（必要なら）
├─ scripts/                # 自動化スクリプト（*.sh / *.ps1 / *.py）
├─ deploy/                 # IaC/ジョブ定義（batch-job.yaml 等）
├─ configs/                # サンプル構成（config.yaml, .env.example）
├─ .github/                # CI / Issue / PR テンプレ
├─ Makefile                # コマンド集約
└─ README.md               # セットアップと使い方
```

> 既存構成が異なる場合は **無理に合わせず**、段階的に移行。置き換え計画をPRで提案すること。

---

## 4. 環境変数・設定（標準案）

`configs/.env.example` を整備。各自 `.env` をルートに作成して読み込む。

```
YOUTUBE_API_KEY=
GCP_PROJECT_ID=
GCP_LOCATION=asia-northeast1
BQ_DATASET=youtube
BQ_TABLE=trending_videos
BATCH_REGION=asia-northeast1
# Optional: GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa.json  # 基本はADC推奨
```

Go 側は `envconfig` / `os.LookupEnv` で取得。必須値のバリデーションを実装。

---

## 5. GCP 認証・前提

* **ADC を既定**:

  ```bash
  gcloud auth login
  gcloud auth application-default login
  gcloud config set project "$GCP_PROJECT_ID"
  ```
* 有効化するAPI（必要に応じて）:

  ```bash
  gcloud services enable youtube.googleapis.com \
    bigquery.googleapis.com batch.googleapis.com \
    artifactregistry.googleapis.com
  ```
* 権限（例）: 実行ユーザ/SA に `BigQuery Job User`, `BigQuery Data Editor`, `Batch Admin` 相当。

---

## 6. ローカル実行・品質ゲート

* 共通要件: **Go ≥1.22**, **Docker ≥24**, **gcloud ≥474**, `jq ≥1.6`（任意）。
* 依存整備:

  ```bash
  go mod tidy
  ```
* Lint/Format（導入提案）: `gofumpt`, `golangci-lint`。
* テスト:

  ```bash
  go test ./...
  ```
* 実行（例）:

  ```bash
  export $(grep -v '^#' .env | xargs -d '\n')
  go run ./cmd/ytt
  ```

> **Claude への指示**: 初手は `make diag`（後述）で状況を棚卸し → 設計/差分計画を PR にまとめる。

---

## 7. Docker/Artifact Registry（WSL & macOS 向け）

### Artifact Registry（初回）

```bash
# リポジトリが無ければ作成
gcloud artifacts repositories create containers \
  --repository-format=docker \
  --location=asia-northeast1 \
  --description="youtube-trend-tracker images"

# 認証
gcloud auth configure-docker asia-northeast1-docker.pkg.dev
```

### Build & Push（タグは日付などで一意化）

```bash
TAG=$(date +%Y%m%d-%H%M%S)
IMAGE=asia-northeast1-docker.pkg.dev/$GCP_PROJECT_ID/containers/ytt-tracker:$TAG

docker build -t "$IMAGE" .
docker push "$IMAGE"
```

> **WSL の注意**: Docker Desktop の WSL2 連携を有効化。プロキシ環境では `~/.docker/config.json` を調整。

---

## 8. Cloud Batch（サンプル定義）

`deploy/batch-job.yaml`（例）:

```yaml
# deploy/batch-job.yaml
taskGroups:
  - taskSpec:
      runnables:
        - container:
            imageUri: ${IMAGE}
            entrypoint: "/app/ytt"
            commands: []
            env:
              - key: YOUTUBE_API_KEY
                value: ${YOUTUBE_API_KEY}
              - key: GCP_PROJECT_ID
                value: ${GCP_PROJECT_ID}
              - key: BQ_DATASET
                value: youtube
              - key: BQ_TABLE
                value: trending_videos
    taskCount: 1
allocationPolicy:
  instances:
    - policy:
        machineType: e2-standard-2
labels:
  app: youtube-trend-tracker
logsPolicy:
  destination: CLOUD_LOGGING
```

作成/実行（例）:

```bash
gcloud batch jobs submit ytt-$(date +%Y%m%d-%H%M%S) \
  --location=$BATCH_REGION \
  --config=deploy/batch-job.yaml
```

> 将来: Cloud Scheduler + Cloud Run Jobs / Workflows での起動も検討（別PR）。

---

## 9. BigQuery スキーマ（標準案）

> 既存テーブルがある場合は**変更しない**。差分は **migration PR** で提案。

提案例（`youtube.trending_videos`）:

```
video_id STRING, title STRING, description STRING, channel_id STRING, channel_title STRING,
published_at TIMESTAMP, captured_at TIMESTAMP,
category_id STRING, tags ARRAY<STRING>,
view_count INT64, like_count INT64, comment_count INT64, favorite_count INT64,
duration STRING, definition STRING, licensed_content BOOL,
region_code STRING
```

* 収集時刻は `captured_at = CURRENT_TIMESTAMP()` を必須にし、時系列分析可能に。
* 文字列の長文列には **SAFE.TRIM** / **LEN** ガードを実装（過大サイズの拒否/切詰め方針を決める）。

---

## 10. 実装規約（Go）

* **context** を全レイヤに伝播、タイムアウト既定 30s。
* ログは **構造化（key=value）**、PII を書かない。`log/slog` か `zerolog` 推奨。
* 外部I/O（YouTube, BQ）は **リトライ（指数バックオフ）** を実装。APIクォータ超過を考慮。
* 設定は **env → config struct** に束ねて DI。
* **ユニットテスト**: 失敗パス・境界値を少なくとも 1 件ずつ。
* **エラーハンドリング**: `errors.Join / Is / As` を活用。返却エラーは文脈付与。

---

## 11. Makefile（サンプル）

```make
.PHONY: diag setup lint test run build docker push batch

GO ?= go
REGION ?= asia-northeast1
IMAGE ?= asia-northeast1-docker.pkg.dev/$(GCP_PROJECT_ID)/containers/ytt-tracker:dev

export $(shell sed -n 's/^[A-Za-z0-9_]*=.*/&/p' .env)

diag:
	@echo "Go version:" && $(GO) version
	@echo "gcloud:" && gcloud --version | head -n1 || true
	@echo "Docker:" && docker --version || true
	@echo "Project: $(GCP_PROJECT_ID) / Region: $(REGION)"

setup:
	$(GO) mod tidy

lint:
	golangci-lint run || true

test:
	$(GO) test ./...

run:
	$(GO) run ./cmd/ytt

build:
	$(GO) build -o bin/ytt ./cmd/ytt

docker:
	docker build -t $(IMAGE) .

push:
	docker push $(IMAGE)

batch:
	gcloud batch jobs submit ytt-`date +%Y%m%d-%H%M%S` \
	  --location=$(BATCH_REGION) \
	  --config=deploy/batch-job.yaml
```

---

## 12. WSL / macOS 補足

* **WSL**: Windows 側の認証ブラウザ起動で詰まる場合は `gcloud auth login --update-adc --no-launch-browser` + 手動コード入力。
* **macOS**: Homebrew 管理推奨。`brew install go jq`、`gcloud` は公式インストーラ。
* **Docker 権限**: ソケット権限問題は `docker context ls` で確認、WSL では Docker Desktop 連携を必ず有効化。

---

## 13. CI（将来追加の雛形・任意）

* GitHub Actions: `go test`, `gofumpt`/`golangci-lint`, `docker build/push`。
* OIDC で GCP 連携（Secrets 最小化）。CI 追加は別PRで段階導入。

---

## 14. Claude の標準ワークフロー

1. **診断**: 既存コード/README/構成を読んで `docs/ADR-000-現状調査.md` に要約を作成。
2. **提案**: 「やること・代替案・影響・試験観点」を Issue/PR 説明に記載。
3. **実装**: `scripts/` と `Makefile` にコマンド集約、README 更新をセット。
4. **検証**: ローカル実行・`go test`・（あれば）ステージ環境でのドライラン。
5. **リリース**: 小さなPRでマージ。破壊的変更は **メジャー** と明記。

---

## 15. 最初の改善タスクリスト（Claude 向け）

* [ ] `configs/.env.example` を整備（本ファイルのキーと同期）。
* [ ] `Makefile` を追加（上記サンプルをベースに最小コマンド実装）。
* [ ] `deploy/batch-job.yaml` を追加（上記サンプル）。
* [ ] README に **セットアップ（ADC, API有効化, Build/Run）** を追記。
* [ ] BigQuery スキーマを `docs/schema.sql` として提案（既存があれば差分比較）。
* [ ] `scripts/` に `local_run.sh`, `build_and_push.sh` を追加（WSL/mac両対応）。
* [ ] ログ/エラーの統一（`slog` 導入 or 現状準拠）。

---

## 16. PR テンプレ（.github/pull\_request\_template.md）

```md
## 概要
- 目的 / 背景:
- このPRでやったこと:

## 変更点
-

## 動作確認
- コマンド:
- 期待結果:

## 影響範囲
- 互換性:
- 秘密情報:

## ロールバック
- 手順:
```

---

## 17. 重要な禁止事項

* 機密の書き込み・コミット（APIキー、OAuth秘密など）。
* 既存テーブルの破壊的スキーマ変更（事前合意なし）。
* 大量ファイルの一括整形（フォーマット方針確定前）。

---

## 18. 参考コマンド（チートシート）

```bash
# env 読み込み（GNU xargs 前提）
export $(grep -v '^#' .env | xargs -d '\n')

# GCP 設定
gcloud config set project "$GCP_PROJECT_ID"

# BigQuery 簡易検証
bq query --use_legacy_sql=false 'SELECT 1'

# Artifact Registry 認証
gcloud auth configure-docker asia-northeast1-docker.pkg.dev

# Cloud Batch 実行
make docker push batch
```

---

### 最後に

Claude は **保守性・安全性・説明可能性** を最優先に、上記ポリシーに従って作業してください。設計判断が必要な場合は **小さな提案PR** から始めます。
