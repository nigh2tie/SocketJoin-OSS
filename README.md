# SocketJoin

> **つなぐだけで、みんな参加。**
> イベント・セミナー・ウェビナー向けの、インストール不要なリアルタイム投票・クイズプラットフォーム。

SocketJoin は、ホストの操作に合わせて参加者の画面がリアルタイムに切り替わる、インタラクティブなイベント支援ツールです。WebSocket を活用し、1秒未満の低遅延でアンケート集計やクイズの進行が可能です。

## 🚀 クイックスタート (Docker)

Docker Compose を使用して、すぐにローカルで試用できます。

### 1. 設定ファイルの作成
```bash
cp .env.example .env
```
`.env` を編集し、`POSTGRES_PASSWORD` を設定してください。

### 2. 起動
```bash
make up
```
このコマンドで DB、Redis、API、フロントエンドがすべて立ち上がり、マイグレーションも自動適用されます。

### 3. アクセス
- **ホスト管理画面**: [http://localhost:3000/host](http://localhost:3000/host) （`WEB_PORT` で変更可能）
- **API サーバー**: [http://localhost:3000/api](http://localhost:3000/api) （`WEB_PORT` 経由。直接アクセスは `APP_PORT`）

---

## 🌐 Web へのデプロイ

外部からアクセス可能な Web サーバーへデプロイする場合は、以下の設定を確認してください。

1. **HTTPS 通信**: セキュリティおよび WebSocket の安定のため、リバースプロキシ（Nginx, Traefik, Cloudflare 等）を介して SSL/TLS を適用してください。
2. **APP_ENV**: `.env` で `APP_ENV=production` を設定すると、認証 Cookie に `Secure` フラグが付与されます。
3. **FRONTEND_URL**: 実際にアクセスするドメイン（例: `https://join.example.com`）を `.env` に設定し、CORS を許可してください。

## ⚙️ 主な環境変数 (`.env`)

| 変数 | 説明 | デフォルト値 |
| :--- | :--- | :--- |
| `POSTGRES_PASSWORD` | DB のパスワード (**設定必須**) | `change_me` |
| `POSTGRES_PORT` | DB の公開ポート (Docker 使用時) | `5432` |
| `REDIS_PORT` | Redis の公開ポート (Docker 使用時) | `6379` |
| `APP_PORT` | API サーバーの公開ポート (Docker 使用時) | `8080` |
| `WEB_PORT` | Web サーバーの公開ポート (Docker 使用時) | `3000` |
| `FRONTEND_URL` | CORS/WebSocket 許可オリジン | `http://localhost:3000` |
| `APP_ENV` | `production` 指定で Secure Cookie 有効 | `development` |
| `NG_WORDS` | 追加のNGワード (カンマ区切り) | (空) |
| `POLL_RETENTION_DAYS` | 投票データの保持期間（日数） | `90` |

## 🗄️ データ保持ポリシー

SocketJoin はサーバー起動後、**毎日 1 回**バックグラウンドでデータ保持ジョブを実行します。

**削除対象**: `POLL_RETENTION_DAYS`（デフォルト 90 日）を超えた `polls` レコード。CASCADE により関連する `options`・`votes`・`vote_submissions` も同時に削除されます。

**削除されないもの**: `events` テーブルのレコードは削除されません。ただし、削除された poll を参照していた `events.current_poll_id` は自動的に `NULL` にリセットされます。

> **注意**: イベントが残っていても、保持期間を超えた投票結果・集計データは失われます。長期保存が必要な場合は、ホスト管理画面の CSV エクスポート機能を使用してください。

## 🛠️ 開発・運用コマンド

`Makefile` を使用して各操作を行えます。

- `make up`: 全サービスをバックグラウンドで起動
- `make down`: 全サービスを停止
- `make logs`: ログをストリーミング表示
- `make migrate-up`: マイグレーションを適用
- `make smoke-test`: 基本的なイベントフローのテストを実行

## 📥 CSV 一括インポート

ホスト管理画面の `CSVテンプレート` から、説明付きテンプレートをダウンロードできます。

- `poll_type` は `survey` または `quiz`
- `option_1` と `option_2` は必須。追加の選択肢は左から詰めて記入
- `max_selections` は未指定なら `1`
- `points` と `correct_options` は `quiz` のときに使用
- `correct_options` は `option_1` を `1` とする番号指定で、複数正解は `1|3` のように記入

例:

```csv
poll_title,poll_type,max_selections,option_1,option_2,option_3,points,correct_options
好きな色は？,survey,1,赤,青,緑,,
2 + 2 は？,quiz,1,3,4,5,10,2
正しいものをすべて選べ,quiz,2,地球は丸い,空は赤い,水は液体,20,1|3
```

## 🏗️ 技術スタック

- **Backend**: Go (chi, Gorilla WebSocket, pgx/v5)
- **Frontend**: SvelteKit (Tailwind CSS)
- **Infrastructure**: Nginx, PostgreSQL, Redis
- **Migration**: golang-migrate

---

## 📄 ライセンス

[LICENSE](LICENSE) ファイルを参照してください。
