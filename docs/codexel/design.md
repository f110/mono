# Codexel — 設計ドキュメント

Status: Draft
Refs: #121, #122

## 1. 目的とスコープ

ローカルで動いているコーディングエージェントと、エージェントが作業するリポジトリをブラウザから管理する。

- エージェントは herdr の中で動いているので、herdr から見つけて操作する
- リポジトリの管理・操作は lightjj の UI/操作体系を再実装する
- リポジトリブラウザからコードレビューできる。レビューは crit の機能を踏襲する。crit とのファイル形式の互換は持たない
- コードに加えてコミットメッセージもレビューできる
- コミットとエージェントを `Agent` / `Agent-Session` Trailer で紐付ける
- jj-pr の PR 作成を UI から実行できる

ソフトウェアは 1 バイナリで、フロントエンドを embed し、起動するだけで完全に動作する。lightjj・crit・jj-pr のバイナリには依存しない。
外部コマンドとして依存するのは `jj` と `gh` (トークン取得のみ) で、herdr にはソケット API で接続する。

スコープ外: 認証。既定では loopback でのみ待ち受ける (§9.3)。

## 2. 全体構成

```
  Browser (React SPA, embed)        Agent (Claude Code 等)
     │  Connect API                    │  MCP (§5.4)
     ▼                                 ▼
  ┌──────────────────────── codexel (1 binary) ────────────────────────┐
  │  agent      ── herdr socket API (~/.config/herdr/herdr.sock)       │
  │  repo       ── jj CLI (command builder + parser)                   │
  │  review     ── レビューファイル (~/.config/codexel/reviews/)       │
  │  pr         ── jj-pr のロジック (go/jujutsu に切り出す) ── GitHub API │
  └────────────────────────────────────────────────────────────────────┘
```

- フロントエンドとの通信は Connect API に統一する。lightjj の SSE (`/api/events`) に相当する変更通知は server streaming RPC にする
- 状態は持たないことを基本にする。jj のリポジトリと herdr のセッションを正とし、Codexel が持つのは設定ファイル (§4.1) と
  レビュー (§5.1) だけにする

### 2.1 パッケージ構成

パッケージは提供するもので分ける。

| パス | 提供するもの |
|---|---|
| `go/cmd/codexel` | エントリポイント。フラグ処理とサーバー起動 |
| `go/codexel/server` | Connect API と MCP のハンドラ、静的ファイル配信 |
| `go/codexel/static` | embed したフロントエンド (`//go:embed`) |
| `go/codexel/review` | レビューの保存と、リビジョン書き換え時のコメントの追従 |
| `go/herdr` | herdr ソケット API のクライアント。Codexel 以外からも使える汎用クライアントにする |
| `go/jujutsu` | jj のコマンドビルダーと出力パーサー (log / diff / bookmark / op log ...) と、`go/cmd/jj-pr` から切り出した PR 作成ロジック (§7.1)。純粋関数と Runner に分ける |
| `proto/codexel` | API 定義 |
| `ts/apps/codexel` | フロントエンド (React + MUI + Connect、`ts/apps/issue` と同じ構成) |

`go/jujutsu` はリポジトリ内に既存のパッケージがないことを `bazel query` で確認してから作る。

## 3. エージェント管理 (#123)

### 3.1 herdr との接続

herdr はソケット (`$HERDR_SOCKET_PATH`、既定 `~/.config/herdr/herdr.sock`) で JSON の request/response API を提供している
(protocol 22, `herdr api schema --json`)。CLI (`herdr agent ...`) を毎回起動せず、ソケットに直接接続する。

| Codexel の機能 | herdr API |
|---|---|
| 一覧と状態表示 | `agent.list` (+ `events.subscribe` で `pane.agent_status_changed` / `pane.agent_detected` / `pane.closed` 等を購読) |
| 出力の閲覧 | `agent.read` (`source=recent`, `format=ansi`)。更新は `pane.output_changed` を契機に再取得 |
| プロンプト送信 | `agent.prompt`。キー入力は `agent.send_keys` |
| エージェント起動 | 既存ペインを指定して `agent.start`。新しく起動する場合は `tab.create` / `pane.split` でペインを作ってから `agent.start` |

`agent.list` が返すエージェントの情報のうち、Codexel が使うもの:

- `agent_session.agent` / `agent_session.value`: エージェントの種類とセッション ID。`Agent` / `Agent-Session` Trailer と突き合わせるキー (§6)
- `agent_status` (idle / working / blocked / done / unknown), `cwd`, `pane_id`, `workspace_id`, `tab_id`, `terminal_title_stripped`

### 3.2 リポジトリとの対応

エージェントの `cwd` から `jj workspace root` を引き、開いているリポジトリと対応付ける。リポジトリ画面にはそのリポジトリで動いている
エージェントを、エージェント画面には作業中のリポジトリを表示する。

### 3.3 出力の表示

`agent.read` の ANSI 出力をブラウザで表示する。ターミナルエミュレーション (xterm.js 等) は持たず、ANSI の色だけを解釈して表示する。
直接の操作が必要な場合は herdr にフォーカスを移す (`agent.focus`)。

## 4. リポジトリ管理 (#125)

lightjj v1.40.0 の UI/操作体系を再実装する。lightjj は Go バックエンドが `jj` CLI を実行し、Svelte の SPA が表示する構成で、
Codexel も同じく `jj` CLI を実行する (jj をライブラリとして組み込む手段がないため)。

### 4.1 リポジトリの管理

- lightjj のタブに相当する。開くリポジトリは設定ファイル `~/.config/codexel/config.json` に書き、起動時に開く
- UI からリポジトリを開く・閉じる操作も設定ファイルを更新する
- herdr のエージェントの `cwd` にある jj リポジトリを候補として表示する
- jj の workspace は別のリポジトリとして開く (lightjj と同じ)

### 4.2 再実装する機能

lightjj の機能を次のとおり分類する。キーバインドは lightjj と同じにする。

| 分類 | 機能 | キー |
|---|---|---|
| リビジョングラフ | revset 入力、SVG の DAG、`@`/immutable/conflict/divergent の表示、bookmark・PR バッジ、`?change=` 等のリンク | `j`/`k`, `/`, `@`, `r` |
| 差分 | unified/split、シンタックスハイライト、単語単位の差分、コンテキスト展開、hunk 移動、Markdown プレビュー、画像差分、エディタで開く | `]`/`[`, `\|`, `m`, `f` |
| ファイル | リビジョン時点の全ファイルツリー、パス検索、ファイル履歴と 2 点比較、インライン編集 (`@` のみ) | `F` |
| 変更操作 | new / edit / abandon / describe / commit / restore / squash / split (ファイル・hunk) / slide / undo | `n`, `e`, `c`, `s`, `S`, `J`/`K`, `u` |
| rebase | インライン rebase (`-r`/`-s`/`-b` × onto/after/before)、複数選択、megamerge | `R`, `M`, `Space` |
| bookmark | 一覧と同期状態 (ahead/behind/diverged/conflict)、set/move/advance/delete/forget/track/untrack | `2`, `b`, `B` |
| git | fetch / push (ストリーミング出力) | `f`/`p` (修飾キー付き), `g` |
| コンフリクト | コンフリクト一覧、`:ours`/`:theirs`、3 ペインのマージエディタ | `3` |
| 履歴 | op log (undo/restore)、evolog とステップごとの差分 | `4`/`O`, `5`/`E` |
| divergence | divergent なコミットの比較と解決 | — |
| workspace | 一覧 / add / forget / rename / stale の復旧 | — |
| その他 | コマンドパレット (Cmd+K)、jj alias の実行、テーマ (dark/light)、設定 | `t` |

### 4.3 再実装しない機能

- lightjj の annotation (change_id 単位のレビューコメント) とレビューモード (`v`): Codexel のレビュー (§5) に置き換える
- Doc mode (Markdown を ProseMirror で編集・コメント)
- SSH プロキシモード (`--remote`)
- エージェント向け HTTP API と `lightjj api` / `lightjj skill`: Codexel の MCP (§5.4) に置き換える
- Ghostty 由来の 486 テーマ: 組み込みテーマだけにする
- `rg` による定義ジャンプ

### 4.4 状態の同期

lightjj と同じく jj の operation ID で変更を検知する。

- `.jj/repo/op_heads/heads/` を fsnotify で監視し、op ID が変わったら server streaming で通知する
- 定期的に `jj util snapshot` を実行して working copy の変更を取り込む
- 差分などの取得結果は commit ID をキーにキャッシュする (commit ID は内容のハッシュなので op が変わっても無効化不要)

## 5. コードレビュー (#126, #128)

crit の機能を踏襲する。crit とのファイル形式の互換は持たず、jj に合わせたデータモデルにする。

### 5.1 データモデル

crit はブランチと作業ツリーを単位にするが、Codexel は jj の change ID を単位にする。change ID はリビジョンを書き換えても
変わらないので、エージェントが `jj squash` や `jj describe` で修正してもコメントの対象を見失わない。

- レビュー: 対象リポジトリ、対象リビジョンの change ID の集合、ベース (レビュー開始時の親の commit ID)、ラウンドの一覧、状態 (open / finished / approved)
- ラウンド: ラウンド開始時の各 change ID の commit ID
- コメント: 対象 (§5.2)、行範囲、行の内容 (anchor)、本文、作者、resolved、返信

保存先は `~/.config/codexel/reviews/<review-id>.json` で、1 レビュー 1 ファイルにする。

### 5.2 コメントの対象

| 対象 | 指定 |
|---|---|
| レビュー全体 | なし |
| ファイル | change ID + パス |
| ファイルの行・範囲 | change ID + パス + 行範囲 |
| コミットメッセージ全体 | change ID |
| コミットメッセージの行・範囲 | change ID + 行範囲 |

行の位置はリビジョンの書き換えで変わるので、ラウンドが進んだら anchor で位置を合わせ直す。見つからなければ drifted にする (crit と同じ扱い)。

### 5.3 機能

- リポジトリブラウザで選んだリビジョン (または範囲) に対してレビューを開始する
- コード: 行・範囲・ファイルへのコメント
- コミットメッセージ (#128): 範囲内の全コミットのメッセージを、ファイルと並べて表示する。行・範囲・メッセージ全体へのコメント
- レビュー全体へのコメント、返信、resolve
- ラウンド間の差分: 差分の始点と終点を選んで表示する (f110/crit の inter-round diff と同じ)
    - 始点: ベース、またはラウンド M
    - 終点: ラウンド N (M < N、既定は最新)
    - 差分は各ラウンドで記録した commit ID 同士の `jj diff --from --to` で出す。コミットメッセージも同じ始点・終点で差分を表示する
- Finish / Approve
- vim キーバインド (`j`/`k`, `c`, `Shift+F`, `?`)
- Markdown・Mermaid のレンダリング、`linguist-generated` の折りたたみ、自動で既読にするパターン

### 5.4 エージェントとのやり取り

Finish したら、コミットの `Agent-Session` から作成したエージェントを特定し、herdr の `agent.prompt` でレビューが終わったことを伝える。

エージェントは Codexel が提供する MCP でコメントを読み、返信・resolve する。

- `go/issue/mcp` と同じく、Codexel のサーバーが MCP エンドポイント (`/mcp`) を提供する
- ツール: 未解決のコメントの一覧、返信、resolve
- MCP サーバーとして登録するには Codexel のアドレスが毎回同じである必要があるので、`--addr` の既定値は固定のポートにする (番号は #123 で決める)

### 5.5 再実装しない機能

- live mode (動いている Web アプリへのレビュー)、HTML preview
- crit-web への share / fetch / unpublish
- story mode
- GitHub PR / GitLab MR との pull / push
- Send to agent (`agent_cmd`): herdr 経由の引き渡し (§5.4) に置き換える

## 6. エージェントとコミットの紐付け (#124)

### 6.1 Trailer

エージェントはコミットするときに次の 2 つの Trailer を付ける。

```
Agent: <エージェントの種類>
Agent-Session: <セッション ID>
```

- `Agent` の値は herdr のエージェントの種類 (`agent_session.agent`。`claude`, `codex` 等) と同じにする
- `Agent-Session` の値は herdr の `agent_session.value` と同じにする。Claude Code では環境変数 `$CLAUDE_CODE_SESSION_ID` の値になる
- セッション ID はエージェントの種類ごとの値なので、紐付けは種類とセッション ID の組で行う

付与は `jj-trailer` に `--agent-session` フラグを追加して行う。エージェントが実行しやすいように、引数なしで付けられるようにする。

- 環境変数からエージェントの種類とセッション ID を決める。`$CLAUDE_CODE_SESSION_ID` があれば `claude` とその値にする。
  他のエージェントは対応する時に追加する。どれにも当てはまらなければエラーにする
- 既に `Agent-Session` がある場合は追加しない (既存の `-t` は重複を許すので、重複しない専用フラグにする)
- エージェントへの指示は CLAUDE.md 等に書く (例: コミット後に `jj-trailer --agent-session -r <rev>` を実行する)

### 6.2 Codexel での読み取り

jj のテンプレート `self.trailers()` で Trailer を構造化して取得する (jj 0.47.1 で確認済み)。グラフ取得のテンプレートに
`Agent` と `Agent-Session` の値を含め、herdr のエージェント一覧と突き合わせて表示する。

- リビジョングラフ: コミットに作成したエージェントのバッジを表示する
- エージェント画面: そのエージェントが作ったコミットの一覧 (`description(regex:"(?m)^Agent-Session: <id>$")` で検索)
- 終了したエージェントは herdr から消えるので、バッジにはエージェントの種類とセッション ID だけを表示する

## 7. jj-pr の統合 (#127)

### 7.1 ロジックの切り出し

`go/cmd/jj-pr` の submit のロジックを `go/jujutsu` に切り出し、jj-pr と Codexel の両方から使う。

- 標準入出力に依存する部分を引数・戻り値にする
  - PR テンプレートの選択 (`pickTemplate` の `fmt.Scanf`): テンプレートの候補を返し、選択を引数で受け取る
  - 進捗の出力 (`fmt.Printf`): 結果 (作成・更新した PR の URL) を戻り値で返す
- トークンは今と同じく `gh auth token` で取得する
- 切り出しはリファクタリングとして先に行い、jj-pr の既存テストが通ることを確認する

### 7.2 UI からの実行

| 選択 | 動作 |
|---|---|
| コミット 1 つ | `jj pr submit -r <change-id>` と同じ |
| コミット複数 | `jj pr submit -r <最上位の change-id> --single-pr` と同じ |

jj-pr の `--single-pr` は「trunk から `-r` までのスタック」を 1 つの PR にする。そのため複数選択は、選択したコミットが
trunk から連続したスタック (`ancestors(latest(<default>@origin) & remote_bookmarks())..change_id(<最上位>) ~ empty()` と一致する)
の場合だけ許可する。一致しなければ PR 作成の操作を無効にし、理由を表示する。

## 8. API

`proto/codexel/codexel.proto` に定義する。サービスは機能ごとに分ける。

| サービス | 主な RPC |
|---|---|
| `AgentService` | `ListAgents`, `WatchAgents` (stream), `ReadOutput`, `WatchOutput` (stream), `SendPrompt`, `SendKeys`, `StartAgent`, `FocusAgent` |
| `RepositoryService` | `ListRepositories`, `OpenRepository`, `CloseRepository`, `WatchRepository` (stream, op ID の変化) |
| `RevisionService` | `Log`, `GetRevision`, `Diff`, `DiffRange`, `Files`, `FileTree`, `FileHistory`, ... と各変更操作 (§4.2) |
| `BookmarkService` / `GitService` / `OperationService` / `WorkspaceService` | §4.2 の各分類 |
| `ReviewService` | `GetReview`, `StartReview`, `AddComment`, `Reply`, `Resolve`, `Finish`, `Approve`, `RoundDiff` |
| `PullRequestService` | `ListTemplates`, `Submit` |

lightjj の HTTP API を 1:1 で Connect の RPC に写す。git push/fetch や alias のようにストリーミング出力があるものは server streaming にする。

## 9. ビルドと配布

### 9.1 フロントエンド

`ts/apps/issue` と同じく Vite でビルドする (`js_run_binary` で `dist_prod` を出力)。

### 9.2 embed

`go/codexel/static` の `go_library` の `embedsrcs` に `//ts/apps/codexel:vite_build_prod` を指定し、`//go:embed` で取り込む。

- rules_go の `embedsrcs` がディレクトリ出力 (tree artifact) を受け付けるかは #123 で最初に確認する
- `make update-deps` (Gazelle) で `embedsrcs` が消えないように `# keep` を付ける
- 開発時は Vite の dev server から Connect API を Go のサーバーへプロキシする

### 9.3 起動

```
codexel [--addr localhost:<固定ポート>] [--allow-unauthenticated-network] [--no-browser]
```

- 既定では loopback でのみ待ち受ける。Codexel には認証がないので、loopback 以外のアドレスを `--addr` に指定するには
  `--allow-unauthenticated-network` も指定する必要がある (crit と同じ)。指定がなければ起動時にエラーにする
- Host / Origin ヘッダーを検証する (DNS rebinding 対策。lightjj と同じ)
- 起動したらブラウザを開く

## 10. 段階実装

| チケット | 内容 |
|---|---|
| #123 | 骨格 (proto, サーバー, embed, フロントエンドの雛形)、`go/herdr`、エージェント管理 |
| #125 | `go/jujutsu`、リポジトリ管理 (§4.2)。分類ごとにコミットを分ける |
| #124 | `jj-trailer --agent-session`、グラフとエージェント画面での紐付け表示 |
| #126 | `go/codexel/review`、コードのレビュー、MCP |
| #128 | コミットメッセージのレビュー |
| #127 | `go/jujutsu` への PR 作成ロジックの切り出し、PR 作成 |
