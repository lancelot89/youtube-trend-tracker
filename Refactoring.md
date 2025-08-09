# YouTubeトレンドトラッカー｜修正すべき点（AI実装向けタスクリスト）

> 目的: **最小で Cloud Run→（認証）→ BigQuery まで通す**。その後に冪等性・運用最適化を段階的に追加する。
>
> 想定スタック: Go 1.22/1.23, Cloud Run, Cloud Scheduler(OIDC), BigQuery（日次パーティション/クラスタリング）, Artifact Registry。

---

## ✅ 優先度: 高（先にやる）

### \[1] ビルド不能な `...` の除去・実装補完

* **対象**: `internal/youtube/client.go`, `internal/storage/bq.go`, `cmd/fetcher/main.go`, `Dockerfile`, `scripts/*`
* **やること**:

  * すべての `...`（プレースホルダ）を削除し、機能するコードを実装
  * 最小限のフロー（YouTube API取得→整形→BQ挿入）を動作させる
* **ポイント**: Gemini CLI編集時はファイル単位で修正を進め、`go build ./...` が通ることを都度確認

### \[2] Go バージョン統一

* **現状**: `go.mod`= `go 1.24.3`、Docker= `golang:1.22-alpine`
* **やること**:

  * `go.mod` と Dockerfile の Go バージョンを同一に
  * バージョン差異で発生する依存ライブラリ不一致を防ぐ

### \[3] Cloud Run 設定受け渡し方式の修正（ConfigMap 廃止）

* **課題**: K8s前提の `configMapKeyRef` はCloud Runで使えない
* **修正案**: 環境変数や Secret Manager 経由で設定を受け渡す
* **Gemini CLIでの留意点**: YAML編集はインデント崩れやスペース/タブ混在を避ける

### \[4] コンテナレジストリ表記の統一

* Artifact Registry推奨。`gcr.io` 参照部分を一括置換
* CLI編集時は`service.yaml`とREADMEをセットで修正

### \[5] Cloud Scheduler→Cloud Run 呼び出しのOIDC化

* SA作成、`oidcToken`設定、`audience`一致の確認
* CLI上でのJSON編集は構文ミスを避けるため整形ツールを利用

### \[6] BigQuery スキーマ運用改善

* パーティション（日次）＋クラスタリング（video\_id）を設定
* CLI修正時は`schema.json`を直接編集せず、`bq mk`コマンドで適用する方針

### \[7] 重複対策・冪等性確保

* `ts, video_id` をキーにした MERGE運用またはビューの一意化ロジックを追加

### \[8] YouTube API 呼び出しの堅牢化

* ページング・指数バックオフ・NULL安全化を必須化
* CLI編集時はテスト関数を追加し、API制限時の挙動も確認

### \[9] エミュレータ/本番切替

* `BIGQUERY_EMULATOR_HOST` の有無で認証処理を分岐
* `.env.example` と README を更新し、CLIからの環境変数差し替え手順を明記

### \[10] 構造化ログ（JSON）化

* Cloud Loggingでのフィルタ・解析を容易にするためJSON出力に統一
* CLI実装時は構造体定義とマーシャル処理を共通化

---

## 🧱 実装サンプル（Gemini CLI参考用）

* Dockerfileのマルチステージビルド例
* YouTube API クライアント骨子
* BigQuery作成コマンド例
* いずれも直接貼り付け可能な形式で記述

---

## 🔧 中〜低優先

* アーキ図と実装の差異解消（READMEに反映）
* 不要コードの削除、テストコード作成

---

## ✅ 受け入れ基準（Definition of Done）

* Dockerビルド成功＆Cloud Runデプロイ可能
* SchedulerからOIDC認証で正常応答
* BigQueryに日次パーティション・クラスタリング済みデータが投入される
* 重複行が論理的に排除される
* Cloud Loggingで主要フィールドのフィルタ・分析が可能
