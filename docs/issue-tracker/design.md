# 内部向け Issue Tracker — 設計ドキュメント

Status: Draft / Working
前提: `README.md`（検討メモ・決定事項）、`mock/`（画面モック）

## 1. 目的とスコープ

内部向けの issue tracker を作る。汎用的なカスタマイズ性は持たず、次の 2 点に機能を集中させる。

- 差分（commit / PR）とチケットの紐づけ、およびその CI 結果の表示
- 人間とコーディングエージェントが同じ操作を行え、どちらが行ったかが常に追跡できること

スコープ外: 通知、リリースとの紐づけ表示、スマートフォン向け API（いずれも `README.md` の「今後の課題」）。
期限・見積・スプリント・エクスポートは持たない。

## 2. 全体構成

### 2.1 コンポーネント

```
  Browser ──┐                         GitHub
  CLI ──────┤                           │ webhook (push / pull_request)
  Agent ────┤ (MCP)                     │
            ▼                           ▼
     ┌─────────────── heimdallr proxy ───────────────┐
     │  人間・Agent: X-Auth-Token (JWT) を付与       │
     │  webhook: 署名検証                            │
     └──────────────────────┬────────────────────────┘
                            ▼
     ┌──────────────────── server ────────────────────┐
     │  Connect API   MCP endpoint   /webhook          │
     │        └────────┬────┘            │             │
     │                 ▼                 ▼             │
     │                issue ◀── event worker           │
     └─────────┬───────────────────────┬──────────────┘
               ▼                       ▼
            MariaDB              MinIO (添付)

     event worker ──▶ GitHub API (PR のコミット一覧)
                  ──▶ git-data-service (change-id の取得)
     issue        ──▶ Build API (CI 結果)
```

サーバーは 1 バイナリで、複数レプリカで動かせる。Webhook は受信と処理を分け、受信時は DB に INSERT して即 200 を
返し、同一プロセス内の worker が処理する。worker は全レプリカで動き、event の取得は
`UPDATE ... SET state = PROCESSING WHERE id = ? AND state = PENDING` の更新件数で奪い合う。
処理自体も UNIQUE 制約で冪等にしてあるので、リーダー選出は持たない。この受信と処理の仕組みは Build と共通化する（§5.1）。

### 2.2 実装上の約束

- チケットに関するすべての書き込みは `issue` パッケージを経由する。Connect API・MCP・event worker は `issue` を
  呼ぶだけで、DB を直接更新しない（MCP は Connect API のハンドラを経由して `issue` を呼ぶ）。これにより「人間とエージェントで同じことができる」ことと
  「すべての操作が履歴に残る」ことを 1 箇所で保証する
- `issue` は標準ライブラリと同じく提供するものの型に操作を持たせる。`Tracker` がチケットの作成・取得・検索を、
  `Ticket` が更新・関連・コメントの追加を、`Comment` が編集・削除を持つ
- 各操作は、チケットの更新と操作履歴の INSERT を同一トランザクションで行う
- 操作主体は `context.Context` で受け渡し、操作は引数で actor を受け取らない

### 2.3 パッケージ構成

| パッケージ | 責務 |
| --- | --- |
| `proto/issue` | Connect API の定義 |
| `go/issue/database`, `go/issue/database/dao` | DB スキーマ（protoc-ddl の `schema.proto`）と生成コード、DAO |
| `go/issue` | issue tracker 本体。チケット・関連・コメント・分類の操作、認可、操作履歴の記録、検索 |
| `go/issue/filter` | AIP-160 で書いたチケットのフィルタを SQL の条件に変換する |
| `go/issue/server` | HTTP サーバー。Connect・Webhook・添付のハンドラ |
| `go/issue/mcp` | MCP のハンドラ。tool は `go/issue/server` の Connect のハンドラを呼ぶ（§6.2） |
| `go/issue/webhook` | push / pull_request の Reconciler と trailer の解釈 |
| `go/github/webhook` | Webhook の受信・永続化・非同期処理のフレームワーク。Build と共用する（§5.1） |
| `go/http/authproxy` | heimdallr の JWT の検証。issue 以外のアプリケーションと共用する（§4.1） |
| `go/cmd/issue-server` | サーバーのエントリポイント |
| `go/cmd/issue` | 人間向け CLI |
| `ts/apps/issue` | Web UI |

## 3. データモデル

MariaDB を使い、スキーマは Build と同じく protoc-ddl で `go/issue/database/schema.proto` から生成する。マイグレーションも Build と同じく
別の `migrate` Job で行い、サーバーは起動時にスキーマハッシュの一致を待つ。

### 3.1 テーブル

| テーブル | 主なカラム | 備考 |
| --- | --- | --- |
| `ticket` | `id` (sequence), `title`, `body`, `status`, `type` (未設定は `TICKET_TYPE_NONE`), `priority` (未設定は `PRIORITY_NONE`), `component_id` (null 可), `assignee_id` (null 可), `reporter_id`, `created_at`, `updated_at`, `last_activity_at` | `id` がそのままチケット番号。`last_activity_at` はコメントを含む最後の操作の時刻 |
| `ticket_body_revision` | `id`, `ticket_id`, `patch`, `created_at` | 編集前の本文を編集後の本文から復元する差分 |
| `actor` | `id`, `kind` (human / agent / system), `name`, `owner_id` (agent のみ、所有者の human の actor), `subject` (human のみ、heimdallr のユーザー ID) | UNIQUE(`subject`), UNIQUE(`owner_id`)。system は固定の 1 行 |
| `component` | `id`, `name` | ソフトウェア分類のマスタ |
| `ticket_template` | `id`, `title`, `body`, `component_id` (null 可) | 新規作成画面の初期値。`title` はテンプレートの名前を兼ねる。UNIQUE(`title`)。チケットからは参照しないので物理削除する |
| `relation` | `from_id`, `type`, `to_id` | UNIQUE(`from_id`, `type`, `to_id`) |
| `mention` | `from_id`, `to_id`, `created_at` | 本文・コメントで言及したチケット (`from_id`) と言及されたチケット (`to_id`)。UNIQUE(`from_id`, `to_id`)（§3.3） |
| `comment` | `id`, `ticket_id`, `actor_id`, `body`, `created_at`, `updated_at`, `deleted_at` | 削除は `deleted_at` を立てる論理削除 |
| `comment_revision` | `id`, `comment_id`, `patch`, `created_at` | 編集前のコメントを編集後のコメントから復元する差分 |
| `attachment` | `id`, `ticket_id`, `actor_id`, `name`, `content_type`, `size`, `object_key`, `created_at` | |
| `commit_link` | `ticket_id`, `repository_id`, `sha`, `change_id` (null 可), `subject`, `author`, `trailer` (refs / fixes), `committed_at` | UNIQUE(`ticket_id`, `repository_id`, `sha`)。`repository_id` は Build の `SourceRepository` の ID |
| `pull_request_link` | `ticket_id`, `repository_id`, `number`, `title`, `author`, `state` (open / closed / merged), `head_sha` | UNIQUE(`ticket_id`, `repository_id`, `number`)。`repository_id` は Build の `SourceRepository` の ID |
| `agent_session` | `id`, `ticket_id`, `agent` (claude_code / codex / gemini_cli / opencode), `hostname`, `session_id`, `created_at` | UNIQUE(`ticket_id`, `hostname`, `session_id`)。チケットに取り組んだコーディングエージェントのセッション（§3.4） |
| `activity` | `id`, `ticket_id`, `actor_id`, `kind`, `from_value`, `to_value`, `ref_id`, `created_at` | 操作履歴 |
| `github_event` | `id`, `delivery_id`, `event_type`, `action`, `payload`, `state`, `status`, `last_error`, `created_at`, `updated_at` | UNIQUE(`delivery_id`)。カラムは Build の `github_event` と揃える |

- ステータス・種別・優先度・関連種別・trailer 種別は proto の enum で持つ
- `relates_to` は対称なので `from_id < to_id` に正規化して保存し、二重登録を防ぐ
- リポジトリは Build に登録されたものを使い、issue tracker 自身は持たない（§5.1）。commit / PR の URL は保存せず、
  Build から引いたリポジトリの URL と sha / 番号から表示時に組み立てる
  (`<url>/commit/<sha>`, `<url>/pull/<number>`)

### 3.2 操作履歴

`activity.kind` は次のいずれか。値の変化は `from_value` / `to_value`、対象のレコードは `ref_id` に入れる。

| kind | from / to | ref_id |
| --- | --- | --- |
| create | — | — |
| title, status, type, priority, component, assignee | 変更前 / 変更後 | — |
| body | — | その編集の `ticket_body_revision.id` |
| relation_added, relation_removed | — / `<type>:<相手の番号>` | — |
| comment | — | `comment.id` |
| comment_edited | — | その編集の `comment_revision.id` |
| comment_deleted | — | `comment.id` |
| attachment | — | `attachment.id` |
| commit_linked, pull_request_linked | — | `commit_link` / `pull_request_link` の行 |
| mentioned, mention_removed | — / 言及元の番号 | — |

本文とコメントは最新だけを全文で `ticket.body` / `comment.body` に持ち、編集ごとに編集前の内容を編集後の内容から
復元する差分を `ticket_body_revision` / `comment_revision` に保存する。過去の版は最新の全文に新しい順に差分を当てて
復元する。削除したコメントは履歴から参照され続けるため、行は消さずに論理削除にする。

操作主体が人間かエージェントかは `actor.kind` で判定する。actor の `kind` と `owner_id` は作成後に変更しない
ため、履歴側にコピーしなくても当時の値が復元できる。

### 3.3 言及（バックリンク）

本文やコメントで `#123` と書いたチケットを、言及されたチケットから辿れるようにする（バックリンク）。関連（`relation`）と
違って操作で張るものではなく、本文とコメントから導出する。

- 言及はチケット単位で持つ。同じチケットの本文と複数のコメントで言及しても `mention` は 1 行で、すべての言及が消えたときに削除する
- 本文の作成・編集とコメントの追加・編集・削除のたびに、そのチケットの本文と削除されていないコメントから言及先を求め直し、
  `mention` との差分を同じトランザクションで追加・削除する
- 追加・削除のたびに、言及されたチケットに `mentioned` / `mention_removed` の履歴を残し、`last_activity_at` を更新する
- `created_at` は最初に言及された時刻で、編集しても言及が残っている間は変わらない
- `#123` は Web UI でリンクに置き換えるもの（§6.4）と同じで、コード・リンク・画像・HTML の中のものは言及としない。
  Markdown は goldmark でパースする
- 自分自身と存在しないチケットへの言及は無視する
- この機能を入れる前の本文・コメントからは作らない

### 3.4 コーディングエージェントのセッション

どのマシンのどのセッションでチケットに取り組んだかを後から辿れるように、コーディングエージェントのセッションをチケットに記録する。
セッションはどのエージェントでもマシンローカルに保存されていて他のマシンからは再開できないので、ホスト名と組で持つ。

| エージェント | セッション ID の形式 | 再開方法 |
| --- | --- | --- |
| Claude Code | UUID | `claude --resume <id>` |
| Codex CLI | UUID | `codex resume <id>` |
| Gemini CLI | UUID | `gemini --resume <id>` |
| OpenCode | `ses_` + 26 文字 | `opencode --session <id>` |

- セッション ID はエージェントによって形式が異なるので、解釈せずに文字列として保存する
- 同じ形式のエージェントがあり ID からエージェントを判別できないので、エージェントの種類を enum で持つ
- 登録は MCP から行う。同じチケット・ホスト・セッション ID の登録は何もしない
- チケットに付随する情報として扱い、操作履歴には残さず `last_activity_at` も更新しない。削除はできない

## 4. 操作主体と認証

### 4.1 人間

heimdallr が付与する `X-Auth-Token` ヘッダーの JWT（ES256 署名）を、heimdallr の公開鍵で検証する。公開鍵は起動時に
heimdallr の internal API（`/internal/publickey`）から取得する。検証は `go/http/authproxy` のミドルウェアが行い、
検証できたクレームを context に入れる。

JWT の `sub` クレームに heimdallr のユーザー ID が入っているので、これを `actor.subject` と突き合わせる。リクエストごとに
DB を引かないように、actor は書き込みなど actor が必要な操作で初めて引き、存在しなければその時点で human の actor を
作成する。heimdallr を通過できた時点で利用を許可されているとみなし、事前登録は不要とする。

### 4.2 エージェント

エージェントは所有者である人間の資格情報で heimdallr を通過し、人間と同じく `X-Auth-Token` の JWT を受け取る。
エージェントが heimdallr をどう通過するかは heimdallr 側で扱い、issue tracker は関与しない。issue tracker は独自の
トークンを持たず、認証は heimdallr に任せる。

- `/mcp` へのリクエストはエージェントの操作として扱う。JWT の `sub` の human の actor を所有者とする `agent` の actor を操作主体にする
- エージェントの actor は人間 1 人につき 1 つで、`owner_id` で所有者を引く。`subject` は持たない
- 人間と同じく、actor は必要な操作で初めて引き、存在しなければその時点で所有者の human の actor とともに作成する。
  同時に作成されたときは UNIQUE(`owner_id`) で 1 つに決まる
- 名前は所有者の名前に ` (agent)` を付けたものにする

同じ `sub` でも Connect API からは human、`/mcp` からは agent として記録されるので、履歴ではどちらの操作かを区別できる。

### 4.3 権限

| 操作 | human | agent | system |
| --- | --- | --- | --- |
| チケットの操作（作成・編集・コメント・関連・添付） | ○ | ○ | ステータス変更と紐づけのみ |
| 分類の管理 | ○ | × | × |
| テンプレートの管理 | ○ | ○ | × |

認可は `issue` の各操作の入り口で `actor.kind` を見て行う。

コメントは他人のものも編集・削除できる。編集ごとの差分と削除の履歴が残るため、投稿者に制限しない。

## 5. 差分との紐づけ

### 5.1 Webhook

GitHub に issue tracker 専用の webhook を登録し、`push` / `pull_request` を受け取る。Build の webhook とは
独立させる。署名検証は Build と同じく前段の heimdallr が行う。

受信と処理の流れは Build の `go/build/webhook` と同じなので、アプリケーションに依存しない部分を
`go/github/webhook` にフレームワークとして取り出し、Build と issue tracker の両方から使う。

| 区分 | 内容 | 現在の Build での実装 |
| --- | --- | --- |
| フレームワーク | `Handler`: ヘッダーから event type と delivery ID を取り出して INSERT。delivery ID の重複は再送とみなして 200 | `handler.go` |
| フレームワーク | `Notifier`: INSERT 後に同一プロセスの `Scheduler` を起こす | `handler.go` |
| フレームワーク | `Scheduler`: PENDING / FAILED の event をポーリングし、期限切れを EXPIRED に、未登録の event type を SKIPPED にして、残りを `Reconciler` に渡す | `scheduler.go` |
| フレームワーク | `Reconciler` インターフェース、`Claim` / `Finalize`、`status` カラムへの進捗の読み書き | `webhook.go` |
| アプリケーション | event type ごとの `Reconciler` の実装、管理対象リポジトリの判定 | `push.go` など |
| アプリケーション | `github_event` テーブルとその DAO | `database` |

DAO は protoc-ddl がアプリケーションごとに生成するため、フレームワークは DAO に直接依存せず、次の `Store`
インターフェースを通して `github_event` を読み書きする。各アプリケーションは自分の DAO をこれに合わせる薄い
アダプタを持つ。

```go
type Store interface {
	Create(ctx context.Context, ev *Event) error // delivery ID の重複は ErrDuplicateDelivery を返す
	ListByState(ctx context.Context, states ...State) ([]*Event, error)
	Claim(ctx context.Context, ev *Event) (bool, error) // state が PENDING / FAILED のときだけ PROCESSING に更新できたかを返す
	Update(ctx context.Context, ev *Event) error
}
```

取り出すときに Build の実装から 2 点を変える。

- `Claim` を条件付き UPDATE にする。現在の Build は無条件に更新し、リーダーだけが処理する前提で排他している。条件付きにすれば複数の worker が同時に動いても同じ event を二重に処理しない
- 処理途中で止まった PROCESSING の event の回収を、起動時に全件を PENDING に戻す方式から、`updated_at` が一定時間より古いものだけを戻す方式にする。起動時の一括リセットは、他のレプリカが処理中の event まで戻してしまうため

Build はリーダー選出を残したままこのフレームワークに移行でき、挙動は変わらない。

issue tracker が扱うリポジトリは Build に登録されたリポジトリとし、issue tracker 自身はリポジトリを管理しない。
Reconciler はペイロードのリポジトリの URL を Build の `ListRepositories` で探し、登録されていなければ event を
SKIPPED にする。Build に接続できないときは FAILED にして再試行する。

### 5.2 trailer の解釈

コミットメッセージの最後の段落を trailer とみなし、`Refs: <番号>` / `Fixes: <番号>` の行を取り出す。
番号は `123` と `#123` のどちらの書き方も受け付ける。1 コミットに複数行あってよい。存在しないチケット番号は無視してログに残す。

### 5.3 push

- `ref` がペイロードの `repository.default_branch` と一致する push だけを処理する
- ペイロードの各コミットについて trailer を読み、`commit_link` を作成し `commit_linked` の履歴を残す
- `Fixes:` を持つコミットがあれば、対象チケットのステータスが Done 以外のとき Done にする
- これらはすべて system を操作主体として `issue` を呼ぶ。コミットの author は `commit_link.author` に入り、表示で使う
- 同じコミットの再処理は UNIQUE 制約で冪等にする

change-id は GitHub の API からは取れないため、git-data-service から取得する。jj はコミットオブジェクトの
`change-id` ヘッダーに change-id を書くので、git-data-service の `Commit` に `change_id` を追加し、go-git の
`ExtraHeaders` から埋める。push 直後は git-data-service がまだ fetch していない可能性があるので、
`commit_link.change_id` は null で作り、worker が後から埋める。

worker はサーバーの中で 1 分ごとに `change_id` が null の `commit_link` を取り出し、Build の `ListRepositories` で引いた
リポジトリ名で git-data-service の `GetCommit` を呼ぶ。git-data-service はリポジトリやコミットが見つからないときに
`NotFound` を返すので、まだ fetch されていないコミットは null のまま次の回に再試行する。jj 以外で作ったコミットは
change-id を持たないので空文字を入れ、null（未取得）と区別して再び問い合わせないようにする。既存の行の backfill も
同じ worker で行う。

### 5.4 pull_request

- `opened` / `reopened` / `synchronize` / `edited` で、GitHub API (`pulls/{number}/commits`) から PR のコミットを取得し、trailer を読む
- PR の本文も読む。本文には trailer という概念がないので、最後の段落に限らずすべての行から `Refs: <番号>` / `Fixes: <番号>` を取り出す。行の途中に書かれたものは拾わない
- コミットの trailer か本文の `Refs:` / `Fixes:` のいずれかで参照されたチケットに `pull_request_link` を作成する
- `closed` では、紐づいているすべての `pull_request_link` の `state` を closed / merged に更新する
- base がデフォルトブランチの PR がマージされたときは、本文の `Fixes:` で参照されたチケットのステータスが Done 以外なら Done にする。
  コミットの `Fixes:` はデフォルトブランチへの push で処理する（§5.3）
- `synchronize` で `head_sha` を更新する
- trailer が後から消えても紐づけは削除しない

### 5.5 CI 結果

CI 結果は Build の gRPC API から取得し、issue tracker には保存しない。`GetTicket` のたびに、commit は `sha`、
PR は `head_sha` を revision として Build の Task を引き、ジョブ名・成否・実行中かどうかと Build ダッシュボードの
Task へのリンクを表示する。GitHub のコミットステータスを経由しないので、`github_status` を有効にしていない
ジョブの結果も表示できる。

Build 側には次の変更が要る。

- `RequestListTasks` に `revisions` を追加する。DB には Task を revision で引く `ByRevision` が既にあるので、API に露出するだけでよい
- `commit_link` / `pull_request_link` は Build のリポジトリ ID を持つので、`repository_ids` と `revisions` を指定して `ListTasks` を呼ぶ

Build に接続できないときは CI 結果を表示せず、チケットの表示自体は失敗させない。

## 6. インターフェース

### 6.1 Connect API

Web UI と CLI は同じ Connect サービスを使う。

| 分類 | RPC |
| --- | --- |
| チケット | `ListTickets`, `GetTicket`, `CreateTicket`, `UpdateTicket`（タイトル・本文・ステータス・優先度・分類・担当を部分更新） |
| 関連 | `AddRelation`, `RemoveRelation` |
| セッション | `AddAgentSession` |
| コメント | `AddComment`, `EditComment`, `DeleteComment` |
| 添付 | `ListAttachments`（アップロード・ダウンロードは §7 の HTTP エンドポイント） |
| 履歴 | `ListActivities`, `ListTicketRevisions`, `ListCommentRevisions`（本文・コメントの編集前の版と編集者） |
| 管理 | `ListComponents`, `SaveComponent` |
| テンプレート | `ListTemplates`, `SaveTemplate`, `DeleteTemplate` |
| その他 | `ListActors`, `WhoAmI` |

`GetTicket` は関連（逆方向の導出を含む）・バックリンク・紐づいた commit / PR と CI 結果・添付・コーディングエージェントのセッションをまとめて返す。

### 6.2 MCP

MCP は Streamable HTTP でサーバーの `/mcp` に公開する。実装には公式の Go SDK（`github.com/modelcontextprotocol/go-sdk`）
を使う。`/mcp` も Connect API と同じく heimdallr の JWT を検証し、操作主体はエージェントになる（§4.2）。

tool は Connect API の RPC と 1 対 1 に対応させ、`go/issue/server` の Connect のハンドラを呼ぶ。入力の検証・エラー・
レスポンスの変換を Connect API と共有するので、挙動が一致する。

| tool | RPC |
| --- | --- |
| `list_tickets`, `get_ticket`, `create_ticket`, `update_ticket` | `ListTickets`, `GetTicket`, `CreateTicket`, `UpdateTicket` |
| `add_relation`, `remove_relation` | `AddRelation`, `RemoveRelation` |
| `add_agent_session` | `AddAgentSession` |
| `add_comment`, `edit_comment`, `delete_comment` | `AddComment`, `EditComment`, `DeleteComment` |
| `list_activities`, `list_ticket_revisions`, `list_comment_revisions` | `ListActivities`, `ListTicketRevisions`, `ListCommentRevisions` |
| `list_components`, `list_actors`, `who_am_i` | `ListComponents`, `ListActors`, `WhoAmI` |
| `list_templates`, `save_template`, `delete_template` | `ListTemplates`, `SaveTemplate`, `DeleteTemplate` |
| `list_attachments` | `ListAttachments`（添付の実装後に追加する） |

- `create_ticket` / `update_ticket` は分類と担当を ID で受け取るので、ID を引くために `list_components` / `list_actors` を公開する
- 分類の変更（`SaveComponent`）は人間だけができる操作なので公開しない
- 入力は tool ごとの struct で受け取って Connect のリクエストに詰め替える。proto の生成コードの struct は内部の
  フィールドを持ち、入力のスキーマにそのまま使えないため。ステータスと優先度は §8 のフィルタと同じ `OPEN` や `P1` の文字列で受け取る
- 結果は Connect のレスポンスを protojson で返す。エラーは tool のエラーとして返し、エージェントが理由を読めるようにする
- 添付のアップロード・ダウンロードは HTTP エンドポイントで行うので（§7）、tool は持たない

### 6.3 CLI

`go/cmd/issue` に `go/cli` で実装する。サブコマンドは `list` / `show` / `create` / `edit` / `comment` /
`relate` / `attach`。本文やコメントは `$EDITOR` か標準入力から受け取る。認証方式は今後の課題とする（§11）。

### 6.4 Web UI

`ts/apps/issue` に Build のフロントエンドと同じ構成（React + Vite + TanStack Router + TanStack Query + Connect）
で作る。画面はモックと同じく一覧・新規作成・詳細の 3 つに、テンプレートの選択と管理画面（分類・テンプレート）を加える。

- URL は `/<番号>` がチケット詳細、`/` が一覧、`/new` が新規作成。サーバーは未知のパスに `index.html` を返す
- 管理画面は `/admin` 以下に置き、左のメニューで管理する対象を切り替える。分類は `/admin/components`、テンプレートは `/admin/templates`
- ヘッダーの新規作成は Split button にし、▼ のメニューでテンプレートを選ぶと、タイトル・本文・分類を埋めた `/new?template=<id>` に移る
- 一覧の検索条件は §8 のフィルタとして URL の `?filter=` に持つ
- Markdown はクライアントでレンダリングしてサニタイズする。`#123` は `/123` へのリンクに置き換える
- 詳細のサイドバーに、関連と並べてバックリンク（言及元のチケットの番号とタイトル）を表示する。言及された時刻は履歴に表示する
- 詳細のサイドバーに、コーディングエージェントのセッション（エージェント・ホスト名・セッション ID）を表示する

## 7. 添付ファイル

- アップロードは `POST /attachments?ticket=<番号>`、ダウンロードは `GET /attachments/<id>` の HTTP エンドポイントで行う。Connect を使わないのは、上限のないファイルをストリームで扱うため
- サーバーはリクエストボディを `go/storage` の `PutReader` でそのまま MinIO に流す。メモリやローカルディスクに溜めない
- オブジェクトキーは `attachments/<attachment.id>` とし、ファイル名は DB にだけ持つ
- アップロードの完了後に `attachment` の INSERT と `attachment` 履歴を記録する。途中で失敗したオブジェクトは削除する

## 8. 一覧と検索

検索条件は AIP-160 のフィルタで表し、Web UI・CLI・MCP で共通に使う。Connect API では `ListTickets` の `filter` で受け取る。

```
(status = OPEN OR status = IN_PROGRESS) AND priority <= P1 AND component = build クラッシュ
```

- フィールドは `status` / `type` / `priority` / `component` / `assignee` / `reporter` / `title` / `body`
- `component` / `assignee` / `reporter` は名前で指定する。`assignee:*` は担当が設定されていること
- `type` は `BUG` / `FEATURE` / `TASK` / `OPERATION`。`type:*` は種別が設定されていること
- `priority` は比較演算子を使える。P0 が最も高いので、`priority < P2` は P0 と P1
- フィールドを指定しない値は、タイトル・本文・削除されていないコメントに部分一致させる

フィルタは実行時に条件が決まるので、静的にクエリを宣言する protoc-ddl の DAO では扱わない。
`go/issue/filter` が einride/aip のパーサーで構文解析し、ticket テーブルに対する WHERE 句に変換する。
部分一致は `LIKE` で行う。内部向けでチケット数は多くても数千程度と見込み、
日本語の分かち書きが必要になる FULLTEXT インデックスや外部の検索エンジンは持ち込まない。
件数が増えて遅くなった時点で見直す。

並び順は `last_activity_at`（コメントを含む最後の操作の時刻）の降順。

ページングは AIP-158 の `page_size` / `page_token` / `next_page_token` で行う。`page_size` の既定は 100、上限は 500。
`page_token` は `filter` と `order_by` とオフセットを持ち、別の `filter` や `order_by` と組み合わせると INVALID_ARGUMENT にする。
オフセット方式なので、ページを移る間にチケットが更新されると重複や抜けが起きうるが、内部向けの件数では許容する。

## 9. デプロイ

- サーバーとフロントエンドのコンテナイメージを `containers/` に追加し、マニフェストを `manifests/` に置く
- MariaDB と MinIO は既存のものに DB とバケットを追加して使う
- heimdallr にバックエンドを登録する。`/webhook` は GitHub の署名検証、それ以外は人間（エージェントは所有者の人間）の認証を通す

### 9.1 ローカル開発

`bazel run //:env_issue`（monodev）が MySQL を起動する。`make run-issue` は `--public-key-url` を指定しないので、
サーバーは heimdallr なしで動き、すべてのリクエストを dev@example.com の人として扱う。

サーバーは heimdallr・Build・git-data-service・GitHub がない環境（kind での動作確認など）でも動く。

- `--public-key-url` がなければ heimdallr なしで動き、すべてのリクエストを `--subject`（既定は dev@example.com）の人として扱う
- `--build-api` がなければ webhook を受け付けない。リポジトリは Build が管理しているので、Build なしではリンクを作れない
- `--git-data-service` がなければ change-id を取得しない（取得には `--build-api` も要る）
- GitHub の認証情報がなければ認証なしで GitHub API を呼ぶので、公開リポジトリの pull request だけを扱える

```
bazel run //:env_issue
make run-issue-migrate
make run-issue
make run-issue-ui
```

| ポート | 内容 |
| --- | --- |
| 8094 | サーバー |
| 8302 | Web UI の開発サーバー。API へのリクエストはサーバーに中継する |

Claude Code からは MCP を次のように登録する。

```
claude mcp add --transport http issue http://127.0.0.1:8094/mcp
```

webhook の処理ではリポジトリを Build の API で判定するので、`make run-api` で起動した Build（127.0.0.1:8081）に
接続する。Build が起動していなくてもサーバーは動き、受け取った event は `--event-max-processing-duration`（既定は
30 分）を過ぎるまで再試行される。pull_request の処理では PR のコミットを GitHub API で取得するので、Build と同じ
GitHub App の認証情報（Makefile の `GITHUB`）を渡す。change-id は Build と同じ git-data-service（127.0.0.1:8056）
から取得する。

### 9.2 デプロイ

マニフェストは `manifests/deploy/issue` に kustomize で置く。Build のアーティファクト収集
（`docs/working/artifact-collection.md`）に乗せ、default branch のビルドで `cd_artifact` が `base` のマニフェストと
コンテナイメージを zot に保存する。環境ごとの overlay は GitOps のリポジトリ（ops）で、公開した `base` を元に作る。

| ディレクトリ | 内容 |
| --- | --- |
| `base` | issue-server・Web UI（`simple-http-server`）・MariaDB の StatefulSet・migrate の Job。`cd_artifact` で zot に保存する |
| `local` | heimdallr・Build・git-data-service・GitHub のないクラスタ（kind など）向け。それらのフラグを渡さずに動かす（§9.1） |

- イメージは `containers/issue` で作り、マニフェストはビルド時にイメージの digest で参照する（`kustomization` の `images`）
- ops の overlay では、namespace、heimdallr の公開鍵・Build の API・git-data-service のアドレス、GitHub App の認証情報を与える
- heimdallr の `Backend` も ops に置く。パスで振り分け、`/mono.issue.Issue/`・`/mcp`・`/webhook` を issue-server に、それ以外を
  Web UI に向ける。GitHub の webhook の登録も `Backend` で行う
- migrate の Job は、`kustomization` が名前の末尾にイメージの digest の先頭 6 文字を付け、イメージごとに別の Job にする（ops の
  Build と同じ）。終わった Job は `ttlSecondsAfterFinished` で消える

## 10. 実装順序

1. スキーマ・`issue`・Connect API（チケット・関連・コメント・履歴）
2. Web UI（一覧・詳細・新規作成）と heimdallr による人間の認証
3. 本文とコメントの編集履歴の表示（Connect API と Web UI）
4. `go/github/webhook` の切り出しと Build の移行
5. Webhook（push / pull_request）と trailer の解釈
6. Build API への `revisions` 追加と CI 結果の表示
7. 添付ファイル
8. MCP とエージェントの actor
9. CLI
10. git-data-service への `change_id` 追加と backfill

## 11. 今後の課題と未決事項

今後の課題:

- CLI の認証方式

未決事項:

- Won't Fix のチケットに `Fixes:` が来たときも Done にするか（本設計では Done にする）
