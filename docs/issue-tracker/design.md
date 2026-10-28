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
     │  人間: X-Auth-Token (JWT) を付与              │
     │  webhook: 署名検証                            │
     └──────────────────────┬────────────────────────┘
                            ▼
     ┌──────────────────── server ────────────────────┐
     │  Connect API   MCP endpoint   /webhook          │
     │        └────────┬────┘            │             │
     │                 ▼                 ▼             │
     │               service ◀── event worker          │
     └─────────┬───────────────────────┬──────────────┘
               ▼                       ▼
            MariaDB              MinIO (添付)

     event worker ──▶ GitHub API (PR のコミット一覧)
                  ──▶ git-data-service (change-id の取得)
     service      ──▶ Build API (CI 結果)
```

サーバーは 1 バイナリで、複数レプリカで動かせる。Webhook は受信と処理を分け、受信時は DB に INSERT して即 200 を
返し、同一プロセス内の worker が処理する。worker は全レプリカで動き、event の取得は
`UPDATE ... SET state = PROCESSING WHERE id = ? AND state = PENDING` の更新件数で奪い合う。
処理自体も UNIQUE 制約で冪等にしてあるので、リーダー選出は持たない。この受信と処理の仕組みは Build と共通化する（§5.1）。

### 2.2 実装上の約束

- チケットに関するすべての書き込みは `service` を経由する。Connect API・MCP・event worker は `service` を
  呼ぶだけで、DB を直接更新しない。これにより「人間とエージェントで同じことができる」ことと
  「すべての操作が履歴に残る」ことを 1 箇所で保証する
- `service` の各操作は、チケットの更新と操作履歴の INSERT を同一トランザクションで行う
- 操作主体は `context.Context` で受け渡し、`service` は引数で actor を受け取らない

### 2.3 パッケージ構成

| パッケージ | 責務 |
| --- | --- |
| `proto/issue` | Connect API の定義 |
| `go/issue/database`, `go/issue/database/dao` | DB スキーマ（protoc-ddl の `schema.proto`）と生成コード、DAO |
| `go/issue/service` | チケット操作と管理操作のビジネスロジック・認可・操作履歴の記録 |
| `go/issue/server` | HTTP サーバー。認証（heimdallr JWT）、Connect・MCP・Webhook・添付のハンドラ |
| `go/issue/webhook` | push / pull_request の Reconciler と trailer の解釈 |
| `go/github/webhook` | Webhook の受信・永続化・非同期処理のフレームワーク。Build と共用する（§5.1） |
| `go/issue/cmd/server` | サーバーのエントリポイント |
| `go/cmd/issue` | 人間向け CLI |
| `ts/apps/issue` | Web UI |

## 3. データモデル

MariaDB を使い、スキーマは Build と同じく protoc-ddl で `go/issue/database/schema.proto` から生成する。マイグレーションも Build と同じく
別の `migrate` Job で行い、サーバーは起動時にスキーマハッシュの一致を待つ。

### 3.1 テーブル

| テーブル | 主なカラム | 備考 |
| --- | --- | --- |
| `ticket` | `id` (sequence), `title`, `body`, `status`, `priority` (null 可), `component_id` (null 可), `assignee_id` (null 可), `reporter_id`, `created_at`, `updated_at` | `id` がそのままチケット番号 |
| `ticket_body_revision` | `id`, `ticket_id`, `body`, `created_at` | 本文の編集ごとの全文 |
| `actor` | `id`, `kind` (human / agent / system), `name`, `owner_id` (agent のみ), `subject` (human のみ、heimdallr のユーザー ID) | system は固定の 1 行 |
| `agent_token` | `id`, `actor_id`, `name`, `token_hash`, `created_at`, `last_used_at`, `revoked_at` | トークンは SHA-256 のみ保存 |
| `component` | `id`, `name` | ソフトウェア分類のマスタ |
| `repository` | `id`, `name`, `url` | GitHub の URL。webhook とは URL で突き合わせる |
| `relation` | `from_id`, `type`, `to_id` | UNIQUE(`from_id`, `type`, `to_id`) |
| `comment` | `id`, `ticket_id`, `actor_id`, `body`, `created_at`, `updated_at`, `deleted_at` | 削除は `deleted_at` を立てる論理削除 |
| `comment_revision` | `id`, `comment_id`, `body`, `created_at` | コメントの編集ごとの全文 |
| `attachment` | `id`, `ticket_id`, `actor_id`, `name`, `content_type`, `size`, `object_key`, `created_at` | |
| `commit_link` | `ticket_id`, `repository_id`, `sha`, `change_id` (null 可), `subject`, `author`, `trailer` (refs / fixes), `committed_at` | UNIQUE(`ticket_id`, `repository_id`, `sha`) |
| `pull_request_link` | `ticket_id`, `repository_id`, `number`, `title`, `author`, `state` (open / closed / merged), `head_sha` | UNIQUE(`ticket_id`, `repository_id`, `number`) |
| `activity` | `id`, `ticket_id`, `actor_id`, `kind`, `from_value`, `to_value`, `ref_id`, `created_at` | 操作履歴 |
| `github_event` | `id`, `delivery_id`, `event_type`, `action`, `payload`, `state`, `status`, `last_error`, `created_at`, `updated_at` | UNIQUE(`delivery_id`)。カラムは Build の `github_event` と揃える |

- ステータス・優先度・関連種別・trailer 種別は proto の enum で持つ
- `relates_to` は対称なので `from_id < to_id` に正規化して保存し、二重登録を防ぐ
- commit / PR の URL は保存せず、`repository.url` と sha / 番号から表示時に組み立てる
  (`<url>/commit/<sha>`, `<url>/pull/<number>`)

### 3.2 操作履歴

`activity.kind` は次のいずれか。値の変化は `from_value` / `to_value`、対象のレコードは `ref_id` に入れる。

| kind | from / to | ref_id |
| --- | --- | --- |
| create | — | — |
| title, status, priority, component, assignee | 変更前 / 変更後 | — |
| body | — | 変更後の本文を保存した `ticket_body_revision.id` |
| relation_added, relation_removed | — / `<type>:<相手の番号>` | — |
| comment | — | `comment.id` |
| comment_edited | — | 変更後のコメントを保存した `comment_revision.id` |
| comment_deleted | — | `comment.id` |
| attachment | — | `attachment.id` |
| commit_linked, pull_request_linked | — | `commit_link` / `pull_request_link` の行 |

本文とコメントの編集は履歴として差分を追えるよう、編集ごとに全文をそれぞれ `ticket_body_revision` /
`comment_revision` に保存する。削除したコメントは履歴から参照され続けるため、行は消さずに論理削除にする。

操作主体が人間かエージェントかは `actor.kind` で判定する。actor の `kind` と `owner_id` は作成後に変更しない
ため、履歴側にコピーしなくても当時の値が復元できる。

## 4. 操作主体と認証

### 4.1 人間

heimdallr が付与する `X-Auth-Token` ヘッダーの JWT（ES256 署名）を、heimdallr の公開鍵で検証する
（`go/cmd/unifi-proxy` と同じ方式）。JWT の `sub` クレームに heimdallr のユーザー ID が入っているので、これを
`actor.subject` と突き合わせ、初回アクセス時に
human の actor を作成する。heimdallr を通過できた時点で利用を許可されているとみなし、事前登録は不要とする。

### 4.2 エージェント

- エージェントトークンは人間が Web UI から発行する。発行した人間がそのトークンの actor の所有者になる
- 発行時に `agent` の actor を 1 つ作り、トークンはその actor に紐づく。1 actor に複数トークンを持てる（ローテーション用）
- トークンは発行時に 1 度だけ表示し、DB には SHA-256 のみ保存する
- 所有者はトークンを失効でき、失効後は `revoked_at` を立てて拒否する

リクエストでのトークンの渡し方と、エージェントのリクエストが heimdallr をどう通過するかは今後の課題とする（§11）。

### 4.3 権限

| 操作 | human | agent | system |
| --- | --- | --- | --- |
| チケットの操作（作成・編集・コメント・関連・添付） | ○ | ○ | ステータス変更と紐づけのみ |
| エージェントトークンの発行・失効 | ○（自分が所有するもの） | × | × |
| リポジトリ・分類の管理 | ○ | × | × |

認可は `service` の入り口で `actor.kind` を見て行う。

## 5. 差分との紐づけ

### 5.1 Webhook

GitHub に issue tracker 専用の webhook を登録し、`push` / `pull_request` を受け取る。Build の webhook とは
独立させる。署名検証は Build と同じく前段の heimdallr が行う。

受信と処理の流れは Build の `go/build/webhook` と同じなので、アプリケーションに依存しない部分を
`go/github/webhook` にフレームワークとして取り出し、Build と issue tracker の両方から使う。

| 区分 | 内容 | 現在の Build での実装 |
| --- | --- | --- |
| フレームワーク | `Handler`: ヘッダーから event type と delivery ID を取り出し、リポジトリの判定を経て INSERT。delivery ID の重複は再送とみなして 200 | `handler.go` |
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

issue tracker では、`repository` に登録されていないリポジトリの event は INSERT せずに 200 を返す。

### 5.2 trailer の解釈

コミットメッセージの最後の段落を trailer とみなし、`Refs: <番号>` / `Fixes: <番号>` の行を取り出す。
番号は `123` と `#123` のどちらの書き方も受け付ける。1 コミットに複数行あってよい。存在しないチケット番号は無視してログに残す。

### 5.3 push

- `ref` がペイロードの `repository.default_branch` と一致する push だけを処理する
- ペイロードの各コミットについて trailer を読み、`commit_link` を作成し `commit_linked` の履歴を残す
- `Fixes:` を持つコミットがあれば、対象チケットのステータスが Done 以外のとき Done にする
- これらはすべて system を操作主体として `service` を呼ぶ。コミットの author は `commit_link.author` に入り、表示で使う
- 同じコミットの再処理は UNIQUE 制約で冪等にする

change-id は GitHub の API からは取れないため、git-data-service から取得する。jj はコミットオブジェクトの
`change-id` ヘッダーに change-id を書くので、git-data-service の `Commit` に `change_id` を追加し、go-git の
`ExtraHeaders` から埋める。push 直後は git-data-service がまだ fetch していない可能性があるので、
`commit_link.change_id` は null で作り、worker が後から埋める。

### 5.4 pull_request

- `opened` / `reopened` / `synchronize` / `edited` で、GitHub API (`pulls/{number}/commits`) から PR のコミットを取得し、trailer を読む
- `Refs:` / `Fixes:` のいずれかで参照されたチケットに `pull_request_link` を作成する（PR ではステータスを変更しない）
- `closed` では、紐づいているすべての `pull_request_link` の `state` を closed / merged に更新する
- `synchronize` で `head_sha` を更新する
- trailer が後から消えても紐づけは削除しない

### 5.5 CI 結果

CI 結果は Build の gRPC API から取得し、issue tracker には保存しない。`GetTicket` のたびに、commit は `sha`、
PR は `head_sha` を revision として Build の Task を引き、ジョブ名・成否・実行中かどうかと Build ダッシュボードの
Task へのリンクを表示する。GitHub のコミットステータスを経由しないので、`github_status` を有効にしていない
ジョブの結果も表示できる。

Build 側には次の変更が要る。

- `RequestListTasks` に `revisions` を追加する。DB には Task を revision で引く `ByRevision` が既にあるので、API に露出するだけでよい
- issue tracker の `repository` と Build の `SourceRepository` は URL で対応づける。Build の `ListRepositories` で URL から Build のリポジトリ ID を引き、`repository_ids` と `revisions` を指定して `ListTasks` を呼ぶ

Build に接続できないときは CI 結果を表示せず、チケットの表示自体は失敗させない。

## 6. インターフェース

### 6.1 Connect API

Web UI と CLI は同じ Connect サービスを使う。

| 分類 | RPC |
| --- | --- |
| チケット | `ListTickets`, `GetTicket`, `CreateTicket`, `UpdateTicket`（タイトル・本文・ステータス・優先度・分類・担当を部分更新） |
| 関連 | `AddRelation`, `RemoveRelation` |
| コメント | `AddComment`, `EditComment`, `DeleteComment` |
| 添付 | `ListAttachments`（アップロード・ダウンロードは §7 の HTTP エンドポイント） |
| 履歴 | `ListActivities` |
| 管理 | `ListRepositories`, `SaveRepository`, `ListComponents`, `SaveComponent` |
| トークン | `ListAgentTokens`, `IssueAgentToken`, `RevokeAgentToken` |
| その他 | `ListActors`, `WhoAmI` |

`GetTicket` は関連（逆方向の導出を含む）・紐づいた commit / PR と CI 結果・添付をまとめて返す。

### 6.2 MCP

MCP は Streamable HTTP でサーバーの `/mcp` に公開する。tool はチケット操作の RPC と 1 対 1 に対応させ、
管理・トークン系は公開しない。tool の実装は `service` を呼ぶだけで、Connect API と同じ挙動になる。

### 6.3 CLI

`go/cmd/issue` に `go/cli` で実装する。サブコマンドは `list` / `show` / `create` / `edit` / `comment` /
`relate` / `attach`。本文やコメントは `$EDITOR` か標準入力から受け取る。認証方式は今後の課題とする（§11）。

### 6.4 Web UI

`ts/apps/issue` に Build のフロントエンドと同じ構成（React + Vite + TanStack Router + TanStack Query + Connect）
で作る。画面はモックと同じく一覧・新規作成・詳細の 3 つに、管理画面（リポジトリ・分類・トークン）を加える。

- URL は `/<番号>` がチケット詳細、`/` が一覧、`/new` が新規作成。サーバーは未知のパスに `index.html` を返す
- 一覧の検索条件はすべて URL のクエリに持つ（`?q=&status=&priority=&component=&assignee=`）
- Markdown はクライアントでレンダリングしてサニタイズする。`#123` は `/123` へのリンクに置き換える

## 7. 添付ファイル

- アップロードは `POST /attachments?ticket=<番号>`、ダウンロードは `GET /attachments/<id>` の HTTP エンドポイントで行う。Connect を使わないのは、上限のないファイルをストリームで扱うため
- サーバーはリクエストボディを `go/storage` の `PutReader` でそのまま MinIO に流す。メモリやローカルディスクに溜めない
- オブジェクトキーは `attachments/<attachment.id>` とし、ファイル名は DB にだけ持つ
- アップロードの完了後に `attachment` の INSERT と `attachment` 履歴を記録する。途中で失敗したオブジェクトは削除する

## 8. 一覧と検索

全文検索はタイトル・本文・コメントに対する `LIKE '%...%'` で実装する。内部向けでチケット数は多くても数千程度と
見込み、日本語の分かち書きが必要になる FULLTEXT インデックスや外部の検索エンジンは持ち込まない。
件数が増えて遅くなった時点で見直す。

フィルタ（ステータス・優先度・分類・担当）は AND で組み合わせ、各フィールド内は複数値の OR を許す。
並び順は `updated_at` の降順。

## 9. デプロイ

- サーバーとフロントエンドのコンテナイメージを `containers/` に追加し、マニフェストを `manifests/` に置く
- MariaDB と MinIO は既存のものに DB とバケットを追加して使う
- heimdallr にバックエンドを登録する。`/webhook` は GitHub の署名検証、それ以外は人間の認証を通す

## 10. 実装順序

1. スキーマ・`service`・Connect API（チケット・関連・コメント・履歴）
2. Web UI（一覧・詳細・新規作成）と heimdallr による人間の認証
3. `go/github/webhook` の切り出しと Build の移行
4. Webhook（push / pull_request）と trailer の解釈
5. Build API への `revisions` 追加と CI 結果の表示
6. 添付ファイル
7. エージェントトークンと MCP
8. CLI
9. git-data-service への `change_id` 追加と backfill

## 11. 今後の課題と未決事項

今後の課題:

- CLI の認証方式
- エージェントの認証方式（トークンの渡し方と、エージェントのリクエストが heimdallr をどう通過するか）

未決事項:

- Won't Fix のチケットに `Fixes:` が来たときも Done にするか（本設計では Done にする）
- 他人のコメントを編集・削除できるか
