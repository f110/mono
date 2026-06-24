## Go guideline

- CLIライブラリ: `go.f110.dev/mono/go/cli`
- エラーハンドリング: `go.f110.dev/xerrors`
    - 一番深いエラー発生箇所で `WithStack` によりスタックトレースを付与
    - エラーを含むログを出力する際は `go.f110.dev/mono/go/logger/slogger.E()` を使用する
- 関数の引数で複数の値をstructでまとめることは極力避ける
- ファイルは無用に分割しない
- 値のポインターを得る場合はnewを使う

### テストのスタイル

- アサーション: 新規コードは `go.f110.dev/mono/go/testing/assertion` を優先 (`MustNoError`, `MustError`, `Equal`, `True`/`False` 等)。`stretchr/testify` は既存ファイルの慣習に合わせる時のみ。
- CLIツールでは、ファイルシステムに触らない純粋関数 (`migrateSource(src) → out` のような) をエントリポイントから分離してテストしやすくする
- FSの挙動自体を検証したい時は `t.TempDir()` でフィクスチャを作る。
