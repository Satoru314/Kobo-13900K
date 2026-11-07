# Kobo-13900K

AI に開発タスクを渡し、キューで順に実行させて、PR 作成とレビュー対応まで任せるための自分用アプリケーション。

設計: [docs/design.md](docs/design.md)

## セットアップ

- `claude` / `opencode` / `gh` / `git` / `playwright-mcp` を入れる（kobo はインストールしない）
- `cp .env.example .env` して、DB とパスを自分の環境に合わせる
- `make migrate` で DB を作る
- `make check` でコマンドと設定を確認する（足りなければ起動しない）

AI モデルは `ai.json` に書く。モデルを増やすときは `models` に1エントリ（`off_peak` を含む）足すだけで、DB の変更は要らない。
