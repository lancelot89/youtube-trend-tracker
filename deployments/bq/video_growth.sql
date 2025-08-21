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