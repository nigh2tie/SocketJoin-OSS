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
- **ホスト管理画面**: [http://localhost:3000/host](http://localhost:3000/host)
- **API サーバー**: [http://localhost:3000/api](http://localhost:3000/api)

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
| `FRONTEND_URL` | CORS/WebSocket 許可オリジン | `http://localhost:3000` |
| `APP_ENV` | `production` 指定で Secure Cookie 有効 | (空) |
| `NG_WORDS` | 追加のNGワード (カンマ区切り) | (空) |

## 🛠️ 開発・運用コマンド

`Makefile` を使用して各操作を行えます。

- `make up`: 全サービスをバックグラウンドで起動
- `make down`: 全サービスを停止
- `make logs`: ログをストリーミング表示
- `make migrate-up`: マイグレーションを適用
- `make smoke-test`: 基本的なイベントフローのテストを実行

## 🏗️ 技術スタック

- **Backend**: Go (chi, Gorilla WebSocket, sqlx)
- **Frontend**: SvelteKit (Vanilla CSS, Lucide Icons)
- **Infrastructure**: Nginx, PostgreSQL, Redis
- **Migration**: golang-migrate

---

## 📄 ライセンス

[LICENSE](LICENSE) ファイルを参照してください。

## 🛡️ セキュリティ

脆弱性を発見した場合は、GitHub の Security Advisory を通じて報告してください。
