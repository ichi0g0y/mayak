# アイテム欄

ブラウザシェルのアイテム欄（`aside.item-panel`。既定は右側。設定で左・下にも置けます）で、1 つのアイテムについて売値・価格の推移・必要なタスクとハイドアウトを表示します。ウインドウ全体の構成は [browser-shell.md](browser-shell.md) を参照してください。

主な実装箇所:

| 役割 | ファイル |
| --- | --- |
| 描画・操作（HTML生成、検索ポップアップ、チャート、リサイズ） | `frontend/src/browser/shell.js` |
| 状態管理・Go 呼び出し・保存 | `frontend/src/browser/api.js` |
| 検証・価格計算・経過時間・チャート計算・アイテムページURL | `frontend/src/browser/item.js` |
| Go 側の入口（`showBrowserItem`、`BrowserItemInfo`、`BrowserItemHistory`、`BrowserItemSearch`、`withTaskURLs`） | `app_item.go` |
| 価格・必要数の組み立て | `internal/iteminfo/info.go`, `live.go`, `history.go` |
| 検索用のアイテム一覧 | `internal/itemapi/client.go`, `internal/itemmatch/match.go` |
| 他言語名 | `internal/locale/locale.go` |
| タスクページURL | `app_quest_site.go` |

## 開く・閉じる

### 開くきっかけ

- **スクリーンショットで認識したアイテム**: OCR 結果の一致度が `0.82` 以上なら `app_recognition.go` が `showBrowserItem(mode, id)` を起動します（認識の詳細は [recognition.md](recognition.md)）。
  - まずカタログのデータで `browser:item` イベントを送り、続いてライブ価格を取得して再送します（タイムアウト 45 秒）。
  - ライブ取得の間に別のアイテムが認識された場合（`status.ItemID` が変わった場合）、2 回目は送りません。
  - シェルは `browser:item` を受けると `itemInfo()` で検証し、欄を開いて (`itemOpen=true`) 価格履歴を読み込み、状態を保存します。ローカル接続時は WebRTC の相手にも転送します。
- **検索**: 検索結果を選ぶと `itemSelect` → `BrowserItemInfo('', id)` で開きます（後述）。
- **再起動後の復元**: 保存された `itemPanel.open` をそのまま使い、`itemPanel.id` があれば `BrowserItemInfo(mode, id)` で最新価格を取り直して表示します。取得が終わるまでは保存値（`restoredItem`）を保持し、上書き保存で消えないようにしています。

### トグル・閉じる

- ドック（縦レイアウトでは下部、横レイアウトでは上部右端）の `package` アイコンのボタン（`itemToggle()`）で開閉します。欄上部の `panelRight` ボタンでも閉じられます（`itemClose`）。
- アイテムがない状態で開くと、検索ポップアップが自動で開きます。本文には「上のルーペからアイテム名で検索できます…」というヒントを表示します。
- 開いている間は、ページビューの横（下に置いたときは下）に欄の幅（高さ）だけの余白を確保します（`api.js` の `bounds()`）。

### 幅の変更

- 欄のページ側の端（`.item-resizer`）をドラッグして幅を変えます。ページ側へドラッグすると広がります。下に置いたときは上端で高さを変えます（`itemPanelHeights` = 180〜560、既定 280、`clampItemPanelHeight`）。
- 幅は `itemPanelWidths` = 最小 `260`・最大 `560`・既定 `320`（論理ピクセル）の範囲に収まります（`clampItemPanel`）。
- ドラッグ中は `requestAnimationFrame` ごとに `itemPanelWidth` を反映し、離したときに `preferences` として保存します。ダブルクリックで既定の `320` に戻ります。

### 保存される状態（`browser.json`）

| キー | 内容 |
| --- | --- |
| `itemPanelWidth` | 欄の幅。読み込み時に `clampItemPanel` で補正 |
| `itemPanel.open` | 開いていたか（`true` のときのみ開く） |
| `itemPanel.id` | 表示中のアイテムID。24桁の16進数でなければ空 |
| `itemPanel.mode` | `regular` / `pve` / `pvp-season`。ID が無効なら空 |

既定値は `{open:false,id:'',mode:''}`（`state.js` の `defaults()` と `restoreItemPanel()`）。

## ヘッダー

- アイコンと名前、その下に「略称 · 幅×高さ · ゲームモード」を表示します。
  - ゲームモード表記: `regular`→`PvP`、`pve`→`PvE`、`pvp-season`→`Season`。
  - アイコンがなければ `package` アイコンで代用します。
- アイコンと名前はボタンになっており、アイテムのページをポップアップウインドウで開きます（`itemPage`、同じページをもう一度押すと閉じる）。ポップアップの位置と動作は [browser-shell.md](browser-shell.md) を参照。
- **言語別の名前**: `names` / `shortNames` はシェルの表示言語（例: `ja`）の名前です。その言語の名前がなければ英語名を使います（`localName`）。`h2` の `title` には常に英語名を入れます。
  - 名前は `internal/locale` の `Languages`（現在は `ja` のみ）ごとに、カタログの `items_ja`・`tasks_ja` などのリソースから取ります。リソースごとに分けて持ち、アイテム名は `items_<lang>`、タスク名は `tasks_<lang>` だけから引きます（キーが衝突しないようにするため）。英語名と同じものは含めません（`index.names`）。詳細は [languages.md](languages.md)。
  - `itemInfo()` は言語コードが `^[a-z]{2}(-[a-z]{2})?$` に合うものだけを最大 20 件受け付けます。

## 価格の鮮度表示と更新ボタン

- 見出しの下に「`LIVE` または `カタログ` · 価格更新 〇〇前」を表示します。`item.live` が真なら `live` クラスが付きます。
- 経過時間は `pricedAt` から計算します（`item.js` の `age()`）。

| 経過 | 日本語 | 英語 |
| --- | --- | --- |
| 1分未満 | たった今 | just now |
| 60分未満 | 〇分前 | 〇 min ago |
| 48時間未満 | 〇時間前 | 〇 h ago |
| 60日未満 | 〇日前 | 〇 days ago |
| それ以上 | 〇か月前（30日=1か月） | 〇 months ago |

- 表示は 30 秒ごとに再描画なしで更新されます。欄が開いていてページが表示中なら、180 秒ごとに自動で `itemRefresh`（`auto: true`）を実行します。自動更新で取得に失敗したときは、エラーを出さずに表示中のアイテムをそのまま残します。
- 行末の小さな更新ボタン（`itemRefresh`）は `BrowserItemInfo(mode, id)` を呼び、同じアイテムならその結果に置き換えて、価格履歴も取り直します。ボタンで押したときの失敗はエラーとして表示します。
  - 実行中はアイコンが回転し（`spin`）、`aria-busy` が立ちます。キャッシュからすぐ返っても回転が見えるよう、最低 600ms は表示します。実行中の再押下は無視します。

## 価格の出どころ（`internal/iteminfo`）

### カタログ（`Catalog`）

- カタログのスナップショット（[catalog.md](catalog.md)）から、`items`・`items_en`・`traders`・`traders_en`・`tasks`・`tasks_en`・`hideout`・`hideout_en` を読んで索引を作ります。索引はモードごとに持ち、スナップショットが変わったときだけ作り直します。
- `pricedAt` は品目の `updated`、なければ `lastScan` です。`fetchedAt` は取得時刻です。
- `lastLowPrice` か `avg24hPrice` があればフリーマーケット情報（`flea`）を作ります。ない場合は「フリーマーケットでは売れません」と表示します。
- モードが不正または `auto` なら `regular` を使います（`Mode`）。

### ライブ価格（`Live`、`live.go`）

- `https://api.tarkov.dev/graphql` に `item(id, gameMode)` を問い合わせ、`lastLowPrice`・`avg24hPrice`・`low24hPrice`・`high24hPrice`・`changeLast48hPercent`・`lastOfferCount`・`updated`・`sellFor` を取ります。
- 対応するのは `regular` と `pve` だけで、`pvp-season` はエラーになります。
- HTTP タイムアウトは 10 秒、同じモード・アイテムは 1 分間（`liveTTL`）キャッシュします。
- `sellFor` のうち `flea-market` は除き、残りをトレーダー買取とします。フリーマーケットの利用可能レベル（`minLevel`）はカタログの値を引き継ぎます。トレーダー価格が空ならカタログの値を残します。
- 成功すると `live=true`、`pricedAt` は `updated` になります。

### 取得順とフォールバック（`Current`）

1. カタログから組み立てる。
2. ライブ価格を試し、成功すればそれを返す。
3. 失敗した場合、スナップショットが 5 分（`staleAfter`）より古ければカタログを強制更新（`Refresh(..., true)`）し、組み立て直して返す。ライブ失敗の理由はエラーとして返す（`showBrowserItem` はデバッグログに出すだけでカタログ表示を続ける）。

### 一番高く売れる先

- `bestSale()`: フリーマーケットの最安値（`flea.lastLow`）が最高値のトレーダーの `priceRub` より高ければフリーマーケット、それ以外は最高値のトレーダーです。
- 金額と、1マスあたりの金額（`priceRub / (width × height)`）を表示します。

### フリーマーケット

最安値、24時間平均、24時間の幅（安値と高値の両方があるときのみ）、48時間の変動（プラスは `up`、マイナスは `down`）、出品数、利用可能レベル（0 でないときのみ `Lv.N`）を表示します。

### トレーダー買取

- ルーブル換算（`priceRub`）の高い順に並べ、先頭行に `best` クラスを付けます。
- ルーブル以外は「元の通貨の金額（ルーブル換算）」と表示します。通貨記号は `₽` / `$` / `€`、桁区切りは `ja-JP` または `en-US` です。
- `itemInfo()` はトレーダーを最大 12 件まで受け付けます。

## 価格の推移チャート

- 取得元: `https://json.tarkov.dev/{mode}/prices/{id}`（`history.go`）。ID は24桁の16進数に限ります。
  - 30 分（`historyTTL`）キャッシュし、取得に失敗したとき（接続できない、200 以外、途中で切れた・読めない応答）はキャッシュがあればそれを返します。応答は最大 8MB まで読みます。
  - 各点は `t`（Unixミリ秒）・`price`（平均）・`min`（最安）で、古い順です。シェルは最新 5000 点まで使います（`historyPoints`）。
- アイテムごとに 1 回読み込み、更新ボタン（180 秒ごとの自動更新も含む）では読み直します（`loadHistory`）。読み込み中・失敗・期間内にデータなしで、それぞれメッセージを表示します。
- フリーマーケットで売れないアイテムではチャートを出しません。
- **期間**: `7d`（7日）・`30d`（30日）・`all`（全期間）。選んだ期間は `localStorage` の `mayak.chartRange` に保存します（既定 `7d`）。
- **描画**（`chartSeries` / `chartPath` / `itemChart`）:
  - SVG は `288×132`、上下に `14` の余白、グリッド線は 4 本です。
  - 縦軸の範囲は、全値の 2% と 98% の分位点に 8% の余白を足したものです（外れ値の影響を抑えるため）。
  - 最安値の線は、現在の `flea.lastLow` を `pricedAt` 時点の点として末尾に足し、今まで伸ばします（履歴はライブ価格より数時間遅れるため）。
  - 値のない点で線は途切れます。
  - グリッド線の価格ラベルは、引き伸ばされる SVG ではなく左側の余白に HTML で置きます（`1k`、`1.2M`、1000万以上は `12M` の形式）。
  - 上に期間内の最高値・最安値・変動率を、下に開始日と終了日（全期間は年月）と凡例を表示します。
  - ホバーすると、最も近い点の日時と平均・最安値を再描画なしで表示します。

## 必要なタスクとハイドアウト

### 必要数の求め方（`build`）

- タスク: 目標の種類が `giveItem`・`findItem`・`plantItem`・`sellItem` のものだけを数えます。
  - 同じタスク・同じアイテムの目標は 1 行にまとめます。個数は最大値、FIR はいずれかが要求していれば付けます。
  - タスク名の昇順に並べます。
- ハイドアウト: 各施設レベルの `itemRequirements` から、施設名・レベル・個数・FIR を取ります。施設名、レベルの順に並べます。

### TarkovTracker の完了状態（`itemProgress` / `Progress`）

- TarkovTracker が `connected` で、そのゲームモードがアイテムのモードと一致するときだけ進捗を付けます（連携は [settings-and-integrations.md](settings-and-integrations.md)）。
  - タスクの `state` は `completed` / `failed` / `uncompleted`。記録がなければ `uncompleted` です。
  - ハイドアウトの `complete` は `true` / `false`。
- 条件を満たさないときは状態を空（ハイドアウトは `null`）にし、「TarkovTrackerと同期すると完了済みがわかります」と表示します。

### 表示

- 各行: 名前（タスクは言語別の名前とトレーダー名、ハイドアウトは施設名と `Lv.N`）、`FIR` バッジ、`×個数`、完了ならチェック。
- 完了済み（タスクは `completed`、ハイドアウトは `complete===true`）は末尾に並べ、`done` クラスを付けます。見出しの数字は未完了の件数です。
- `itemInfo()` はタスク・ハイドアウトとも最大 80 件まで受け付けます。
- **タスクを押す**と、そのタスクのページをポップアップウインドウで開きます（`itemTask`、ウインドウは [browser-shell.md](browser-shell.md)、タスク表示は [tasks-and-maps.md](tasks-and-maps.md)）。サイトは押した時点の設定で決まります（`itemSite()`）。
  - ローカル接続（Host 上）: Host の設定 `questSite`。
  - それ以外: ブラウザ設定の `questSite`。`host`（Hostの設定に従う）のときは、アイテムと一緒に届いた `questSite`（なければ `tarkov-dev`）。

## アイテム検索

- 欄上部のルーペボタンで、アイテムの上に重なる検索ポップアップを開きます（アイテム表示は動きません）。ポップアップの外を押すか `Escape` で閉じ、検索語をクリアします。
- 入力から 150ms 待って `itemSearch` を実行します。入力は最大 80 文字、後から打った検索の結果が優先されます。
- Go 側 `BrowserItemSearch` は、検出中のゲームモードのアイテム一覧（`itemapi.ItemsForMode`、12 時間キャッシュ）から最大 20 件を返します。
  - 照合する名前: 英語名、略称、他言語名（`Aliases`）、他言語略称（`ShortAliases`）。大文字小文字は区別せず、いちばん良い順位を採用します。日本語名でも英語名と同じ扱いで検索できます。
- **順位**（`searchRank`）:

| 順位 | 条件 |
| --- | --- |
| 0 | 完全一致 |
| 1 | 名前の先頭と一致 |
| 2 | 単語の先頭と一致（空白の後） |
| 3 | 単語の途中に含む。4 文字以上の検索語、または漢字・ひらがな・カタカナを含む 2 文字以上の検索語のみ |

  同順位では英語名の短い順、次に名前順です。
- 結果にはアイコン、表示言語の名前、略称を表示します。該当なしなら「見つかりませんでした」。
- 結果を押すか、`Enter` で先頭の結果を選ぶと `BrowserItemInfo('', id)`（モードは検出中のもの）で読み込み、欄を開いて保存します。

## ページURL

### タスクページ（`withTaskURLs`、`app_item.go`）

アイテムを送る直前に、各タスクの 3 サイト分の URL を `urls` に付け、Host の設定サイトを `questSite` に入れます。URL は認識したタスクと同じ `questStatusURL` で作ります（`app_quest_site.go`）。

| サイト | URL |
| --- | --- |
| `tarkov-dev` | `https://tarkov.dev/task/{normalizedName}`（なければ `https://tarkov.dev/tasks/`） |
| `official-wiki` | タスクの `wikiLink`（`https://escapefromtarkov.fandom.com/wiki/...` の場合）。なければ `https://escapefromtarkov.fandom.com/wiki/{名前の空白を_に}` |
| `japanese-wiki` | `https://wikiwiki.jp/eft/{トレーダー}/{名前}`。名前からカンマと末尾の ` [PVE ZONE]` を除く。トレーダー不明なら `https://wikiwiki.jp/eft/?cmd=search&word={名前}` |

### アイテムページ（`itemPageURL`、`item.js`）

| サイト | URL |
| --- | --- |
| `official-wiki` | `wikiLink`（あれば） |
| `japanese-wiki` | `https://wikiwiki.jp/eft/{英語名}` |
| それ以外・該当なし | `link`（tarkov.dev）、なければ `wikiLink` |

ヘッダーのサイト選択は、Host 上のローカル接続なら Host の設定、それ以外はブラウザ設定（`host` ならアイテムの `questSite`）です。ボタンの `title` は「アイテムのページを開く · サイト名」です。
