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
-- videos テーブル: 動画のトレンドデータ
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `${PROJECT_ID}.youtube.videos` (
  -- 識別子
  video_id STRING NOT NULL OPTIONS(description="YouTube動画ID"),
  channel_id STRING NOT NULL OPTIONS(description="YouTubeチャンネルID"),
  
  -- 基本情報
  title STRING OPTIONS(description="動画タイトル"),
  description STRING OPTIONS(description="動画の説明文"),
  channel_title STRING OPTIONS(description="チャンネル名"),
  
  -- 時刻情報
  published_at TIMESTAMP OPTIONS(description="動画の公開日時"),
  captured_at TIMESTAMP NOT NULL OPTIONS(description="データ取得日時"),
  
  -- カテゴリとタグ
  category_id STRING OPTIONS(description="YouTubeカテゴリID"),
  tags ARRAY<STRING> OPTIONS(description="動画タグのリスト"),
  
  -- 統計情報
  view_count INT64 OPTIONS(description="再生回数"),
  like_count INT64 OPTIONS(description="高評価数"),
  comment_count INT64 OPTIONS(description="コメント数"),
  favorite_count INT64 OPTIONS(description="お気に入り数"),
  
  -- 動画メタデータ
  duration STRING OPTIONS(description="動画の長さ (ISO 8601形式)"),
  definition STRING OPTIONS(description="動画の画質 (hd/sd)"),
  caption STRING OPTIONS(description="字幕の有無"),
  licensed_content BOOL OPTIONS(description="ライセンスコンテンツフラグ"),
  
  -- 地域情報
  region_code STRING OPTIONS(description="取得地域コード (JP)"),
  
  -- ショート動画判定
  is_short BOOL OPTIONS(description="ショート動画フラグ")
)
PARTITION BY DATE(captured_at)
CLUSTER BY channel_id, video_id
OPTIONS(
  description="YouTube動画のトレンドデータを格納するテーブル",
  partition_expiration_days=365  -- 1年後に自動削除
);

-- ----------------------------------------------------------------------------
-- channels テーブル: チャンネル情報
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `${PROJECT_ID}.youtube.channels` (
  -- 識別子
  channel_id STRING NOT NULL OPTIONS(description="YouTubeチャンネルID"),
  
  -- 基本情報
  title STRING OPTIONS(description="チャンネル名"),
  description STRING OPTIONS(description="チャンネルの説明"),
  custom_url STRING OPTIONS(description="カスタムURL"),
  country STRING OPTIONS(description="チャンネルの国"),
  
  -- 時刻情報
  published_at TIMESTAMP OPTIONS(description="チャンネル作成日時"),
  captured_at TIMESTAMP NOT NULL OPTIONS(description="データ取得日時"),
  
  -- 統計情報
  view_count INT64 OPTIONS(description="総再生回数"),
  subscriber_count INT64 OPTIONS(description="チャンネル登録者数"),
  video_count INT64 OPTIONS(description="公開動画数"),
  
  -- プレイリスト
  uploads_playlist_id STRING OPTIONS(description="アップロード動画のプレイリストID"),
  
  -- サムネイル
  thumbnail_url STRING OPTIONS(description="チャンネルサムネイルURL"),
  
  -- その他のメタデータ
  topic_categories ARRAY<STRING> OPTIONS(description="トピックカテゴリ"),
  keywords ARRAY<STRING> OPTIONS(description="チャンネルキーワード")
)
PARTITION BY DATE(captured_at)
CLUSTER BY channel_id
OPTIONS(
  description="YouTubeチャンネルの情報を格納するテーブル",
  partition_expiration_days=365  -- 1年後に自動削除
);

-- ----------------------------------------------------------------------------
-- video_trends ビュー: 動画のトレンド分析用
-- ----------------------------------------------------------------------------
CREATE OR REPLACE VIEW `${PROJECT_ID}.youtube.video_trends` AS
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
  `${PROJECT_ID}.youtube.videos`
WHERE
  captured_at >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 30 DAY);

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
  `${PROJECT_ID}.youtube.videos`
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