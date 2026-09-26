# ゲームデータ（カタログ）

MAYAK が使うアイテム・マップ・トレーダー・タスク・ハイドアウトのデータは、tarkov.dev の公開 JSON（`https://json.tarkov.dev/`）から取得します。取得と保存は `internal/catalog` が一手に担い、アイテム照合（`internal/itemapi`）・タスク照合（`internal/questapi`）・アイテム欄（`internal/iteminfo`）・ハイドアウトはすべて同じスナップショットを共有します。

## 取得するリソース

URL は `https://json.tarkov.dev/<mode>/<resource>` です。

| 種別 | リソース | 欠けたとき |
|---|---|---|
| 必須 | `items`, `items_en`, `maps`, `maps_en`, `traders`, `traders_en`, `tasks`, `tasks_en`, `hideout`, `hideout_en` | 更新全体を失敗扱いにする |
| 任意（言語別） | `tasks_<lang>`, `items_<lang>`（現在は `tasks_ja`, `items_ja`） | 前回の内容を引き継ぐ。なければ英語のみで動作 |

- 任意リソースの言語は `internal/locale` の `Languages`（現在 `["ja"]`）から `locale.Resource(base, lang)` で組み立てます。言語を追加すると取得対象も増えます（[言語](languages.md)）。
- 任意リソースは `data` が空でないオブジェクトのときだけ採用されます。
- `Client.Get` は上記以外のリソース名を拒否します（`unsupported catalog resource`）。

## ゲームモード

| カタログのモード | 意味 |
|---|---|
| `regular` | 通常の PvP |
| `pve` | PvE |
| `pvp-season` | シーズン PvP |

設定 `GameMode` は `auto` / `regular` / `pve` / `pvp-season`（既定 `auto`、不正値は `auto` に正規化）。

### 自動判別（`auto`）

`effectiveCatalogMode`（`app_catalog.go`）は、`auto` のとき EFT ログから検出したプロファイルのモード（`status.Tracker.Mode`）を対応づけます。

| ログ上のモード | カタログのモード |
|---|---|
| `pve` | `pve` |
| `pvp` | `regular` |
| `seasonal` | `pvp-season` |
| 未検出 | `auto`（カタログ更新は行わない） |

- モードが未検出のまま手動更新すると `EFT game mode has not been detected; select a game mode or start log monitoring` を返します。
- 照合側の扱い: アイテムは `auto` を `regular` とみなし、タスクは `auto` のとき全モードを統合した一覧を使います（後述）。

## 更新の仕組み

- 更新間隔 `RefreshInterval` は **5 分**。スナップショットの `UpdatedAt` から 5 分未満なら、ネットワークに出ずにそのまま返します。
- 各リソースは並列に取得し、前回の `ETag` があれば `If-None-Match` を付けた条件付きリクエストにします。`304 Not Modified` なら前回のデータと ETag を再利用するので、変化のないリソースは再ダウンロードされません。
- HTTP タイムアウト 30 秒、1 レスポンス上限 64 MiB、`User-Agent: MAYAK/0.1.0`。200 以外・不正 JSON はエラー。
- 失敗後は `retryCooldown`（**1 分**）の間、強制でない更新は再試行せず、前回スナップショットと最後のエラーを返します。
- 同時に走る更新は `refreshMu` で直列化されます。
- 強制更新（`force`）は 5 分判定と 1 分クールダウンの両方を無視します。


### 検証

全リソースがそろってから `validate` を通したものだけを新しいスナップショットとして公開します。部分的・不正なダウンロードで、動いていたカタログを置き換えることはありません。

- 必須リソースそれぞれに空でない `data` オブジェクトがあること
- `items` / `maps` / `tasks` は入れ子の同名マップが空でないこと
- `traders` / `hideout` はエントリが空でないこと
- 件数（アイテム・マップ・トレーダー・タスク・ハイドアウト施設）と、`items` の `settings.scavCooldownSeconds`、`playerLevels` の件数をスナップショットに記録

## 保存と最終正常版（オフライン利用）

- 保存先: `os.UserConfigDir()` 配下の `Mayak\catalog\<mode>.json`（Windows では通常 `%AppData%\Mayak\catalog\regular.json` など）。
- 一時ファイル（`.catalog-*`）に書いて `Sync` 後にリネームする原子的な保存。ディレクトリは `0700`。
- 各リソースの本文と ETag、件数、`UpdatedAt` を含みます。
- 起動後そのモードで最初の更新時に、メモリになければディスクから読み込み、モード一致と `validate` を満たせば最終正常版として採用します。
- ダウンロードや検証に失敗しても、最終正常版があればそれを返して利用を続けます（オフライン時もこの保存データで動作）。保存だけ失敗した場合は、新しいカタログを使いつつエラーを返します。

## Host での自動更新と状態表示

`app_catalog.go` の `watchCatalog` は Windows の Host モードでのみ起動します（`startBrowserHostBackground`）。ブラウザの受信側（WebRTC クライアント）や終了中は更新しません。

更新のきっかけ:

| きっかけ | 強制 |
|---|---|
| 起動時（有効なモードが決まっていれば） | いいえ |
| 5 分ごとのティッカー | いいえ |
| 設定の `GameMode` 変更 | いいえ |
| ログで別のプロファイル／モードを検出 | いいえ |
| 設定画面の「データを更新」（`RefreshCatalog`） | はい |
| ハイドアウトの更新（`RefreshHideout`） | はい |

- 1 回の更新は 60 秒でタイムアウト。完了時にモードが変わっていたら結果を捨て、状態表示を更新前のものに戻します（その間に新しい更新が始まっていれば、そちらの表示に任せます）。
- スナップショットが得られたら、変わった部分の使い手だけを作り直します。部分ごとの版（`Snapshot.Version`、各リソースの ETag。無ければ長さと CRC32）をモードごとに覚えておき（`catalogChanged`）、前回と違うときだけ次を行います。
  - タスク（`catalog.TaskResources`: tasks・maps・traders とその各言語）: `questapi` を無効化。
  - アイテム（`catalog.ItemResources`: items とその各言語）: `itemapi` を無効化。フリマ価格が入っているので、ほぼ毎回変わります。
  - ハイドアウト（`catalog.HideoutResources`: hideout・items_en・traders とその英語名）: 施設一覧を読み直し（[ハイドアウト](hideout.md)）。items は価格で毎回変わるので対象にせず、新しいアイテムは items_en の変化で拾います。
  5 分ごとの確認で価格だけが変わったときは、タスク一覧とハイドアウトを作り直しません。
- `status.catalog`（`model.CatalogStatus`）の `state`:

| `state` | 意味 | 画面表示 |
|---|---|---|
| `loading` | 更新中 | 処理中 |
| `ready` | 最新（5 分以内） | 利用可能 |
| `stale` | 取得から 5 分以上（更新失敗で保存データを使用中） | 保存済みデータを使用中 |
| `error` | 使えるデータがない | 待機中 |

- Host 設定の状態カード（`frontend/src/main.tsx`）に、モード・状態・各件数・最終取得時刻・`lastError` と「データを更新」ボタンを表示します。
- ログ（カテゴリ `Catalog`）: 成功時は件数を `Info`、失敗時は `Warn` で記録し、最終正常版を使うときはその取得時刻も記録します。

## アイテム一覧（`internal/itemapi`）

- `ItemsForMode(mode)`: `""` / `auto` は `regular` として扱い、それ以外の不明モードはエラー。
- モードごとに **12 時間**メモリにキャッシュ。ただしカタログのタスク部分が変わると `Invalidate` されるので、実際はカタログのスナップショットに追従します。
- 名前・略称は `items_en` で英語化。`items_<lang>` の名前が英語と異なる場合、`Aliases` / `ShortAliases` に加え、`Names[lang]` に保持します（照合と表示言語に使用、[アイテム欄](item-panel.md)）。
- 言語リソースが取れなくても英語名で動作します。
- `NewWithSource` でカタログを取得元にします（アプリはこちら）。取得元なしの場合は JSON API を直接読みます（タイムアウト 30 秒）。

## タスク一覧（`internal/questapi`）

- `QuestsForMode(mode)`:
  - `regular` / `pve` / `pvp-season`: そのモードの `tasks`・`maps`・`traders` と英語名から構築し、補足タスクと Wiki タスクを加えます。
  - `""` / `auto`: `Quests()` が `regular` を基準に、`pve`・`pvp-season` にしかないタスクを ID 単位で追加した統合一覧を返します（マップ・トレーダー名は `regular` のもの）。
- キャッシュは統合一覧・モード別とも **12 時間**。カタログのアイテム部分が変わると `Invalidate` されます。
- 各タスクは ID・英語名・トレーダー名・マップ（`normalizedName`）・`normalizedName`・`wikiLink`・目標（説明と対象マップ）を持ちます。タスク自体にマップがなければ、最初にマップを持つ目標のマップを使います。
- 別言語名: `tasks_<lang>` にある名前が英語名と異なれば `Aliases` に追加します。表示名と Wiki リンクは英語（正規）のまま。言語リソースが欠けても一覧は使えます。
- 取得元なしの場合の HTTP タイムアウトは 20 秒。

### 補足タスク（`supplemental.go`）

tarkov.dev の公開カタログに載る前の新タスクを、コードに直接持っています。

| ID | 名前 | トレーダー | マップ |
|---|---|---|---|
| `mayak:preliminary-survey` | Preliminary Survey | Ref | `the-lab`（tarkov.dev のマップ名。ログ上は `laboratory`） |

- 正規化した名前が一覧に既にあれば追加しません。
- `NormalizedName` は空のままにし、tarkov.dev のまだ存在しないタスクページへ移動させません（Remote Control ではマップ送信にフォールバック、[タスクとマップ](tasks-and-maps.md)）。

### 公式 Wiki からの補足（`wiki.go`）

公式 Wiki（`escapefromtarkov.fandom.com`）は tarkov.dev より先に新タスクを載せるため、名前だけでも照合できるようにします。

- MediaWiki API（`https://escapefromtarkov.fandom.com/api.php`）で `Category:Quests` のメンバー（名前空間 0、500 件ずつ最大 10 ページ、追加日時付き）を取得します。
- `Category:Event content` と `Category:Historical content` に属するページ（過去イベント）は除外します。
- カテゴリへの追加が **直近 180 日**以内のものだけを採用します（Arena や過去イベントによる誤一致を避けるため）。
- `Category:Story chapters`（ストーリー章。tarkov.dev のカタログに無い）は追加日時にかかわらず採用します。
- 取得はバックグラウンドで行います。アプリは起動時に `WarmWiki()` で取得を始めます。
- 手元に一覧があれば、タスク一覧の構築は Wiki を待ちません（古ければ裏で取り直します）。まだ一覧が無く取得中のときだけ、その完了を最大 **10 秒**（または照合の context が終わるまで）待ちます。ストーリー章は Wiki にしか無いため、起動直後の最初の認識で一覧に章が無いと、1 回目だけ失敗して 2 回目に一致することになるからです。
- 成功した一覧は **12 時間**ごとに取り直します。取得に失敗した（200 以外の応答を含む）、または結果が空のときは前回の一覧（なければなし）を保ち、**30 分**後に再取得します。
- 取り直した一覧が前回と変わっていれば、タスク一覧のキャッシュを無効化し、次の照合から新しい一覧で作り直します。
- 既に名前で一致するタスクと、カテゴリ概要ページ `Quests` は追加しません。
- 追加されるタスクは ID `wiki:<タイトル>`、名前のみで、`WikiLink` はそのページ（空白を `_` に置換）。トレーダー・マップ・`NormalizedName` は持ちません。
- `EnableWiki()` を呼んだクライアントだけが有効（既定はオフ）。アプリは `NewApp` で有効にし、テストやツールはオフラインのままです。
