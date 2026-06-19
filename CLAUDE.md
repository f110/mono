# CLAUDE.md

## 一般ガイドライン

- このリポジトリはモノレポになっていて複数のプロジェクトが1つのリポジトリになっている
- リポジトリの構造はlanguage agnosticになっているのでプロジェクトごとの明確なディレクトリはない
- BUILD.bazel ファイルの更新は `make update-deps` で行う
- ビルドは `bazel build` で行う

## ディレクトリ構造

### 言語別ソース

- `go/`: Goコードの本体。大半のプロジェクトはここにある。
    - `go/cmd/`: 各バイナリのエントリポイント (`main` パッケージ)。1ディレクトリ1コマンド。
    - `go/<pkg>/`: ライブラリ・ドメインコード。`build/` `k8s/` `git/` `notion/` `storage/` などプロジェクトや関心ごとにパッケージ化。
    - `go/api/`: Kubernetes CRD の型定義 (`*v1alpha1`)。
    - `go/testing/`: テスト補助 (`assertion` など)。
    - Go固有のガイドラインは `go/CLAUDE.md` を参照。
- `ts/`: TypeScript。pnpm workspace + turbo 構成。`ts/apps/` がアプリ、`ts/packages/` が共有設定。
- `py/`: Python プロジェクト。
- `ruby/`: Ruby のスクリプト類。
- `sh/`: シェルスクリプトで実装されたコマンド。
- `cmd/`: Go以外も含むトップレベルのコマンド (`mirror-releases`, `rotarypress` 等)。

### スキーマ・定義

- `proto/`: Protocol Buffers 定義。`go/` の各プロジェクトに対応 (`build/`, `git/`, `docutil/` 等)。生成コードは各言語ディレクトリに出力。
- `sql/`: SQL スキーマ定義。
- `manifests/`: Kubernetes マニフェスト (CRD, RBAC, deploy, devcluster 等)。

### ビルド・インフラ

- `build/`: Bazel のカスタムルール・マクロ (`build/rules/` に言語別の `go/`, `ts/`, `proto/`, `container/`, `deb/` 等)。
- `containers/`: コンテナイメージのビルド定義。1ディレクトリ1イメージで、多くは `go/cmd/` のバイナリに対応。
- `third_party/`: ベンダリングした外部ソース (`universal-ctags` 等)。
- `patch/`: 依存パッケージへ当てるパッチ。
- `script/`: リポジトリ運用・ベンダリング用のスクリプト。
- `deb/`: Debian パッケージ関連。
- `docs/`: ドキュメント。
- `.build`: 自作CIツールの設定ファイル。ファイルのスキーマは `go/build/config/schema.cue`
- ルートの `MODULE.bazel` / `WORKSPACE` / `go.mod` / `Makefile` がリポジトリ全体のビルド・依存のエントリポイント。

## バグ修正・挙動変更のワークフロー

既にテストがあるパッケージで修正をする時は、テストファースト:

1. 期待する挙動を表すテストケースを既存のテストファイルに追加 (table-drivenなら同じテーブルに行を足す)
2. テストを実行し、想定通りの理由で失敗することを確認。即パスしたらバグを再現できていないのでテストを見直す
3. 実装を変更してテストをパスさせる
4. テスト全体を実行して回帰がないことを確認

## プロジェクト

- Build: 自作のCIツール @docs/build.md
