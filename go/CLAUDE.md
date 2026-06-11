## Go guideline

- CLIライブラリ: `go.f110.dev/mono/go/cli`
- エラーハンドリング: `go.f110.dev/xerrors`
    - 一番深いエラー発生箇所で `WithStack` によりスタックトレースを付与
    - エラーを含むログを出力する際は `go.f110.dev/mono/go/logger/slogger.E()` を使用する
- 関数の引数で複数の値をstructでまとめることは極力避ける
- ファイルは無用に分割しない
