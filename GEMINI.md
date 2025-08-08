GEMINI.md — AI Collaboration Contract
Scope
新規ソフトウェアプロジェクトのセットアップから実装・運用まで、Gemini と共同作業するためのガイドラインを規定する。Gemini は本ファイルと同じ階層以下のコンテキストにのみ適用される。

General Rules
言語: 返答は日本語、コード内コメントは英語。

トーン: 丁寧だが簡潔。「です・ます」調は不要。あいさつ・謝辞は省く。

フォーマット: 見出しは ## で統一。箇条書きは - を使用。

コード: go fmt && gofumpt に準拠。diff は ```patch ブロックで提示。

出力長: 400 行以内。不要な背景説明は避け、“次に人間がコピペできる” 情報のみ返す。

セキュリティ: Secret 値（パスワード、APIキー等）そのものは、決してプロンプトやコード例に含めない。常にプレースホルダー (<SECRET_NAME>) を使用する。

PROTOCOL:INIT_PROJECT
目的: 新規プロジェクトをゼロから scaffold する。

入力
name: <プロジェクト名>
language: <主要言語>
features:
  - name: <機能名1>
    type: <api|batch|middleware>
    brief: <機能の概要>
  - name: <機能名2>
    type: <api|batch|middleware>
    brief: <機能の概要>

出力
project.yaml: モジュール、Secret プレースホルダー、リージョンなどメタ情報。

README.md: 300 文字以内の概要 + 最低 3 行の Quick Start。

repo_tree: ルートからのツリー表示 (最初の 3 層)。

next_actions: 人間が次に実施すべき 3 ステップ。

PROTOCOL:ADD_FEATURE
目的: 既存プロジェクトへ新機能・モジュールを追加。

入力
feature: <機能名>
impact_area: <directory/module>
brief: <やりたいこと 1–2 文>

出力
plan_table: 以下のカラムを持つマークダウンテーブル。
| Task | File(s) To Modify | Assignee (Gemini/Human) | Notes |
| :--- | :--- | :--- | :--- |

diff: 主要ファイルの差分を ```patch で提示。変更が多い場合はファイル名のみ列挙。

PROTOCOL:EXPLAIN_ERROR
目的: エラー・失敗した CI の原因を特定。

入力
error_log: |
  <stderr / stacktrace>

出力
summary: 200 字以内の要約。

root_causes: 箇条書きで最大 3 点。

fix_steps: 手順を箇条書き。コード例は ``` ブロック内に。

PROTOCOL:REFACTOR
目的: コード品質・構成を改善。

入力
file: <path/to/file.go>
goals:
  - <目的1>
  - <目的2>

出力
diff: ```patch 形式の差分。

risk: 箇条書きで挙動への影響を列挙。

verification_steps: 変更が安全であることを人間が確認するための手順を箇条書き。

Coding Style & Repository Layout
Go
変数: mixedCaps。定数: SCREAMING_SNAKE。

エラー: fmt.Errorf("%w", err) でラップ。

コンテキストは必ず ctx 第一引数へ渡す。

YAML / JSON
2 スペースインデント。ダブルクォート。

Markdown
見出し直後に空行を入れる。

Repository Layout (初期値)
.
├── cmd/            # Entry points
├── internal/       # ドメインロジック
├── configs/        # YAML, JSON settings
├── deployments/    # IaC manifests
├── scripts/        # Helper scripts
└── test/           # Unit tests
