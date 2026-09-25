# タスクとマップ

タスク画面のスクリーンショットを認識するとタスクのページを開きます。EFT のログや座標付きスクリーンショットをもとに、tarkov.dev のマップも表示します。画像の分類と OCR については [スクリーンショット認識](recognition.md)、タブの動作全般は [ブラウザシェル](browser-shell.md) を参照してください。

## タスク認識後の流れ

`handleTaskAnalysis`（`app.go`）の処理です。

1. タスク画面と判定したら、OCR の結果を `QuestsForMode(effectiveCatalogMode(GameMode))` の一覧と照合します（一覧の中身は [ゲームデータ](catalog.md)）。
2. 上位 5 件を `QuestCandidates` として状態に載せます。
3. 最上位の一致度が **0.78 以上**なら確定し、状態に次の値を入れます。
   - `LastQuest`（英語名）、`QuestID`、`QuestTrader`、`QuestMap`
   - `QuestURL`: `normalizedName` があれば `https://tarkov.dev/task/<normalizedName>`
   - `QuestWikiURL`: カタログの `wikiLink`
   - `QuestObjectives`: 目標の説明と対象マップ（一覧にそのタスクが見つからなければ空にし、前のタスクの目標を残しません）
4. 確定したらタスク音、しなければエラー音を鳴らします。
5. 確定したら `showBrowserTask` がブラウザへ `browser:task` イベントを送ります。
6. Remote Control の送信（後述）を行います。

座標付きのスクリーンショットでも、アイテム画面でなくタスク画面らしければ先に照合します。一致度が 0.78 以上ならタスクとして扱い、座標は送りません。

Host 設定の「タスクを開く」（`OpenQuestPage`）は、最後に認識したタスクを現在のサイト設定でもう一度開きます。まだ認識したタスクがなければエラー（`no recognized task`）です。

## タスクサイトと URL（`app_quest_site.go`）

`browser:task` には `id`・`name`・`site`（Host の設定）と、3 サイトすべての URL（`urls`）が入ります。

| サイト（`QuestSite`） | URL |
|---|---|
| `tarkov-dev`（既定） | `QuestURL`。なければ `https://tarkov.dev/tasks/` |
| `official-wiki` | `QuestWikiURL`（`https://escapefromtarkov.fandom.com/wiki/...` の形式のときだけ）。それ以外は名前の空白を `_` にしてパスエスケープした `https://escapefromtarkov.fandom.com/wiki/<name>` |
| `japanese-wiki` | トレーダーがあれば `https://wikiwiki.jp/eft/<trader>/<name>`（どちらもパスエスケープ）。なければ `https://wikiwiki.jp/eft/?cmd=search&word=<name>` |

- 日本語 Wiki はページ名にカンマと ` [PVE ZONE]` 接尾辞を含まないため、名前からカンマを除き、末尾の ` [PVE ZONE]` を取り除きます（例: `Camera, Action!` → `Camera Action!`）。
- 不明なサイト値は `tarkov-dev` に正規化されます（`normalizeQuestSite`）。
- 補足タスクや公式 Wiki 由来のタスクは `normalizedName` を持たないため、tarkov.dev ではタスク一覧ページにフォールバックします。Wiki 由来のタスクはトレーダーもないため、日本語 Wiki では検索ページを開きます。
- アイテム欄の「必要なタスク」にも同じ 3 つの URL が付きます（`withTaskURLs`、[アイテム欄](item-panel.md)）。

## ブラウザでのタスクタブ

`receiveTask`（`frontend/src/browser/state.js`）の動作です。

- **開くサイト**: ブラウザ側の `questSite` が `host`（既定）なら Host から届いた `site`、そうでなければブラウザ側の選択を使います。
- **同じタスクのタブ**: 同じタスク ID・同じ URL のタブが既にあれば、そのタブをアクティブにします。スクリーンショットを繰り返し撮ってもタブは増えません。
- **`taskMode`**: 既定の `new` は新しいタブを追加します。`reuse` は固定（ピン留め）されていない最初のタスクタブを更新します。
- **上限**: タブが 80 個に達していれば開きません。
- **タスクタブのツールバー**:
  - サイト選択（`#task-site`）で 3 サイトの URL を切り替えられます。
  - 検索ボタンは、日本語 Wiki 表示中なら日本語 Wiki の検索、それ以外なら公式 Wiki の `Special:Search` でタスク名を検索します。

### Host のサイト設定との関係

- **Host モード（このPCで検出）**: ブラウザ設定「タスク」の「表示するサイト」は Host の `QuestSite` そのものを変更します（`PersistSettings`）。ブラウザ側の `questSite` は `host` に固定されます。以前ブラウザ側で別のサイトを選んでいた場合は、起動時にその値を Host 設定へ移します。
- **別の PC の Host に接続（WebRTC）**: Host から転送された `browser:task` を受け取り、「Hostの設定に従う」か、自分のサイト選択を使います。
- アイテム欄から開くタスクページも同じ規則です。Host モードでは Host の設定、クライアントではブラウザの選択またはアイテム情報に付いたサイトを使います。

## Remote Control（tarkov.dev）

tarkov.dev のマップ・タスクページは、`wss://socket.tarkov.dev` 経由で Remote Control の命令を受けます。

### 送信先 ID

| 種類 | 内容 |
|---|---|
| `RemoteTargets` | 設定で登録する外部の ID。各 ID に `Map` / `Tasks` の役割を指定。旧設定の `RemoteID` は `Remote 1`（両方の役割）へ移行 |
| `BrowserRemoteID` | 内蔵ブラウザのマップビュー用の ID |

- `BrowserRemoteID` は初回起動時に `A-Z0-9` の 12 文字を `crypto/rand` で生成して保存します（`app_browser_remote.go`）。設定を保存しても値は保たれます。tarkov.dev 自身の ID は 4 文字で、ほかのページとかぶりえます。この ID は MAYAK の中だけで使うので、かぶらず推測もされない長さにしています。
- MAYAK はこの ID の Remote Control セッションに、ページと同じ受け手としても接続します（`remote.Watch`）。この ID へは MAYAK しか送らないので、MAYAK が直前（15 秒以内）に送っていない命令が届いたら、ほかのページが同じ ID を使っていると判断します。そのときは新しい ID を生成して保存し、警告をログに出します。シェルは `browser:remote-id` を受けてマップビューを新しい ID で開き直します。ほかの端末からこの ID でマップビューを操作した場合も同じ扱いになり、ID が変わります。
- `BrowserRemoteID` はマップ系の命令（`map` チャンネル）の送信先にだけ加わります。タスクは内蔵ブラウザ自身が開くので、Remote Control では送りません。
- 内蔵ブラウザは tarkov.dev の `/map/` ページとマップ一覧（`/maps/`）にだけ `?connection=<ID>` を付けて Remote Control に接続させます。tarkov.dev はページを開いた時点で localStorage に残っている `sessionId` で接続するため、ドキュメントスクリプトはこの ID を localStorage にも先に書き込みます。
  - Host は tarkov.dev のスクリプトより先に実行するドキュメントスクリプトで付与します。
  - ブラウザシェルも固定マップタブを開くときに直接付けます（`viewURL`）。
  - タスクページは接続させません（マップ命令でページが移動してしまうため）。
  - 保存するタブ URL からはこのパラメータを取り除きます（`pageURL`）。

### タスク確定時の送信

`Tasks` 役割の送信先が 1 つ以上あり、一致度が 0.78 以上のとき、次のどちらかを送ります。

- `normalizedName` があれば `task/<normalizedName>` を送ります。
- なければ、タスクのマップが分かっていれば `map/<QuestMap>` を送ります（補足タスクなど）。このとき内蔵ブラウザにも `browser:map` を送ります。

サイト設定はタスクを開く場所を決めるだけで、Remote Control の送信は各送信先の役割だけで決まります。

## レイド状態とマップ（`internal/logdetect`）

### ログの監視

- 監視を始めると、Logs フォルダ以下で名前に `application` を含む最新の `.log` を 1 秒ごとに読み進めます。
- 最新ファイルは 2 秒ごとに探し直します。読み始めの位置は末尾 1 MiB です。
- 最初の読み込みで見つかったイベントは通知しません（起動前の出来事で音を鳴らさないため）。
- 起動時の状態は `LatestSnapshot` で復元します。レイド中なら開始時刻と走り抜けタイマーを再計算し、走り抜けの通知音も予約します（開始音などほかの通知音は鳴らしません）。
- Logs / Screenshots フォルダが未設定なら、起動時に `internal/eftdetect` が既定の場所やランチャー設定の `gamesRootDir` から自動検出します。

### イベント

| ログ行（小文字化して判定） | イベント |
|---|---|
| `\|application\|matchingcompleted:` | `MatchFound`（`real:` の値を待ち時間秒として記録） |
| `\|application\|gamestarted:` | `RaidStarted` |
| `usermatchover`、`\|application\|prepareselectedprofilelocally profileid:`、`\|application\|completeselectedprofile profileid:` | `RaidExited`（レイド中のときのみ） |

- `RaidState` は最新セッションの `application` / `backend` ログの末尾 4 MiB を読み、最後の開始行と終了行のどちらが新しいかで、レイド中かどうかを判定します。
- 座標付きスクリーンショットの送信可否など、「今レイド中か」の判定には常にこのログ由来の状態を使います。

### マップ名

`scene preset path:maps/<name>.bundle` または `Location: <name>` のうち最後に現れたものを読み取り、`_preset` を除いて tarkov.dev のマップ名に変換します。

| ログ上の名前 | マップ |
|---|---|
| `bigmap` | `customs` |
| `factory4_day` / `factory_day` | `factory` |
| `factory4_night` / `factory_night` | `night-factory` |
| `sandbox` / `groundzero` | `ground-zero` |
| `sandbox_high` | `ground-zero-21` |
| `sandbox_start` | `ground-zero-tutorial` |
| `shopping_mall` | `interchange` |
| `laboratory` / `laboratory_dark` | `the-lab` / `the-lab-dark` |
| `labyrinth` | `the-labyrinth` |
| `rezervbase` / `rezerv_base` | `reserve` |
| `tarkovstreets` / `streets` / `city` | `streets-of-tarkov` |

`customs`、`factory`、`interchange`、`lighthouse`、`reserve`、`shoreline`、`icebreaker`、`terminal`、`woods` はそのままの名前で認識します。

## マップを開く・移動する

### レイド開始時（`OpenMapOnRaidStart`、既定オン）

- ログでマップかレイド状態が変わると `handleMap` が現在のマップを状態に反映します。
- レイド中で `OpenMapOnRaidStart` がオンなら `sendMapTargets` を呼び、次の 2 つを行います。
  - 内蔵ブラウザへ `browser:map` を送ります。
  - `Map` 役割の送信先と `BrowserRemoteID` へ `map/<name>` を送ります。
- `RaidStarted` から 1.5 秒後に同じ送信をもう一度行います。tarkov.dev が再接続した直後で、最初の命令を受け取れないことがあるためです。

### 座標付きスクリーンショット（`NavigateMapOnShot`、既定オン）

1. アイテム画面・タスク画面でないことを確かめます。
2. ログでレイド中と分かるときだけ位置として扱います。レイド外の座標は無視します（EFT はメニュー画面のスクリーンショットにも古い座標を付けるため）。
3. マップはログの最新マップを使います。レイド中でもマップが分からなければ、設定の `Map` を使います。
   - `Map` の値は正規化されます: `laboratory` → `the-lab`、`labyrinth` → `the-labyrinth`、`factory4_night` → `night-factory`。
4. `Map` 役割の送信先と `BrowserRemoteID` に `playerPosition` を送ります。`NavigateMapOnShot` がオンなら、続けて `map` 命令も送ります。位置を先に送るのは、座標の高さから正しい階を選べるようにするためです。
5. `ground-zero-21` は tarkov.dev では `ground-zero` として送ります。

位置の送信では `browser:map` を送りません。内蔵ブラウザのマップタブは Remote Control 接続で追従します。

### 固定マップタブ

- タブ一覧の先頭には、閉じる・ピン留め・移動のできないマップタブ（`id: map`）が固定されています。
- `browser:map` を受け取ると、このタブを `https://tarkov.dev/map/<name>` に変えてアクティブにします。`ground-zero-21` は `ground-zero` として開きます。
- マップ名は `^[a-z0-9-]{1,60}$` に一致するものだけを受け付けます。
- 初期ページは `https://tarkov.dev/maps/` です。
- 2 番目の固定タブは TarkovTracker（`https://tarkovtracker.org/`）です。
- 固定タブのアドレス欄は読み取り専用で、URL を入力できません。使えるのは戻る・進む・再読み込み・ホーム・外部ブラウザで開く、だけです。

## 走り抜けタイマー（`RunThroughSeconds`）

- レイド開始時に `RunThroughAt` = 開始時刻 + `RunThroughSeconds` を状態に入れます。
  - `RunThroughSeconds` の既定は 430 秒です。1〜3599 の範囲外なら 430 になります。
- 対象になるのは次のどちらかのときです。
  - PvE のとき。設定の `GameMode` が `pve` のときも、`auto` でログから PvE を検出したときも対象です（`effectiveCatalogMode`）。`auto` で PvE と分かったのがレイド開始より後（起動時に復元したレイドなど）でも、その時点でタイマーを加えます（`handleTrackerLogEvent`）。
  - ログの `gamestarting` から `gamestarted` までが 3 秒を超えたとき（`RunThroughEligible`）。
- 対象で、`SoundsEnabled` と `RunThroughSound`（既定オフ）がオンなら、レイド開始時刻 + `RunThroughSeconds` の時点に通知音を予約します（その時刻を過ぎていれば予約しません）。鳴らす直前にログでまだレイド中かを確かめます。
- レイド終了で予約を取り消し、`RaidStartedAt` / `RunThroughAt` を消します。
- レイド開始時には、ほかに開始音・持ち物の確認音も鳴らせます。TarkovTracker に失敗状態のタスクがあれば、再開始の案内音も鳴らせます。マッチ成立時にはマッチ成立音も鳴らせます。音の設定は [設定と連携](settings-and-integrations.md) を参照してください。

## TarkovTracker のタスク状態

- タスクタブ自体には TarkovTracker の進捗を表示しません。
- Host は同期したタスク状態（`trackerTasks`）を次の 2 つに使います。
  - アイテム欄の「必要なタスク」の完了表示。TarkovTracker が接続済みで、検出したモードがアイテムのカタログモードと一致するときだけです（[アイテム欄](item-panel.md)）。
  - レイド開始時の、失敗タスク数に応じた再開始の案内音。
- ブラウザのサイドバーには、Host 状態として現在のマップ・レイド中か・TarkovTracker の接続状態を表示します。
- 連携の詳細は [設定と連携](settings-and-integrations.md) を参照してください。
