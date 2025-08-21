# 簡易版YouTube Studio 改修計画（YouTubeTrendTrackerベース・Fetcher実装範囲）

## 目的

YouTube Studioと同等の内部データは取得できないが、**YouTube Data APIで取得可能な公開情報**を用いて、外部からでもチャンネル分析できる「簡易版YouTube Studio」の元データを生成する。
本計画では、レポジトリの実装範囲を **BigQueryにデータを投入するまで** に限定する。

## 主な変更点（Fetcher部分のみ）

### 1. データ取得（Fetcher改修）

* `videos.list` の取得範囲拡張：`contentDetails`, `topicDetails` を追加
* 取得項目追加：

  * `duration_sec`（動画の再生時間を秒単位で保持、ISO8601フォーマットから変換）
* 新着は現行通り**1時間間隔**で取得（ロングテール専用スケジュールは不要）

### 2. BigQuery スキーマ（投入先テーブル）

対象: `youtube.video_trends`

* 上記追加項目を反映するカラムを追加

```sql
ALTER TABLE youtube.video_trends
ADD COLUMN duration_sec INT64,
ADD COLUMN content_details STRING,
ADD COLUMN topic_details ARRAY<STRING>;
```

* `content_details` はYouTube APIの `contentDetails` をJSONや文字列として格納
* `topic_details` は `topicDetails.topicCategories` を配列として格納

## 実装ステップ（Fetcher部分）

1. BigQuery DDL適用（追加カラム含む `video_trends` スキーマ変更）
2. Fetcher改修（`contentDetails`, `topicDetails` 取得＋`duration_sec`変換）
3. BigQuery投入ロジックに新カラムを渡す
4. スケジューラーは現行の1時間間隔設定を継続
