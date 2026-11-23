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

スコープ外: 認証。

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
- 状態は持たないことを基本にする。jj のリポジトリと herdr のセッションを正とし、Codexel が持つのは設定ファイル (§4.1)、
  レビュー (§5.1)、Codexel から作成した PR (§7.3) だけにする

### 2.1 パッケージ構成

パッケージは提供するもので分ける。

| パス | 提供するもの |
|---|---|
| `go/cmd/codexel` | エントリポイント。フラグ処理とサーバー起動、エージェントへの MCP の登録 (`setup`) |
| `go/codexel/server` | Connect API と MCP のハンドラ、静的ファイル配信 |
| `go/codexel/static` | embed したフロントエンド (`//go:embed`) |
| `go/codexel/review` | レビューの保存と、リビジョン書き換え時のコメントの追従 |
| `go/codexel/pullrequest` | Codexel から作成した PR の保存 |
| `go/herdr` | herdr ソケット API のクライアント。Codexel 以外からも使える汎用クライアントにする |
| `go/claudecode` | Claude Code のトランスクリプトのリーダー。形式に依存するコードはここだけに置く |
| `go/jujutsu` | jj のコマンドビルダーと出力パーサー (log / diff / bookmark / op log ...) と、`go/cmd/jj-pr` から切り出した PR 作成ロジック (§7.1)。純粋関数と Runner に分ける |
| `proto/codexel` | API 定義 |
| `ts/apps/codexel` | フロントエンド (React + MUI + Connect、`ts/apps/issue` と同じ構成) |

`go/jujutsu` はリポジトリ内に既存のパッケージがないことを `bazel query` で確認してから作る。

## 3. エージェント管理 (#123)

### 3.1 herdr との接続

herdr はソケット (`$HERDR_SOCKET_PATH`、既定 `~/.config/herdr/herdr.sock`) で JSON の request/response API を提供している
(protocol 22, `herdr api schema --json`)。CLI (`herdr agent ...`) を毎回起動せず、ソケットに直接接続する。
1 接続で処理されるリクエストは 1 つなので、リクエストごとに接続する。`events.subscribe` の接続はレスポンスの後もイベントを送り続ける。

| Codexel の機能 | herdr API |
|---|---|
| 一覧と状態表示 | `agent.list` + `events.subscribe`。`pane.created` / `pane.closed` / `pane.updated` / `pane.agent_detected` を購読し、状態の変化は各エージェントのペインを指定して `pane.agent_status_changed` を購読する (`pane_id` が必須) |
| 出力の閲覧 | `agent.read` (`source=recent`, `format=ansi`)。出力の変化は購読できないので、表示している間だけ定期的に再取得する |
| プロンプト送信 | `agent.prompt`。キー入力は `agent.send_keys` |
| エージェント起動 | 既存ペインを指定して `agent.start`。新しく起動する場合は `tab.create` / `pane.split` でペインを作ってから `agent.start`。ワークスペースを指定しない場合は `workspace.create` で作り、herdr のサーバーが動いていなければ `herdr server` を起動する |
| エージェント終了 | `pane.close` でエージェントのペインを閉じる。herdr に `agent.stop` 相当はなく、終了コマンドはエージェントの種類ごとに違うため。タブ・workspace の最後のペインなら herdr がタブ・workspace も閉じる。閉じる前に `agent.get` でエージェントが動いていることを確かめ、エージェントが終わった後のシェルのペインは閉じない。UI では確認ダイアログを出してから実行する |

`agent.list` が返すエージェントの情報のうち、Codexel が使うもの:

- `agent_session.agent` / `agent_session.value`: エージェントの種類とセッション ID。`Agent` / `Agent-Session` Trailer と突き合わせるキー (§6)
- `agent_status` (idle / working / blocked / done / unknown), `cwd`, `pane_id`, `workspace_id`, `tab_id`, `terminal_title_stripped`

エージェント画面の起動ダイアログでは、作業ディレクトリの候補として Codexel で開いているリポジトリ (§4.1) のパスを表示する (#200)。
候補にないディレクトリも入力でき、空なら herdr の既定になる。

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
| リビジョングラフ | revset 入力、SVG の DAG、`@`/immutable/conflict/divergent の表示、各 workspace の working copy (`<name>@`)・bookmark・PR バッジ (§7.3)、`?change=` 等のリンク | `j`/`k`, `/`, `@`, `r` |
| 差分 | unified/split、シンタックスハイライト、単語単位の差分、コンテキスト展開、hunk 移動、Markdown プレビュー、画像差分、エディタで開く | `]`/`[`, `\|`, `m`, `f` |
| ファイル | リビジョン時点の全ファイルツリー、パス検索、ファイル履歴と 2 点比較、working copy のファイルツリーのサイドバー (§4.5) | `F`, `T` |
| 変更操作 | new / edit / abandon / describe / commit / restore / squash / split (ファイル・hunk) / slide / undo | `n`, `e`, `c`, `s`, `S`, `J`/`K`, `u` |
| rebase | インライン rebase (`-r`/`-s`/`-b` × onto/after/before)、複数選択、megamerge | `R`, `M`, `Space` |
| bookmark | 一覧と同期状態 (ahead/behind/diverged/conflict)、set/move/advance/delete/forget/track/untrack | `2`, `b`, `B` |
| git | fetch / push (ストリーミング出力) | `f`/`p` (修飾キー付き), `g` |
| 履歴 | op log (undo/restore)、evolog とステップごとの差分 | `4`/`O`, `5`/`E` |
| divergence | divergent なコミットの比較と解決 | — |
| workspace | 一覧 / add / remove / forget / rename / stale の復旧 | `w` |
| その他 | コマンドパレット (Cmd+K)、jj alias の実行、設定 | — |

リビジョングラフ・bookmark 一覧・op log・workspace 一覧は、キーに加えて左ペイン上部のタブでも切り替えられる。

レビュー画面などからブラウザバックでリポジトリ画面に戻った時は、最後に表示していたタブ (ファイルブラウザを含む) を表示する (#222)。
ファイルブラウザ (§4.5) と同じく、同じ URL の履歴の state に表示を持つ。表示を切り替えるたびに今のエントリの state を書き換える。
evolog は対象のリビジョンを state に持たないので戻さず、リビジョングラフを表示する。

リビジョングラフの bookmark は jjui (jj の `bookmarks` キーワード) と同じ形式で出す (#212)。
ローカルの bookmark と同じ名前のリモートの bookmark が同じリビジョンにある場合はローカルのものだけを出す。
ローカルの bookmark が tracking しているリモートの bookmark と違うリビジョンを指す場合は `<name>*` と出す。判定は jj の `synced()` で、tracking していないリモートの bookmark は対象にしない。

workspace 一覧では `d` で remove (`jj workspace remove`)、`f` で forget する (#175)。bookmark 一覧の `d` (delete) / `f` (forget) に合わせる。
作業を終えた workspace はディレクトリごと消す remove を使うことが多いので、選択中の行のボタンは Remove を Forget の前に置く。
remove は ignore されたファイルも消え、undo してもディレクトリは戻らないので確認ダイアログを出してから実行する。
remove・forget ともに Codexel で開いている workspace は対象にできない。開いているリポジトリのディレクトリや working copy がなくなるため。

リビジョングラフからも remove できる (#204)。working copy のリビジョンを見て消す対象を選べるため。
選択中のリビジョンの「⋯」メニューに、そのリビジョンを working copy にしている workspace ごとに「Remove the workspace `<name>`」を出す。
破壊的な操作なので Abandon と同じグループに赤字で置き、workspace 一覧と同じ確認ダイアログを出してから実行する。
表示しているリポジトリの workspace と、チェックで複数のリビジョンを対象にしている時は無効にする。forget はディレクトリが残り使う機会が少ないので workspace 一覧だけに置く。

リビジョンの詳細パネルの変更ファイルのサマリでは、コンフリクトしているファイルに `conflict` のチップを付け、ヘッダにコンフリクトしているファイルの数を出す (#166)。
親から引き継いだコンフリクトのようにそのリビジョンの差分に含まれないファイルも、変更ファイルの後に並べる。lightjj のファイル一覧と同じ。

リビジョンの詳細パネルの change ID の横のアイコンで change ID をクリップボードにコピーする (#177)。リビジョングラフでは `y` で選択中のリビジョンの change ID をコピーする。
詳細パネルの「⋯」メニューにも入れる。
lightjj にはコピーのキーがないので vim の yank に合わせる (`docs/ui-conventions.md`)。

push はダイアログでチェックしたローカルの bookmark を `jj git push --bookmark=exact:<name>` で push する (#203)。
引数なしの `jj git push` は `revsets.git-push` の範囲にある tracking 済みの bookmark しか push しないため、`@` の祖先でない bookmark や新しい bookmark を push できない。
候補は tracking しているリモートの bookmark と同期していないもの、tracking しているリモートの bookmark がないもの、削除したものとする。コンフリクトしているものは push できないので除く。
split のファイルの選択と同じく複数チェックして一度に push できる。初期状態は何もチェックしない。

デフォルトブランチをチェックした時、push する範囲に未レビューのエージェントのコミットがあればダイアログに警告とそのコミットを表示する (#205)。
エージェントのコミットは push 前のレビューを必須にするため。警告だけで push は止めない。
デフォルトブランチ以外は push 先 (PR など) でレビューされる可能性があるので対象にしない。

- デフォルトブランチは `trunk()` の remote bookmark の名前にする。jj-pr のベースブランチ (`jujutsu.DefaultBranch`) と同じ。
  GitHub には問い合わせない。`trunk()` に remote bookmark がなければ警告しない
- push する範囲は `trunk()..<ローカルの bookmark のコミット>` にする
- エージェントのコミットは `Agent` と `Agent-Session` の Trailer (§6.1) の両方があるコミットにする。リビジョングラフのバッジと同じ
- 未レビューは Approve のマーク (§5.3) がないコミットにする
- 判定は `ReviewService.ListUnreviewedCommits` で行う。チェックを変えるたびに問い合わせる

すべての操作はボタンでも実行できるようにする (#155)。iPad などのタッチ端末ではキーボード・コマンドパレットが使えないため。
キー・コマンドパレットはショートカットとして残す。長押しでメニューを開く操作は見つけにくいので使わない。
右クリックのメニューは使わない (#186)。右クリックのメニューの操作はすべてボタンでも実行でき、導線が重複するため。

| 操作 | ボタンの配置 |
|---|---|
| 選択中のリビジョンの操作 (new / edit / describe / abandon / split / rebase / squash / slide / bookmark / workspace の追加・remove / evolog / ファイル / レビュー / PR 作成) | 詳細パネルの change ID の行の右端の「⋯」ボタンで開くメニュー。ボタンを横に並べると詳細パネルの主役である説明と差分より目立つため。チェックがある場合はチェックしたリビジョンが対象になる (キーと同じ) |
| レビューの開始 (#202) | 「⋯」ボタンの左のアイコンボタン。頻繁に使うのでメニューを開かずに 1 回で開始できるようにする。他の操作との導線を揃えるためメニューにも残す。対象はメニューと同じ |
| 複数選択のチェック | 左ペインのタブの右の Select ボタンで選択モードに入り、グラフの各行にチェックボックスを出す。普段はグラフを狭くしないようにチェックボックスを出さない。Cancel・操作の実行・Esc でチェックを外して選択モードを終える |
| リポジトリ全体の操作 (commit / undo / fetch / push / `@` への移動 / 再読み込み / alias) | revset 入力の右の「⋯」ボタンで開くメニュー。ボタンを並べると種類の違う操作の区別がつかず見づらいため |
| rebase / squash モード | モード中の警告バーにソース・ターゲット (squash は説明の結合) の切り替えと、実行・キャンセル。移動先は行のタップで選ぶ |
| bookmark の delete / forget / track / untrack、op log の revert / restore、workspace の open / remove / forget | 一覧の選択中の行 |
| 開いている workspace の rename / stale の復旧 | workspace 一覧の上部 |
| ファイルブラウザからログに戻る | ファイルツリーの上部 |
| working copy のファイルツリーのサイドバーの開閉 | revset 入力の左のトグルボタン。サイドバーを出す左端に置く |
| 差分の split 表示・Markdown プレビュー・hunk 移動 | 右ペインの上部 (スクロールしない位置) |

### 4.3 再実装しない機能

- lightjj の annotation (change_id 単位のレビューコメント) とレビューモード (`v`): Codexel のレビュー (§5) に置き換える
- Doc mode (Markdown を ProseMirror で編集・コメント)
- SSH プロキシモード (`--remote`)
- エージェント向け HTTP API と `lightjj api` / `lightjj skill`: Codexel の MCP (§5.4) に置き換える
- テーマ (Ghostty 由来の 486 テーマ、dark/light の切り替え): ライトテーマだけにする
- `rg` による定義ジャンプ
- コンフリクト一覧、`:ours`/`:theirs`、3 ペインのマージエディタ: 使わないので廃止した (#183)
- ファイルのインライン編集 (`@` のみ)

### 4.4 状態の同期

lightjj と同じく jj の operation ID で変更を検知する。

- `.jj/repo/op_heads/heads/` を fsnotify で監視し、op ID が変わったら server streaming で通知する
- 定期的に `jj util snapshot` を実行して working copy の変更を取り込む
- 差分などの取得結果は commit ID をキーにキャッシュする (commit ID は内容のハッシュなので op が変わっても無効化不要)

### 4.5 ファイルブラウザ (#150)

§4.2 のファイル (`F`) を、リポジトリ内のファイルを読むための 2 ペインのビュワーにする。左にファイルツリー、右にタブで開いたファイルを表示する。
定義へのジャンプなどファイルの構造を解析したナビゲーションは持たない (§4.3 の `rg` による定義ジャンプと同じ扱い)。

- 表示するのはリビジョングラフで選んだリビジョン (working copy なら `@`) の時点のファイルにする。`F` を押した時点の commit ID で
  `ListFiles` / `GetFile` を呼ぶので、working copy の変更は `F` を押し直すと反映される
- ファイルは jj のストアから `jj file show` で読む。ファイルシステムは読まないので、symlink の先は読まない。`GetFile` はリポジトリの外を指す
  パス (絶対パスと `..` を含むパス) を jj を実行する前に拒否する
- タブはファイルのパスで持ち、`F` で別のリビジョンを開いても残す。ログに戻っても (`1` / `Esc`) 残す
- ブラウザバックで前に開いていたファイルとスクロール位置に戻る (#148 と同じく、同じ URL の履歴の state に位置を持つ)
    - ファイルを開く・タブを切り替える時に、今のエントリにそれまで表示していたファイルとスクロール位置を書き、開いたファイルのエントリを push する
    - 戻った先のファイルのタブが閉じられていれば開き直す。リビジョンが違えばそのリビジョンに切り替える
    - ログなど他の表示から戻った場合や、リロードした場合もファイルブラウザを表示する
- Markdown のファイルは GFM としてプレビューする (#181)。ファイルを読むためのビュワーなので、デフォルトはプレビュー表示にし、
  ソースは見たい時だけ差分の Markdown プレビューと同じ `m` とファイル名の横の Preview ボタンで切り替えて表示する
    - 状態は差分と共有しない。差分は変更を見るためのものなのでデフォルトはソース表示のままにする。
      ファイルブラウザの表示中は差分のキー (`m`, `|`, `]`/`[`) を処理しない
- ツリーのファイル・ディレクトリのアイコンは `material-icon-theme` (VS Code の Material Icon Theme) の SVG を使う (#188)。
  言語ごとにロゴを元にした形と色が違うアイコンで、一目で種別を見分けられるため。拡張子・ファイル名とアイコンの対応は
  `material-icon-theme` の対応表に合わせて codexel 側に持ち、使うアイコンだけを個別に import する (全アイコンは 5MB あるため)。
  対応表にないファイルは汎用のファイルアイコンにする。Codexel はライトテーマなので `_light` の版があるものはそちらを使う
- キー: `j`/`k`, `h`/`l` (ツリー), `Enter` (開く), `H`/`L` (前後のタブ), `x` (タブを閉じる), `m` (Markdown のプレビュー), `/` (パスの絞り込み)

#### サイドバー (#182)

リビジョンを選ばずに working copy のファイルを開けるように、リポジトリ画面の左端に `@` のファイルツリーのサイドバーを出す。
多くの場合に見たいのは `@` のファイルなため。

- 表示するのは常に `@` のファイル。選択中のリビジョンには追従しない。選択中のリビジョンのファイルは `F` で開く
- サイドバーはファイルブラウザへの入口にする。ツリーでファイルを選ぶと、その時点の `@` の commit ID でファイルブラウザを開いてそのファイルを開く。
  ファイルの表示・タブ・履歴はファイルブラウザのものを使い、ファイルを表示する場所を 2 つにしない
- ツリーの表示と絞り込みはファイルブラウザのツリーと同じ部品を使う。キーはリビジョングラフが使うので、ツリーの操作はタップ (クリック) で行う
- `@` の commit ID は op ID が変わるたびに取り直す (§4.4)。working copy の変更は自動で反映される
- 開閉は `T` と revset 入力の左のトグルボタンで行い、開閉の状態は差分の split 表示と同じくローカルストレージに保存する。
  キーは表示を切り替える他のキー (`F`, `O`, `E`) と同じく大文字にし、Tree の頭文字にする

## 5. コードレビュー (#126, #128)

crit の機能を踏襲する。crit とのファイル形式の互換は持たず、jj に合わせたデータモデルにする。

### 5.1 データモデル

crit はブランチと作業ツリーを単位にするが、Codexel は jj の change ID を単位にする。change ID はリビジョンを書き換えても
変わらないので、エージェントが `jj squash` や `jj describe` で修正してもコメントの対象を見失わない。

- レビュー: 対象リポジトリ、対象リビジョンの change ID の集合、ラウンドの一覧、状態 (open / finished / approved / discarded)
- ラウンド: ラウンド開始時の各 change ID の commit ID
- コメント: 対象 (§5.2)、行範囲、差分の側 (old / new)、行の内容 (anchor)、本文、作者、resolved、返信
- 既読: change ID + パスごとの既読・未読。未読の記録は自動で既読にするパターンより優先する

差分は change ごとに親との差分を表示する。コメントの対象の change ID が表示の単位から決まるようにするため。

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
- エージェント画面から、そのセッションのコミット (§6.2) をまとめた change ID の集合に対してレビューを開始する
- コード: 行・範囲・ファイルへのコメント
- コミットメッセージ (#128): 範囲内の全コミットのメッセージを、ファイルと並べて表示する。行・範囲・メッセージ全体へのコメント
    - 各 change の最初のファイルとして表示する。メッセージは短いので、全行を 1 つの hunk にする
    - ベースからは表示しているラウンドのメッセージ、ラウンド M からは M のメッセージとの差分を表示する
    - 行へのコメントは最新ラウンドのメッセージの行で、ラウンドが進んだらファイルと同じく anchor で位置を合わせ直す
- レビュー全体へのコメント、返信、resolve
- ラウンド間の差分: 差分の始点と終点を選んで表示する (f110/crit の inter-round diff と同じ)
    - 始点: ベース、またはラウンド M
    - 終点: ラウンド N (M < N、既定は最新)
    - ベースからの差分は、ラウンド N の commit ID と親の差分 (通常の表示) にする
    - ラウンド M からの差分は change ごとに各ラウンドで記録した commit ID 同士の `jj diff --from --to` で出す。コミットメッセージも同じ始点・終点で差分を表示する
- Finish: 未 resolve のコメントがあれば finished にしてエージェントに伝える。なければ approved にする (#142)
- Discard (#176): open か finished のレビューを approve せずに終わらせ、discarded にする。approved のレビューは対象外
    - ラウンドで記録した commit が古くなった場合など、進め方を変えたいレビューを終わらせるため
    - コメント・ラウンド・既読は残す。エージェントには伝えず、次のラウンドの開始とコメントの追加は approved と同じく拒否する
    - 戻す手段がなく Finish の隣にあるので、確認のダイアログを出す
- Approve したコミットは、リビジョングラフとエージェント画面のコミット一覧でマークする (#135)
    - マークは保存せず、レビューから導く。approved のレビューの最新ラウンドの commit と、同じ change の今の commit を
      `jj interdiff --git` で比べ、差分がなければマークする
    - interdiff は親の違いを無視するので、rebase してもマークは外れない。内容かコミットメッセージを変えると外れる
    - commit ID は内容のハッシュなので、比較の結果は commit ID の組でキャッシュする
- リポジトリ画面からコミットのレビューに移動できる (#141)
    - change ID でレビューと対応付ける。同じ change のレビューは状態 (open / finished / approved / discarded) に関わらずすべて対象にする
    - リビジョングラフの行には最新のレビューの状態とレビューの数をバッジで表示し、クリックで最新のレビューを開く
    - 詳細パネルにはその change のレビューをすべて表示し、選んだレビューを開く
- 次のラウンドはエージェントが指摘に対応した後に MCP のツールで開始する (crit で再度 `crit` を実行するのと同じ)。その時点の各 change ID の
  commit ID を記録し、コメントの位置を合わせ直して、状態を open に戻す
    - finished の間はエージェントを待っていることをモーダルで表示する (crit と同じ)。閉じることもでき、次のラウンドが始まると閉じる (#140)
- vim キーバインド (`j`/`k`, `c`, `Shift+F`, `?`)。既読の切り替えは crit と同じ `a`
- キーと同じ操作をボタンでも実行できる (#155、§4.2)
    - 行・範囲の選択は行番号のクリックで行う。範囲は Shift+クリックに加えて、コメントの入力欄の「Extend the range」をオンにしてから
      行番号をクリックしても広げられる
    - split 表示・Markdown プレビュー・hunk 移動のボタンは、Finish と同じくスクロールしない上部のヘッダーに置く
- Markdown・Mermaid のレンダリング。mermaid はサイズが大きいので図を表示する時だけ読み込む
- 自動生成ファイルの折りたたみ (#165): GitHub と同じく、`linguist-generated` が設定されていればそれに従い、未設定なら Linguist のヒューリスティック
  (Go 移植の go-enry の `IsGenerated`) で判定する
    - `.gitattributes` はコミット時点のトップレベルだけを読む。パターンは git と同じ解釈にする。`-linguist-generated` と `=false` はヒューリスティックを打ち消す
    - ヒューリスティックはファイルの内容を使う。削除されたファイルとバイナリはパスだけで判定する
- 既読: 既読のファイルは折りたたむ。次のラウンドでは前のラウンドとの差分 (`jj diff --from --to`) に含まれるファイルの既読を外す
- 自動で既読にするパターン: `config.json` の `auto_viewed_patterns` (全リポジトリ共通)。レビュー開始時とラウンド開始時に、記録のない変更ファイルに適用する
- レビューエージェント (#190): open のレビューに対して、人間のレビューとは別にレビューするエージェントを起動できる (§5.4)

### 5.4 エージェントとのやり取り

未 resolve のコメントを残して Finish したら、コミットの `Agent-Session` から作成したエージェントを特定し、herdr の `agent.prompt` でレビューが終わったことを伝える。

エージェントは Codexel が提供する MCP でコメントを読み、返信・resolve する。

- `go/issue/mcp` と同じく、Codexel のサーバーが MCP エンドポイント (`/mcp`) を提供する
- ツール: レビューの一覧、未解決のコメントの一覧、コメントの追加、返信、resolve、次のラウンドの開始
- MCP サーバーとして登録するには Codexel のアドレスが毎回同じである必要があるので、`--addr` の既定値は固定のポートにする (番号は #123 で決める)
- 登録は `codexel setup` で行う (#131)。`claude mcp add --transport http --scope user codexel <URL>` を実行する
    - `~/.claude.json` は Claude Code の内部状態を含み、動いている Claude Code も書き込むので、直接編集せず `claude` コマンドに任せる
    - URL は `--addr` (サーバーと同じフラグ) から `http://<addr>/mcp` にする。ホストが空や `0.0.0.0` / `::` の場合は `localhost` にする
    - 既に登録されている場合は `claude mcp remove --scope user codexel` で消してから登録し直す。何度実行しても同じ結果になり、`--addr` を変えたら URL も追従する

#### レビューエージェント (#190)

レビュー画面の「Start review agent」で、変更をレビューしてコメントするエージェントを herdr で起動する。

- プロンプトはリポジトリの `.codexel/review.md` を使い、起動ごとには指定しない。起動ごとに指定するとレビューの観点がぶれ、手間もかかるため
    - `jj file show` で `trunk()` の時点のファイルを読む。レビュー対象の変更や working copy の編集でレビューエージェントへの指示が変わらないようにするため。
      ファイルがなければエラーにし、エージェントは起動しない
    - Codexel はこの内容の前に、レビュー ID、最新ラウンドの change ID と commit ID、MCP の `add_review_comment` でコメントすること、
      ファイル・コミットを変更しないこと、返信・resolve・次のラウンドの開始をしないことを付けて `agent.prompt` で送る
- 起動はエージェント画面の起動 (§3.1) と同じ `AgentService.Start` に `review_id` を渡して行う。ワークスペース (空なら新しく作る) と種類を選び、
  作業ディレクトリはレビューのリポジトリにする (`cwd` は指定できない)。
  起動か `agent.prompt` に失敗した場合は作ったタブ・workspace を閉じる
- 起動できるのは open のレビューだけにする。finished は実装エージェントの対応を待っている状態のため
- レビューエージェントのコメントは作者が `agent` の通常のコメントになる (MCP 経由の操作の作者は `agent`)。実装エージェントはコメントを追加せず返信するだけなので、
  作者が `agent` のコメントはレビューエージェントのものになる
- コメントを実装エージェントに渡すかは人間が決める。Finish で伝わるのは未 resolve のコメントなので、渡さないコメントは人間が resolve する。
  人間は返信を付けて補足することもできる
- レビューエージェントは Trailer の付いたコミットを作らないので、Finish の通知先にはならない。終了は他のエージェントと同じくエージェント画面の Stop で行う

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
  `$OPENCODE_SESSION_ID` があれば `opencode` とその値にする。他のエージェントは対応する時に追加する。どれにも当てはまらなければエラーにする
- opencode はセッション ID を環境変数で渡さないので、プラグインの `shell.env` フックで `input.sessionID` を `OPENCODE_SESSION_ID` に設定する。
  この値は herdr の opencode 統合が `agent_session` として報告する sessionID と同じ
- 既に `Agent-Session` がある場合は追加しない (既存の `-t` は重複を許すので、重複しない専用フラグにする)
- エージェントへの指示は CLAUDE.md 等に書く (例: コミット後に `jj-trailer --agent-session -r <rev>` を実行する)

### 6.2 Codexel での読み取り

jj のテンプレート `self.trailers()` で Trailer を構造化して取得する (jj 0.47.1 で確認済み)。グラフ取得のテンプレートに
`Agent` と `Agent-Session` の値を含め、herdr のエージェント一覧と突き合わせて表示する。

- リビジョングラフ: コミットに作成したエージェントのバッジを表示する
- エージェント画面: そのエージェントが作ったコミットの一覧。`description(regex:"(?m)^Agent: <種類>$") & description(regex:"(?m)^Agent-Session: <id>$")` で
  絞り込み、本文中の同じ行に一致したコミットは Trailer で除外する。エージェントは `cwd` 以外のリポジトリでもコミットするので、リポジトリ画面で開いている全リポジトリを検索し、リポジトリごとにまとめて表示する (#201)。
  `cwd` から `jj workspace root` で引いたリポジトリを先に検索し、同じ jj リポジトリの workspace を複数開いていて同じコミットが見つかった場合は先に見つけたリポジトリにだけ表示する。一覧はコミットのあるリポジトリと `cwd` のリポジトリが変わるたびに読み直す。
  選んだコミットの説明と差分をリポジトリ画面と同じ部品で表示する。一覧からリポジトリごとにそのセッションのコミットのレビューを開始できる (§5.3)
- 終了したエージェントは herdr から消えるので、バッジにはエージェントの種類とセッション ID だけを表示する

### 6.3 Issue へのリンク (#178)

リビジョンの説明の Trailer にある `#N` を Issue トラッカーのチケット N へのリンクにする。

- Issue トラッカーの Web UI の URL は `--issue-url` で指定する (例: `https://issue.internal.x.f110.dev`)。チケット N の URL は `<--issue-url>/N` にする。issue-server の `--url` と同じ形
- フロントエンドへは `SystemService.GetStatus` で渡す。指定しない場合はリンクにしない
- Trailer は説明の最後の段落とする (jj の `trailers()` と同じ)。`trailers` が空のコミットはリンクにしない

## 7. jj-pr の統合 (#127)

### 7.1 ロジックの切り出し

`go/cmd/jj-pr` の submit のロジックを `go/jujutsu` に切り出し、jj-pr と Codexel の両方から使う。

- 標準入出力に依存する部分を引数・戻り値にする
  - PR テンプレートの選択 (`pickTemplate` の `fmt.Scanf`): テンプレートの候補を返し、選択を引数で受け取る
    - 候補は `PullRequestTemplates` で返す。前回選んだテンプレート (`.jj/.last_template_file`) があればそれだけを返す
    - 選択は `PullRequestSubmitter.SelectTemplate` で受け取る。PR を作成するときに候補が複数ある場合だけ呼ぶ。
      候補が 1 つならそれを使う (jj-pr と同じ)
  - 進捗の出力 (`fmt.Printf`): 結果 (作成・更新した PR の URL) を戻り値で返す
    - `Submit` が push した change ID と、PR ごとの操作 (作成・更新・既存) と URL を返す。jj-pr はそれを表示する
- トークンは今と同じく `gh auth token` で取得する (`go/github` の `CLIToken`)。テストでは `PullRequestSubmitter.GitHubClient` にモックのクライアントを渡す
- 切り出しはリファクタリングとして先に行い、jj-pr の既存テストが通ることを確認する

### 7.2 UI からの実行

| 選択 | 動作 |
|---|---|
| コミット 1 つ | `jj pr submit -r <change-id>` と同じ |
| コミット複数 | `jj pr submit -r <最上位の change-id> --single-pr` と同じ |

jj-pr の `--single-pr` は「trunk から `-r` までのスタック」を 1 つの PR にする。そのため複数選択は、選択したコミットが
trunk から連続したスタック (`ancestors(latest(<default>@origin) & remote_bookmarks())..change_id(<最上位>) ~ empty()` と一致する)
の場合だけ許可する。一致しなければ PR 作成の操作を無効にし、理由を表示する。

- 最上位は選択したコミットの `heads()` で決める。1 つに決まらなければ許可しない
- 判定は `PullRequestService.Validate` で行い、`Submit` でも同じ判定をする
- 1 つ選択した場合も、trunk からのスタックが空 (選択したコミットが trunk に含まれる等) なら許可しない
- スタックのコミットがすべて `WIP:` / `no-send:` で始まる場合も、PR にするコミットがないので許可しない (jj-pr はこれらを除外する)
- テンプレートが複数あるときは jj-pr と同じく選択を必須にする (「使わない」も選べる)。1 つならそれを使う
- 結果として push した change ID と、PR ごとの操作と URL をダイアログに表示する
- PR の更新時にコメント (push 前の head からの compare の URL) を付けるかをチェックボックスで選ぶ。`jj pr submit --comment-on-update` と同じで、
  デフォルトは付けない (#158)
  - 送信するコミットにブックマークがない (初めて push する) 場合は、更新する PR がないのでチェックボックスを disabled にする。
    判定は `PullRequestService.Validate` で行う
  - PR の状態 (open / draft) は Codexel では取得しない。draft の PR にはチェックが入っていても `PullRequestSubmitter` がコメントを付けない

### 7.3 PR の保存 (#159)

UI から作成した PR の URL を保存し、リビジョングラフの PR バッジから PR を開けるようにする。jj-pr の CLI で作成した PR は保存しない
(今は Codexel だけが使う情報のため)。

- 保存先は `~/.config/codexel/pullrequests.json` の 1 ファイルにする。PR は ID を持たずリポジトリと change ID で引くだけなので、
  レビュー (1 レビュー 1 ファイル) ではなく設定ファイルと同じ扱いにする。書き込みはプロセス内で直列化し、一時ファイルに書いて rename する
- 1 件はリポジトリ (`jj workspace root`。レビューと同じ)、change ID、URL。同じリポジトリと change ID の PR は上書きする
- `Submit` の結果の PR を作成・更新・既存に関わらずすべて保存する。複数選択 (`--single-pr`) は結果に最上位の change だけが入るので、
  最上位の change に紐付く
- 保存に失敗しても PR は作成済みなので、`Submit` は失敗にせず結果を返す
- 保存する change ID は `Submit` の結果のまま (完全な change ID) にする。`List` は `RepositoryService.Log` の change ID と同じ形式
  (`change_id.short()` の 12 文字) にして返す (#211)
- フロントエンドは `PullRequestService.List` で取得し、change ID で引いてバッジを表示する。バッジは PR を新しいタブで開く

## 8. API

`proto/codexel/codexel.proto` に定義する。サービスは機能ごとに分ける。

| サービス | 主な RPC |
|---|---|
| `AgentService` | `List`, `Watch` (stream), `ReadOutput`, `SendPrompt`, `SendKeys`, `Focus`, `ListWorkspaces`, `Start`, `Stop`, `WatchTranscript` (stream) |
| `RepositoryService` | `List`, `Open`, `Close`, `Watch` (stream, op ID の変化), `Log`, `GetRevision`, `Diff`, `DiffRange`, `Files`, `FileTree`, `FileHistory`, ... と §4.2 の各操作 (リビジョン、ブックマーク、Git、オペレーション、ファイル、ワークスペース、エイリアス) |
| `ReviewService` | `List`, `ListApprovedCommits`, `ListUnreviewedCommits`, `Get`, `Start`, `AddComment`, `EditComment`, `DeleteComment`, `Reply`, `ResolveComment`, `Finish`, `Discard`, `StartNextRound`, `RoundDiff`, `SetViewed` |
| `PullRequestService` | `List`, `ListTemplates`, `Validate`, `Submit` |
| `SystemService` | `GetStatus` (起動時に PATH に見つからなかった `herdr` と `jj`、`--issue-url`) |

lightjj の HTTP API を 1:1 で Connect の RPC に写す。git push/fetch や alias のようにストリーミング出力があるものは server streaming にする。

## 9. ビルドと配布

### 9.1 フロントエンド

`ts/apps/issue` と同じく Vite でビルドする (`js_run_binary` で `dist_prod` を出力)。

画面をまたいで揃える見た目は MUI のテーマ (`providers/AppProvider.tsx`) に置き、コンポーネントの `sx` で個別に上書きしない (#207)。

- ボタン・タブ・ToggleButton のラベルは大文字にしない。リポジトリ名・ファイル名・bookmark 名のように大文字小文字に意味があるラベルがあるため
- タブの最小の高さは 36px にする。閉じるボタンを持つタブ (リポジトリ・開いたファイル) もほかのタブと同じ高さにするため、閉じるボタンの余白をなくす
- 小さい Chip (`size="small"`) の高さは 18px にする。リビジョングラフの 1 行に複数並べるため
- 等幅フォントは `monospace` にする

### 9.2 embed

`//ts/apps/codexel:vite_build_prod` の出力ディレクトリを `go/codexel/static` に `copy_directory` でコピーし、それを `go_library` の
`embedsrcs` に指定して `//go:embed` で取り込む。

- 別のパッケージの出力を `embedsrcs` に直接指定すると、`//go:embed` のパターンにマッチしない。そのため同じパッケージにコピーする
- `make update-deps` (Gazelle) で `embedsrcs` が消えないように `# keep` を付ける
- 開発時は Vite の dev server から Connect API を Go のサーバーへプロキシする

### 9.3 起動

```
codexel [--addr localhost:<固定ポート>] [--no-browser] [--issue-url <URL>]
```

- 起動したらブラウザを開く
- `--issue-url` は Trailer の `#N` をリンクにする Issue トラッカーの URL (§6.3)
- `codexel setup [--addr ...]` はエージェントに Codexel を使わせるための設定を行う。今は MCP の登録 (§5.4) だけを行う

## 10. 段階実装

| チケット | 内容 |
|---|---|
| #123 | 骨格 (proto, サーバー, embed, フロントエンドの雛形)、`go/herdr`、エージェント管理 |
| #125 | `go/jujutsu`、リポジトリ管理 (§4.2)。分類ごとにコミットを分ける |
| #124 | `jj-trailer --agent-session`、グラフとエージェント画面での紐付け表示 |
| #126 | `go/codexel/review`、コードのレビュー、MCP |
| #128 | コミットメッセージのレビュー |
| #127 | `go/jujutsu` への PR 作成ロジックの切り出し、PR 作成 |
