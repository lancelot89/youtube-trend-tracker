# IAM 権限マトリクス

> このドキュメントは YouTube Trend Tracker プロジェクトで使用される全サービスアカウントと、それらに必要な最小権限を定義しています。

## サービスアカウント一覧

### 1. trend-tracker-sa

**用途**: Cloud Run サービスの実行用サービスアカウント

**メールアドレス**: `trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com`

**必要な権限**:

| ロール | リソース | 理由 | 必須 |
|-------|---------|------|------|
| `roles/artifactregistry.reader` | プロジェクト | Artifact Registry からプライベートコンテナイメージを Pull するため | ✅ |
| `roles/bigquery.dataEditor` | プロジェクト | BigQuery テーブルへのデータ書き込み | ✅ |
| `roles/bigquery.jobUser` | プロジェクト | BigQuery ジョブ（INSERT, CREATE TABLE等）の実行 | ✅ |
| `roles/secretmanager.secretAccessor` | Secret: `youtube-api-key` | YouTube Data API キーへのアクセス | ✅ |

### 2. scheduler-sa

**用途**: Cloud Scheduler から Cloud Run サービスを呼び出すためのサービスアカウント

**メールアドレス**: `scheduler-sa@${PROJECT_ID}.iam.gserviceaccount.com`

**必要な権限**:

| ロール | リソース | 理由 | 必須 |
|-------|---------|------|------|
| `roles/run.invoker` | Cloud Run Service: `${SERVICE_NAME}` | Cloud Run サービスの HTTP エンドポイントを呼び出すため | ✅ |

## 権限の最小化原則

### ✅ 実装済みの最小権限

1. **リソースレベルの権限付与**
   - Secret Manager へのアクセスは特定のシークレット（`youtube-api-key`）のみに制限
   - Cloud Run Invoker 権限は特定のサービスのみに制限

2. **不要な権限の排除**
   - `roles/owner` や `roles/editor` などの広範な権限は使用しない
   - デフォルトのサービスアカウントは使用しない

### ⚠️ 検討事項

1. **BigQuery 権限の細分化**
   - 現在: `roles/bigquery.dataEditor` をプロジェクトレベルで付与
   - 改善案: 特定のデータセット（`youtube`）のみへの権限に制限可能
   ```bash
   # データセットレベルの権限付与例
   bq add-iam-policy-binding \
     --member="serviceAccount:trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
     --role="roles/bigquery.dataEditor" \
     ${PROJECT_ID}:youtube
   ```

2. **監査ログの有効化**
   - サービスアカウントのアクティビティを監視するため、Cloud Audit Logs の有効化を推奨

## セットアップ手順

### 自動セットアップ（推奨）

```bash
# 全サービスアカウントの作成と権限設定を一括実行
./scripts/setup-service-accounts.sh <project_id> <region> <service_name>
```

### 手動セットアップ

個別にサービスアカウントを作成・設定する場合:

```bash
# 1. trend-tracker-sa の作成
gcloud iam service-accounts create trend-tracker-sa \
  --display-name="YouTube Trend Tracker Service Account"

# 2. scheduler-sa の作成
gcloud iam service-accounts create scheduler-sa \
  --display-name="Cloud Scheduler Service Account"

# 3. 権限の付与（上記マトリクス参照）
# ...
```

## 権限の検証

### 現在の権限を確認

```bash
# trend-tracker-sa の権限確認
gcloud projects get-iam-policy ${PROJECT_ID} \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --format="table(bindings.role)"

# scheduler-sa の権限確認
gcloud run services get-iam-policy ${SERVICE_NAME} \
  --region=${REGION} \
  --format=json
```

### 不要な権限の削除

```bash
# 例: 過剰な権限を削除
gcloud projects remove-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/editor"  # 削除すべき過剰な権限
```

## セキュリティベストプラクティス

1. **定期的な権限レビュー**
   - 四半期ごとに権限の妥当性を確認
   - 未使用のサービスアカウントを削除

2. **サービスアカウントキーの管理**
   - 可能な限りキーレスアクセス（Workload Identity）を使用
   - キーを作成する場合は定期的なローテーションを実施

3. **最小権限の原則**
   - 必要最小限の権限のみを付与
   - プロジェクトレベルよりリソースレベルの権限を優先

4. **監査とモニタリング**
   - Cloud Audit Logs でアクティビティを記録
   - 異常なアクセスパターンをアラート設定

## トラブルシューティング

### よくある権限エラーと対処法

| エラーメッセージ | 原因 | 対処法 |
|-----------------|------|--------|
| `Permission 'secretmanager.versions.access' denied` | Secret Manager へのアクセス権限不足 | `setup-service-accounts.sh` を実行 |
| `Permission 'run.routes.invoke' denied` | Cloud Run Invoker 権限不足 | scheduler-sa に `roles/run.invoker` を付与 |
| `Permission 'bigquery.tables.create' denied` | BigQuery テーブル作成権限不足 | `roles/bigquery.dataEditor` と `roles/bigquery.jobUser` の両方が必要 |

## 更新履歴

- 2024-01-XX: 初版作成
- 2024-01-XX: setup-service-accounts.sh スクリプトによる自動化対応