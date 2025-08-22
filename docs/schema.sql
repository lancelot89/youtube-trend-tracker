-- ============================================================================
-- YouTube Trend Tracker BigQuery Schema
-- ============================================================================
-- このファイルはYouTube Trend Trackerで使用するBigQueryテーブルの
-- スキーマ定義を管理します。
--
-- データセット: youtube
-- テーブル: videos, channels
-- ============================================================================

-- ----------------------------------------------------------------------------
-- データセットの作成
-- ----------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS `${PROJECT_ID}.youtube`
OPTIONS(
  description="YouTube trend data storage",
  location="asia-northeast1"
);

-- ----------------------------------------------------------------------------
-- video_trends テーブル: 動画のトレンドデータ
-- ----------------------------------------------------------------------------
-- 実際のコード(internal/storage/bq.go)で定義されているスキーマ
CREATE TABLE IF NOT EXISTS `${PROJECT_ID}.youtube.video_trends` (
  -- 日付と識別子
  dt DATE NOT NULL OPTIONS(description="スナップショット日付"),
  channel_id STRING NOT NULL OPTIONS(description="YouTubeチャンネルID"),
  video_id STRING NOT NULL OPTIONS(description="YouTube動画ID"),
  
  -- 基本情報
  title STRING OPTIONS(description="動画タイトル"),
  channel_name STRING OPTIONS(description="チャンネル名"),
  tags ARRAY<STRING> OPTIONS(description="動画タグのリスト"),
  is_short BOOL OPTIONS(description="ショート動画フラグ"),
  
  -- 統計情報
  views INT64 OPTIONS(description="再生回数"),
  likes INT64 OPTIONS(description="高評価数"),
  comments INT64 OPTIONS(description="コメント数"),
  
  -- 時刻情報
  published_at TIMESTAMP OPTIONS(description="動画の公開日時"),
  created_at TIMESTAMP NOT NULL OPTIONS(description="データ取得日時"),
  
  -- 追加メタデータ
  duration_sec INT64 OPTIONS(description="動画の長さ（秒）"),
  content_details STRING OPTIONS(description="コンテンツ詳細"),
  topic_details ARRAY<STRING> OPTIONS(description="トピック詳細")
)
PARTITION BY dt  -- dtフィールドでパーティショニング
CLUSTER BY channel_id, video_id
OPTIONS(
  description="YouTube動画のトレンドデータを格納するテーブル"
);

-- ----------------------------------------------------------------------------
-- 注意: 現在の実装ではvideo_trendsテーブルのみ使用
-- channelsテーブルは将来の拡張用（未実装）
-- ----------------------------------------------------------------------------

-- ----------------------------------------------------------------------------
-- video_trends_analysis ビュー: 動画のトレンド分析用
-- ----------------------------------------------------------------------------
CREATE OR REPLACE VIEW `${PROJECT_ID}.youtube.video_trends_analysis` AS
SELECT
  video_id,
  channel_id,
  title,
  published_at,
  captured_at,
  view_count,
  like_count,
  comment_count,
  -- 前回との差分
  LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at) AS prev_view_count,
  LAG(like_count) OVER (PARTITION BY video_id ORDER BY captured_at) AS prev_like_count,
  -- 増加率
  SAFE_DIVIDE(
    view_count - LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at),
    LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at)
  ) * 100 AS view_growth_rate,
  -- 時間あたりの増加数
  SAFE_DIVIDE(
    view_count - LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at),
    TIMESTAMP_DIFF(captured_at, LAG(captured_at) OVER (PARTITION BY video_id ORDER BY captured_at), HOUR)
  ) AS views_per_hour,
  region_code
FROM
  `${PROJECT_ID}.youtube.video_trends`
WHERE
  created_at >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 30 DAY);

-- ----------------------------------------------------------------------------
-- daily_summary ビュー: 日次サマリー
-- ----------------------------------------------------------------------------
CREATE OR REPLACE VIEW `${PROJECT_ID}.youtube.daily_summary` AS
SELECT
  DATE(captured_at) AS date,
  region_code,
  COUNT(DISTINCT video_id) AS unique_videos,
  COUNT(DISTINCT channel_id) AS unique_channels,
  SUM(view_count) AS total_views,
  SUM(like_count) AS total_likes,
  SUM(comment_count) AS total_comments,
  AVG(view_count) AS avg_views,
  MAX(view_count) AS max_views,
  ARRAY_AGG(
    STRUCT(video_id, title, view_count) 
    ORDER BY view_count DESC 
    LIMIT 10
  ) AS top_10_videos
FROM
  `${PROJECT_ID}.youtube.video_trends`
GROUP BY
  date, region_code;

-- ----------------------------------------------------------------------------
-- インデックス（必要に応じて追加）
-- ----------------------------------------------------------------------------
-- BigQueryは自動的にインデックスを管理しますが、
-- 検索パフォーマンスを向上させるために検索インデックスを作成できます

-- 例: video_idでの高速検索用
-- CREATE SEARCH INDEX video_search_idx
-- ON `${PROJECT_ID}.youtube.videos`(video_id, title, description);

-- ----------------------------------------------------------------------------
-- マイグレーション履歴
-- ----------------------------------------------------------------------------
-- 2024-01-XX: 初版作成
-- 2024-01-XX: is_shortカラムを追加
-- 2024-01-XX: パーティショニングとクラスタリングを追加