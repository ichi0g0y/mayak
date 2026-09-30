# 設定と連携

MAYAK の設定は保存場所の異なる 2 系統に分かれています。

- **Host 設定**: Go 側の `internal/config.Settings`。`settings.json` に保存され、React の設定ページ（`frontend/src/main.tsx`）で編集します。このページはブラウザシェルの設定タブ内に `<iframe id="host-settings">`（`/settings.html#<section>`）として埋め込まれます。
- **ブラウザ設定**: シェル側の状態（`frontend/src/browser/state.js`）。`browser.json` に保存されます。

どちらも、別の PC でも同じであってほしい好みと、この PC に属するものを別のファイルに分けて保存します（[ユーザーデータの保存](user-data.md)）。

画面上では両者を区別せず、1 つの設定タブのサイドバーにまとめて表示します（シェルの詳細は [browser-shell.md](browser-shell.md) を参照）。

## 設定画面の構成

設定タブのサイドバーは `shell.js` の `settingsGroups` の順に並びます。Host のセクション（`hostSections`）は、Windows 上で動いているときだけ表示されます（接続方法は問いません）。

| グループ | セクション キー | 表示名 (ja) | 提供元 |
|---|---|---|---|
| 一般 | `appearance` | 表示 | ブラウザ |
| | `startup` | 起動とウィンドウ | Host |
| | `sounds` | 通知 | Host |
| ゲーム | `folders` | フォルダと保存 | Host |
| | `recognition` | ゲームと認識 | Host |
| | `tasks` | タスクの開き方 | ブラウザ |
| 連携 | `remote` | tarkov.dev 連携 | Host |
| | `tracker` | TarkovTracker | Host |
| | `connection` | 他のPCとの接続 | ブラウザ |
| ブラウザ | `adblock` | 広告ブロック | ブラウザ |
| 状態と診断 | `status` | ステータス | Host |
| | `logs` | ログ | Host |
| | `debug` | デバッグ | Host |

- Host 側のページは URL ハッシュでセクションを切り替えます。ハッシュが変わると、保留中の保存を待ってから `GetSettings()` で設定を読み直します。
- シェルの表示言語（`<html lang>`）が変わると、Host 設定の `language` にも反映されます。テーマも同一オリジンの iframe に適用されます。

## Host 設定 (`internal/config.Settings`)

### 保存の仕組み

- 保存先: `%APPDATA%\Mayak\settings.json`（`os.UserConfigDir()` 配下）
- 書き込みは一時ファイル経由のアトミック置換（パーミッション `0600`）です。書き込み前に、有効な JSON であれば直前の内容を `settings.json.bak` に退避します。
- 読み込み時に JSON が壊れていれば `.bak` から復元し、それも失敗した場合は既定値を使います。
- 設定ページは項目を変更するたびに `PersistSettings` を呼んで即時保存します。実行中のサービスは再起動しません。どちらも同じ `saveSettings` を通り、変更された項目（自動起動、TarkovTracker、ゲームモード、言語、マーカー、スクショ整理）だけを反映します。明示的な保存（エラー時の「再試行」）は `SaveSettings` を使い、監視中にフォルダが変わった場合はウォッチャーも張り替えます。
- どちらの保存でも `normalizeSettings` が値を補正します。また `keepWindowSettings` により、`windowX/Y/Width/Height/Configured`、`browserRemoteId`、`ocrDefaultRevision` はフロントエンドから上書きできません。
- `launchAtStartup` の変更は保存より先にレジストリへ適用され、保存に失敗すると元に戻します。

### マイグレーション

| 対象 | 内容 |
|---|---|
| `minimizeToTray` → `closeToTray` | `closeToTray` キーが無い古いファイルでは `closeToTray = minimizeToTray` とする（以前は `minimizeToTray` が「閉じる」も兼ねていたため） |
| `remoteId` → `remoteTargets` | `remoteTargets` が空で `remoteId` があれば、`{id, name:"Remote 1", map:true, tasks:true}` の 1 件に変換する。保存後の `remoteId` は常に先頭ターゲットの ID |
| `ocrDefaultRevision` | `< 1` のとき、`ocrEngine:"windows"` を Tesseract が使えれば `tesseract` に一度だけ移行し、`1` を記録する |
| OCR フォールバック | `ocrEngine` が `tesseract` で `tesseractPath` が空なのに、同梱版も PATH 上の `tesseract` も無い場合は `windows` にする |
| `browserRemoteId` | 12 文字 `[A-Z0-9]` でなければ起動時に生成する（以前の 4 文字の ID も作り直す） |
| `map` | `laboratory`→`the-lab`、`labyrinth`→`the-labyrinth`、`factory4_night`→`night-factory` |
| フォルダ | `screenshotDirectory` / `logsDirectory` が空なら、起動時に `eftdetect` で自動検出した値を入れる |
| ウィンドウ位置 | `window.json` が未設定で旧 `windowConfigured` が真なら、旧 `windowX/Y/Width/Height` を初期位置に使う |

起動時の補正で内容が変わった場合は、その場で保存し直します（`settings.json` の読み込みに失敗した場合は保存しません）。`ocrDefaultRevision`、OCR フォールバック、フォルダの補正は Host として起動したときだけ行います。

### 全項目と既定値

既定値は `config.defaults()` の値です。記載の無い項目はゼロ値（`""` / `false` / `0` / 空配列）です。

| キー | 既定値 | 意味 / 補正 |
|---|---|---|
| `language` | `"ja"` | Host UI とトレイメニューの言語。`ja` / `en` 以外は `ja` |
| `gameLanguage` | `""`（→ `"auto"`） | OCR に使うゲーム言語。`auto`、`en`、または `internal/locale` の言語のみ |
| `questSite` | `""`（→ `"tarkov-dev"`） | タスクを開くサイト。`tarkov-dev` / `official-wiki` / `japanese-wiki` |
| `screenshotDirectory` | `""` | EFT の Screenshots フォルダ（`filepath.Clean` 済み） |
| `logsDirectory` | `""` | EFT の Logs フォルダ |
| `remoteId` | `""` | 旧形式の Remote Control ID。互換用 |
| `remoteTargets` | `[]` | tarkov.dev Remote Control の送信先。`{id, name, map, tasks}`。空 ID と重複 ID は除去し、名前が空なら `Remote N` |
| `browserRemoteId` | 自動生成 | 内蔵ブラウザで開いた tarkov.dev のマップページ用 Remote Control ID（12 文字） |
| `map` | `""` | マップ未検出時に使うマップ。空なら自動 |
| `gameMode` | `"auto"` | `auto` / `regular` / `pve` / `pvp-season`。変更するとカタログを再取得 |
| `ocrEngine` | `"tesseract"` | `tesseract` / `windows`。それ以外は `tesseract` |
| `tesseractPath` | `""` | Tesseract 実行ファイルのパス（空なら同梱版または PATH） |
| `ocrDefaultRevision` | `0` | OCR 既定値移行の記録（内部用） |
| `debug` | `false` | デバッグ表示（crop プレビュー、候補、Hideout 診断） |
| `saveRecognitionDebug` | `false` | 認識データを `Screenshots\Mayak-Debug` に保存する |
| `screenshotCleanup` | `false` | 古いスクリーンショットを自動で削除する |
| `screenshotRetainCount` | `500` | 残す最大枚数。範囲は 0–100000 で、0 はこの条件を使わない |
| `screenshotRetainHours` | `168` | 保持時間（時間）。範囲は 0–87600 で、0 はこの条件を使わない |
| `soundsEnabled` | `false` | 通知音のマスタースイッチ（新しく入れたときはオフ） |
| `questSoundEnabled` / `questSoundPath` | `true` / `""` | タスクを認識したとき |
| `taskNotMatchedSoundEnabled` / `taskNotMatchedSoundPath` | `true` / `""` | タスク画面の文字は読めたが、該当するタスクが無いとき |
| `itemSoundEnabled` / `itemSoundPath` | `false` / `""` | アイテムを認識したとき |
| `itemNotMatchedSoundEnabled` / `itemNotMatchedSoundPath` | `false` / `""` | アイテムの文字は読めたが、該当するアイテムが無いとき |
| `errorSoundEnabled` / `errorSoundPath` | `true` / `""` | スクリーンショットを解析できなかったとき（OCR・データ取得の失敗） |
| `remoteErrorSoundEnabled` / `remoteErrorSoundPath` | `true` / `""` | tarkov.dev への送信に失敗したとき（ID に接続できないだけのときは鳴らない） |
| `matchFoundSoundEnabled` / `matchFoundSoundPath` | `true` / `""` | マッチ成立時 |
| `raidStartSoundEnabled` / `raidStartSoundPath` | `true` / `""` | レイド開始時 |
| `runThroughSoundEnabled` / `runThroughSoundPath` | `true` / `""` | ランスルー時間が過ぎたとき |
| `runThroughSeconds` | `430` | ランスルー時間（秒）。1–3599 の範囲外なら 430 |
| `gameExitSoundEnabled` / `gameExitSoundPath` | `false` / `""` | タルコフを閉じたとき（プロセスの終了。クラッシュも同じ） |
| `gameStartSoundEnabled` / `gameStartSoundPath` | `false` / `""` | ゲーム起動時（メニューに着いたとき） |
| `questItemsSoundEnabled` / `questItemsSoundPath` | `false` / `""` | レイドから戻ったとき（名前は以前のタスクアイテム確認の名残） |
| `taskFailedSoundEnabled` / `taskFailedSoundPath` | `false` / `""` | ゲームでタスクが失敗したとき（TarkovTracker が無くても。まとめて失敗しても 10 秒に 1 回） |
| `soundVolume` | `28` | 全体の音量（0–100 に丸める） |
| `soundVoice` | `""` | 基本の声。組み込みの声のパック名、`beep`（ビープ音）、空なら言語ごとの標準（日本語は `tsumugi`、ほかは `heart`） |
| `soundVoices` | `{}` | 通知ごとの声（通知の種類 → パック名 / `beep` / `custom`）。無い通知は基本の声 |
| `soundVolumeOffsets` | `{}` | 通知ごとの、全体の音量からの調整（−50〜＋50、0 は持たない） |
$1| `autoStartMonitoring` | `true` | 起動時に監視を開始する |
| `openMapOnRaidStart` | `true` | レイド開始時に tarkov.dev を現在のマップに切り替える |
| `navigateMapOnPositionScreenshot` | `true` | 位置スクリーンショットの送信後、そのマップに切り替える |
| `tarkovTrackerEnabled` | `false` | TarkovTracker との同期 |
| `startMinimized` | `false` | 最小化した状態で起動する（「起動とウィンドウ」）。ウインドウの位置を復元したあとで最小化し、`minimizeToTray` が真ならトレイに入る |
| `minimizeToTray` | `false` | 最小化したときにウィンドウを隠し、タスクバーから消す |
| `closeToTray` | `false` | 閉じるボタンでは終了せず、トレイに常駐する |
| `keepPriority` | `false` | 起動 3 秒後と以後 10 秒ごとに、MAYAK 自身と直下の `msedgewebview2.exe` が Idle / BelowNormal なら Normal に戻す（`guardPriority`、Windows のみ）。Process Lasso の ProBalance などが起動直後の CPU 使用で優先度を下げ、そのあいだに生まれた WebView2 プロセスが低いまま残って UI が固まって見えるときに使う |
| `launchAtStartup` | `false` | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` に登録する |
| `updateChannel` | `stable` | 更新の取得先。`nightly` にすると nightly ビルドも対象にし、安定版と新しい方を適用する（[自動アップデート](#自動アップデート)） |
| `autoUpdate` | `true` | GitHub Releases の新しい版を自動で確認・ダウンロードし、終了時に適用する（[自動アップデート](#自動アップデート)） |
| `windowX` / `windowY` / `windowWidth` / `windowHeight` / `windowConfigured` | `0` / `false` | 旧形式のウィンドウ位置。現在は `window.json` を使う |

効果音のパスはトリムと `filepath.Clean` を通します。空のときは組み込み音を使い、ファイルの再生に失敗した場合も組み込み音にフォールバックします。

### 設定ページのセクション別の項目

| セクション | 項目 | 操作 |
|---|---|---|
| `folders` | `screenshotDirectory`, `logsDirectory`, `screenshotCleanup`（オンのとき `screenshotRetainCount` と `screenshotRetainHours` を表示） | 自動検出（`AutoDetectEFTDirectories`）、フォルダ選択 |
| `recognition` | `gameMode`, `gameLanguage`（選択肢は「自動」と Go の `GameLanguages()` が返す言語。表示名は `main.tsx` の `languageNames`）, `ocrEngine`, `tesseractPath`（`tesseract` のときのみ表示） | — |
| `remote` | `remoteTargets`（名前、ID（伏せ字。目のボタンで表示）、役割「マップ」「タスク」）, `map`, `openMapOnRaidStart`, `navigateMapOnPositionScreenshot` | 追加、ブラウザからの ID 自動検出（`AutoDetectRemoteID`）、接続テスト（`TestRemote`） |
| `tracker` | `tarkovTrackerEnabled`、トークンの取り込み、保存済みキー、既知プロフィールへのキーの割り当て、過去ログの同期 | ログからプロフィールを探す、tarkovtracker.org の API 設定を開く |
| `sounds` | `soundsEnabled`。オンのとき、各通知（Hideout エラー、クエスト認識成功、認識・接続エラー、マッチ成立、レイド開始、ランスルー終了、タスクアイテム確認、失敗タスクの再開確認）の ON/OFF・音声ファイル・リセット・試聴、ランスルー時間（分・秒。ランスルー終了の通知がオンのときだけ表示）、`soundVolume` | `ChooseSoundFile`, `PreviewSound` |
| `startup` | `launchAtStartup`, `startMinimized`, `autoStartMonitoring`, `minimizeToTray`, `closeToTray`, `autoUpdate` | — |
| `status` | 表示のみ: Remote 接続状態、現在のマップ、レイド状態、スクリーンショット種別、TarkovTracker の状態、最後の検出結果、カタログの状態、アップデートの状態 | 最新スクリーンショットの解析、TarkovTracker の更新、カタログの更新、タスクページを開く、更新の確認・ダウンロード・再起動して更新、リリースノートを開く |
| `logs` | 表示のみ（[ログ](#ログ) を参照） | フォルダを開く、ログの消去 |
| `debug` | `debug`, `saveRecognitionDebug`。`debug` がオンのときは Hideout 診断フォルダ、候補一覧、crop、目標も表示 | `OpenHideoutDiagnostics` |

## ブラウザ設定（シェル側）

`state.js` の `defaults()` と `restore()` で決まります。`persist()` が保存するのは下表の項目だけで、接続コードや UI のエラーは保存しません。

| キー | 既定値 | 取り得る値 / 意味 |
|---|---|---|
| `language` | `"ja"` | `ja` / `en`。シェルの表示言語（Host 設定にも反映） |
| `theme` | `"mayak-dark"` | `system`（OS の明暗に合わせて `mayak-light` / `mayak-dark`）, `mayak-dark`, `mayak-light`, `catppuccin-latte`, `catppuccin-frappe`, `catppuccin-macchiato`, `catppuccin-mocha`, `nord`, `dracula`, `gruvbox-dark`, `tokyo-night`, `solarized-dark`, `solarized-light` |
| `layout` | `"vertical"` | `vertical`（左サイドバー）/ `horizontal`（上部に横並び） |
| `sidebarCollapsed` | `false` | サイドバーを折りたたむ |
| `sidebarWidth` | `224` | 180–420 に収める |
| `bookmarksCollapsed` | `false` | サイドバーのブックマーク欄を折りたたむ |
| `bookmarkView` | `"grid"` | `grid` / `list` |
| `itemPanelWidth` / `itemPanel` | `320` / `{open:false,id:"",mode:""}` | アイテム欄の幅と、再起動後に復元する状態（[item-panel.md](item-panel.md)） |
| `adblock` | `true` | 広告ブロック。切り替えると表示中のページを再読み込みする |
| `taskMode` | `"new"` | 検出したタスクの開き方。`new`（新しいタブを追加）/ `reuse`（固定していないタスクタブを更新） |
| `questSite` | `"host"` | `host`（Host の設定に従う）/ `tarkov-dev` / `official-wiki` / `japanese-wiki` |
| `connection` | `{mode:"local", link:null, receive:{task:true, map:true, item:true}}` | 接続方法、ペアリングの鍵と役割、Client で出すもの（[Host / Client モード](#host--client-モード)） |
| `mapHidden` / `mapSettings` / `mapCollapsed` / `squadName` / `squadColor` / `squadCollapsed` / `squadCode` / `squadRecent` | `[]` / 下記 / `[]` / `""` / `""` / `false` / `""` / `[]` | マップと分隊（[browser-shell.md](browser-shell.md#マップ)、[ユーザーデータの保存](user-data.md)）。`mapSettings` の既定は `{snipers:true, extracts:false, activeTasks:false, subtleLabels:false, extractText:100, labelText:100, fade:20, style:"svg", mode:"auto"}` |
| `bookmarks` / `bookmarkRevision` | 既定のブックマーク / `3` | 最大 100 件。revision 1 でブックマークを追加し、revision 2 で TarkovTracker を `.org` に移行、revision 3 で tarkov.dev と TarkovTracker をサイドバーにピン留めしたブックマークにする（以前の固定タブの代わり） |
| `favicons` | `{}` | ホスト名ごとのアイコン URL（最大 200 件） |
| `tabs` / `active` | マップ（`livemap`）、設定 / マップ | 以前の固定タブ `map`・`tracker` は復元時に捨てる。タブは最大 80 個 |

ウィンドウの位置とサイズは `window.json` に保存し、`browser.json` には持ちません（旧形式の `window` キーは読み込まず、次の保存で消えます）。

### セクション別の項目

| セクション | 内容 |
|---|---|
| `appearance` | 表示言語、テーマ、時刻表示、タブとアイテム情報の位置、「チュートリアルを表示」 |
| `tasks` | Host 上（`local`）では Host の `questSite` を直接編集する（`hostQuestSite` アクション。シェルの `questSite` は `host` に戻る）。Client では `questSite`（「Host の設定に従う」を含む）。どちらでも `taskMode` を設定する |
| `adblock` | 有効化のチェックボックス。EasyList、EasyPrivacy、AdGuard 日本語フィルタを使う。tarkov.dev は対象外 |
| `connection` | 接続方法（`local` は Windows のみ / `client`。前の版の `off` は `client` に読み替える）。Host では接続コードの発行（ペアリング済みなら「別の PC を追加」）とすべての PC との解除、Client では接続コードの入力（長いコードの貼り付けも可）、この PC に出すもの（`connection.receive`）、この PC の解除 |
| `about` | 名前とバージョン（`GetVersion`。開発ビルドでは「開発ビルド」）、公式サイト・ソースコード・変更履歴へのリンク、ライセンスとクレジット、アップデート（現在の版・最新の版・最終確認、「更新を確認」`CheckForUpdates`、「ダウンロード」`DownloadUpdate`、「再起動して適用」`InstallUpdate`）。タスクトレイの右クリックにも「更新を確認」がある |

- 以前、ブラウザ側で `questSite` を選んでいた場合は、Windows の Host で起動したときに一度だけ Host の `questSite` へ移し、ブラウザ側を `host` に戻します。
- タブの配置は、ツールバーのボタン（`toggleLayout`）でも切り替えられます。

## 監視

- **開始**: `autoStartMonitoring` が真で `screenshotDirectory` が設定済みなら、起動時に `StartMonitoring()` を呼びます。手動ではシェル上部のモニターボタンで開始・停止します。
- **条件**: Windows の Host モード（`local`）でのみ動作します。Client モードに切り替えると監視を止め、進行中の解析と Remote 接続も閉じます。
- **Screenshots フォルダ**: `watcher` で新しい画像を検知し、認識処理に回します（[recognition.md](recognition.md)）。処理済みの記録（`processed-screenshot.json`）が無い場合は、起動時に既存の最新ファイルを処理済みとして記録するため、再処理しません。
- **Logs フォルダ**: 次の 3 つの検出器を起動します。
  - `logdetect`: マップ、マッチ成立、レイド開始・終了、待ち時間、ランスルー時間
  - `trackerlog`: EFT のプロフィール検出、タスクの状態変化
  - `hideoutlog`: Hideout 操作の結果
- 監視中に Logs フォルダが変わると、検出器を張り替えます（`SaveSettings` のとき）。
- **スクリーンショットの自動削除**: 解析のたびに加えて、Host のバックグラウンドで 30 分ごとに実行します。対象は Screenshots 直下の画像だけで、`Mayak-Debug` は削除しません。

### シェル上の状態表示

サイドバーの上部（横並びレイアウトではタブ列の右端）にインジケーターを表示します。

| 表示 | 状態 |
|---|---|
| ゲームモード（`game-mode-badge`） | Host が検出しているゲームモード（PvP / PvE / Season）を太字で出す（枠なし、PvE と Season は色付き）。分からないあいだは出さない。クリックで 設定 → ゲームと認識 を開く（`settingsAt`） |
| Host アイコン（`mode-status`） | `host`（`local`・Host モード）/ `linked`（Client で Host とつながっている）/ `unlinked`（Client で未接続）/ `off`（受信 OFF）。ツールチップには「Hostモード」や「Clientモード · 接続中」などを出し、Host で接続中の PC があれば台数も出す。クリックで 設定 → 他のPCとの接続 を開く（`settingsAt`） |
| 監視ドット（`monitor-toggle`） | Windows の Host のときだけ表示する。`on` クラスで監視中を示す。ツールチップは「監視中/監視停止中 · マップ · レイド中/外 · TarkovTracker: 状態 · クリックで開始/停止」。クリックで `StartMonitoring` / `StopMonitoring` を呼ぶ。監視停止中に押したとき Screenshots フォルダが未設定（`GetSettings` の `screenshotDirectory` が空）なら、監視を始める代わりに 設定 → フォルダ を開く（自動判別に失敗したまま「何も認識されない」状態から、選び直す場所へ直接行けるように） |
| TarkovTracker（`tracker-status`） | Windows の Host で TarkovTracker のキーが 1 つ以上あるときだけ、監視ドットの左に出す（無ければ出さない）。`status.tracker.connection` に合わせて緑（`connected`・同期中）、黄（`connecting` / `waiting-profile`）、赤（`missing-token` / `error`）、灰（`disabled`）。ツールチップに状態と最後のエラー。クリックで 設定 → TarkovTracker を開く（`settingsAt`） |

状態は Host が送る `status:update` イベント（`monitoring`、`currentMap`、`raidActive`、`tracker.connection`）から更新します。

## 通知

### トースト（設定ページ右下）

| 種類 | 表示条件 | 消え方 |
|---|---|---|
| 保存成功 | 自動保存の完了 | 2 秒で消える |
| 保存失敗 | `PersistSettings` / `SaveSettings` のエラー | 次の保存が成功するまで残る（× は無い）。「再試行」ボタン付き |
| 通知（成功） | 各操作の成功メッセージ | 3 秒で消える |
| 通知（エラー） | 各操作の失敗 | × で閉じるまで残る |

### 通知音

- すべての音は `soundsEnabled` と各項目のスイッチの両方が真のときだけ鳴ります。
- 鳴らすものは、通知ごとに「カスタム（ファイル）→ その通知の声 → 基本の声 → ビープ音」の順に決まります（`internal/app/app_sound.go`）。ファイルはデータフォルダの `sounds/` に取り込んだものです（[ユーザーデータの保存](user-data.md)）。
- 音量は `soundVolume` に通知ごとの `soundVolumeOffsets` を足して 0–100 に収めたものです。
- 遅らせる設定（`soundDelays`）のある通知は、その秒数だけ待ってから鳴らす順番に入ります（ゲームの音と重ならないように。待っているあいだもほかの通知は鳴ります）。試聴はすぐ鳴らします。
- 同時に起きた通知は重ねずに順番に鳴らします。
- 認識の通知とエラーは、同じ内容で 3 秒以内なら繰り返し鳴らしません（`recognitionSoundCooldown`）。

| 通知 | 鳴るタイミング |
|---|---|
| タスク認識 | タスクを認識したとき |
| タスクを特定できない | タスク画面の文字は読めたが、該当するタスクが無いとき |
| アイテム認識 | アイテムを認識したとき |
| アイテムを特定できない | アイテムの文字は読めたが、該当するアイテムが無いとき |
| スクリーンショットの解析エラー | OCR やデータ取得に失敗したとき |
| マップ連携のエラー | tarkov.dev への送信に失敗したとき（ID に接続できないだけのときは鳴らない） |
| マッチ成立 | ログでマッチ成立を検出したとき |
| レイド開始 | ログでレイド開始を検出したとき |
| ランスルー終了 | PvE（設定または自動判定）、またはランスルー判定の対象となるレイドで、開始から `runThroughSeconds` が経過し、まだレイド中のとき |
| ゲーム起動時 | ゲームを起動してメニューに着いたとき（ログのプロフィール読み込みの 1 回目で、レイドを挟んでいないもの） |
| レイドから戻ったとき | レイドの後にメニューに着いたとき。生還か死亡かはログからは分からないので、どちらでも同じ通知 |
| 失敗タスクの再開確認 | メニューに戻ったときに、TarkovTracker 上で失敗しているタスクがあるとき |
| ゲーム終了時 | 5 秒ごとの確認で `EscapeFromTarkov.exe` が起動中から無くなったとき（MAYAK の起動時にすでに閉じていたら鳴らない） |

Hideout の操作が EFT のログでエラーになったことは通知しません（ゲームの画面には出ず、プレイヤーにできることも無いため）。記録はログページの Hideout で見られます（[hideout.md](hideout.md)）。

## トレイとウィンドウ

- **トレイ**（`tray.go`）: アイコンをクリックまたはダブルクリックすると `showWindow` でウィンドウを表示します。メニューは「MAYAKを開く / Open MAYAK」と「終了 / Quit」で、表示言語は `language` に従います。保存した `language` が変わると、その場でメニューの文言を切り替えます（`setTrayLanguage`、再起動は不要）。
- **最小化**: `minimizeToTray` が真なら、最小化したときにウィンドウを `Hide()` し、タスクバーから消します。ウィンドウのフックは起動処理（`ServiceStartup`）より先に走ることがあるため、`main.go` がウィンドウを作る前に保存済みの設定を入れておきます。起動処理は設定をロックを取って置き換えます。
- **閉じる**: `closeToTray` が真なら、閉じる操作をキャンセルしてウィンドウを隠します。終了はトレイメニューからのみ行えます（`quitting` フラグで区別）。
- **二重起動の防止**: `SingleInstance`（`UniqueID: com.ichi0g0y.mayak`）で制御します。2 つ目のプロセスを起動すると、既存のウィンドウを表示します。
- **ウィンドウ**: フレームレスです（macOS ではボタンが左上の赤・黄・緑）。既定サイズは 1120×760、最小サイズは 760×560 です。
- **位置の復元**（`window.json`）:
  - 移動、リサイズ、最大化、最大化解除、閉じる、終了のたびに保存します。最小化中の座標や、最小サイズ未満のサイズは保存しません。
  - ディスプレイ名・ID と作業領域の原点を記録し、ディスプレイ基準の相対座標で復元します。該当するディスプレイが無い場合は、重なりが最大のディスプレイ、なければプライマリディスプレイを使い、作業領域内に収めます。最大化状態も復元します。
  - 保存値が異常な場合（幅 5000 超など）は無視します。
- アイテム欄から開くポップアップウィンドウも、同じ方式で `popup.json` に位置を保存します。

## 自動アップデート

MAYAK は [GitHub Releases](https://github.com/ichi0g0y/mayak/releases) から自分自身を更新します（`internal/update`、`internal/app/app_update.go`）。Windows／macOS／Linux のどれでも同じ仕組みです。

- **版の比較**: ビルドに埋め込まれた版（`internal/version`、`task build` が `git describe` かリリースタグから `-ldflags -X` で入れる）と、GitHub の「latest」リリース（ドラフトとプレリリースは除く）のタグを semver で比べます。タグ直後のコミットを含む開発ビルド（`0.1.0-3-g1a2b3c4`）は `0.1.0` より新しい扱いなので、同じ版の通知は出ません。版が入っていない `dev` ビルドはどのリリースよりも古い扱いです。
- **更新チャンネル**（`updateChannel`、設定 → MAYAK について の「アップデート」。シェルの `aboutUpdate` がこの PC の設定を読み書きする。OS と接続モードによらず選べる）: `stable`（既定）は上の「latest」リリースだけ。`nightly` は加えて nightly ビルド（プレリリース `nightly` タグ。[開発](development.md#nightly-ビルド)）も取り、両方のうち新しい方を選びます（`update.LatestFor`）。nightly の版は `git describe`（`0.1.17-16-ge057eae`）で、直前のリリースより新しく次のリリースより古いので、nightly のあとに安定版が出ればそちらに更新します。nightly が読めない、またはこの OS 向けのアーカイブが無いときは安定版だけで判断します。`stable` に戻すと、ダウンロード済みの nightly は破棄し（`dropStagedNightly`、起動時も同じ）、すぐ確認し直します。インストール済みの nightly より新しい安定版が出るまでは、そのまま使います（版を戻すことはしない）。
- **確認のタイミング**: `autoUpdate` がオンなら起動直後（1 秒後、goroutine なので起動は待たせない）と、その後 6 時間ごと。失敗したときは 5 分後から 1 時間まで間隔を倍にしながら再試行する。リリース情報はまず `https://mayak.ich.sh/api/release`（Worker が 5 分キャッシュ、GitHub の IP ごとの制限を受けない）から取り、届かなければ GitHub API に当たる。オフのときはステータスの「更新を確認」だけです。GitHub API は認証なしで呼びます（IP ごとに 60 回/時）。
- **ダウンロード**: リリースのアセットから、この OS と CPU 向けのアーカイブ（`Mayak-<version>-windows-amd64.zip`、`Mayak-<version>-darwin-arm64.tar.gz` など。名前は `update.ArchiveName`）と `SHA256SUMS.txt` を取り、チェックサムが一致したものだけを設定フォルダの `updates/` に展開します。アーカイブに無い OS なら「このOS向けのビルドはありません」になります。`autoUpdate` がオンなら見つけ次第、オフなら「ダウンロード」を押したときに始まります。展開先に `staged.json` が残っていれば次回起動時に引き継ぎ、現在の版より新しくなければ捨てます。
- **適用**: 展開したファイルを実行ファイルと同じフォルダへ入れ替えます。置き換える前のファイルは `*.mayak-old` に改名してから新しいものを置くので、実行中の exe（Windows では上書きも削除もできない）でも差し替えられます。失敗したときは改名したファイルを元に戻します。`autoUpdate` がオンなら終了時（`shutdown` の最後）に自動で適用し、次回起動から新しい版になります。ステータスの「再起動して更新」を押すと、その場で適用してから新しい版を起動し、自分は終了します。
- **再起動**: 新しいプロセスは環境変数 `MAYAK_UPDATE_WAIT_PID` で古いプロセスの終了を最大 30 秒待ってから起動します（二重起動防止と衝突しないため）。起動時には `*.mayak-old` を削除します。
- **通知**: 新しい版が見つかる・ダウンロード中・準備完了のあいだ、シェルはウインドウ下端に専用の行（32px）でアップデートバーを出します（`shell.js` の `updateBarHTML`。ページのネイティブビューはシェルより上に重なるので、重ねると隠れる。`api.js` の `bounds()` がその分ページを持ち上げます）。エラーの通知（「操作を完了できませんでした」）も同じ仕組みで、もう 1 行（`.error-bar`、アップデートバーの上）に出ます。行数は `statusRows` として `body[data-status-rows]` に渡り、CSS の `--status-bar` が 32px × 行数になります。「再起動して適用」は `InstallUpdate`（適用して再起動）、「ダウンロード」は `DownloadUpdate`（`autoUpdate` がオフのとき）、「変更点」はリリースページ、「あとで」はその版のあいだだけ消します。適用しなくても終了時に入れ替わります。
- **状態**: `GetUpdateStatus` と `update:status` イベント（`model.UpdateStatus`）。`state` は `idle`／`checking`／`current`／`available`／`downloading`（`progress` は %）／`ready`／`unsupported`／`error`。ログのカテゴリは `Update` です。

インストーラーはユーザー単位のフォルダ（`%LOCALAPPDATA%\Programs\MAYAK`）に入れるので、そのままこの差し替えが動きます。インストール先に書き込めない場合（管理者権限が要るフォルダなど）は適用に失敗し、エラーがステータスとログに出ます。その場合はリリースページから手動で入れ替えてください。リリースの作り方は [開発ガイド](development.md#リリース) を参照してください。

## TarkovTracker 連携

- **API**: `https://api.tarkovtracker.org`（`internal/tracker`）
- **トークンの取り込み**: `PVP_` / `PVE_` / `SZN_` の接頭辞からモード（`pvp` / `pve` / `seasonal`）を判定します。そのうえで `TokenInfo` を取得し、次を確認します。
  - 報告されたモードが接頭辞と一致すること
  - トークンの同一性
  - `GP`（Get Progress）と `WP`（Write Progress）の権限があること

  確認できたトークンはキーとして保存し、すぐにプロフィールへ付けられるときは付けます（`autoAssignTrackerKey`）: そのモードでキーの無いプロフィールが 1 つだけならそこへ、複数あればプレイ中のプロフィール（`status.Tracker` の account・profile・mode）がその中にあればそこへ。候補はアカウントごとの最新のプロフィールだけ（`latestTrackerProfiles`）。どちらでもなければ未割り当てのまま残し、通知はどこに付いたかを伝えます（`ImportTrackerToken` が付け先の説明を返す）。キー名には TarkovTracker の note を使い、起動時、tracker セクションの表示中は 60 秒ごと、およびウィンドウにフォーカスが戻ったときに更新します。
- **保存**（`internal/trackerstore`）: `%APPDATA%\Mayak\tracker-tokens.dat` に保存します。Windows では DPAPI（`CryptProtectData`）で暗号化し、それ以外の OS では平文です。ドキュメントのバージョンは 2 です。
- **プロフィールとの紐付け**: EFT のログ（`trackerlog`）から account・profile・mode の組を検出して記憶し、組ごとにキーを 1 つ割り当てます。設定ページは Step 1「TarkovTracker のキーを追加する」（Step の丸いラベル付きの見出し。枠で囲まない）（トークン入力、TarkovTracker とその API トークンのページへのリンク、(2) 登録したキーの一覧）と (1) Step 2「キーを EFT プロフィールに割り当てる」の順で、両方とも同じ見た目の区切りです。追加したキーは入力のすぐ下の一覧に現れます。(1) Step 2「キーを EFT プロフィールに割り当てる」: モードごとのパネル（PvP・Season・PvE の順）に分けたプロフィールごとの行。同じアカウントの同じモードで複数のプロフィールがあるときは最新（ログで最後に見たもの）だけを行に出して「最新」バッジを付け、古いもの（過去のワイプ）は「以前のプロフィール」に畳みます（割り当ては可能だが通常は不要。自動割り当ての候補にもならない: `latestTrackerProfiles`）（プレイ中のプロフィールの行は薄く着色、キーの無い行は「未割り当て」バッジ）に、付いているキー（名前とマスク済みトークン）と、同じモードのキーから選び直す小さなドロップダウン、「過去ログを再チェック」ボタン（キーがある行だけ。押したその行のプロフィールだけを同期する）。そのモードのキーが 1 つも無ければその旨を出す。(2) 「登録したキー」: 全キーを常に並べ、各行は TarkovTracker でのキーの名前を先頭に、モードのバッジとマスク済みトークン、下に付け先（未割り当てのキーは点線の枠で「下のプロフィールで選ぶと割り当てられます」）、右にゴミ箱の削除ボタン（付いているキーは削除できない）。どの割り当ても `SetTrackerProfileKey`（「未割り当て」を選ぶとそのキーをプロフィールから外す）です。
- **読み取るデータ**（`Progress`）: `tasksProgress`（完了・失敗）、`hideoutModulesProgress`、`displayName`、`playerLevel`、`meta.gameMode`。モードが一致しない場合はエラーにします。
- **書き込むデータ**: ログで検出したタスクの状態変化（`completed` / `failed` / `uncompleted`）を `SetTask` で送ります。`uncompleted`（再開）は、以前の状態が `failed` のときだけ送ります。キーを割り当てたあとの同期はライブで、MAYAK の起動中に EFT のログへ出たタスクの変化だけを送ります（起動時点のログの末尾から読むため）。キーをプロフィールに割り当てたとき（行のドロップダウン、割り当て待ちのキー、追加時の自動割り当てのどれでも）は、そのプロフィールの過去ログを裏で同期し（`syncAssignedHistory`）、結果を `tracker:history` イベントで設定ページのトーストに出します（送るものが無ければ何も出さない）。MAYAK を起動していなかった間の分は、行の「過去ログを再チェック」（`SyncTrackerProfileHistory`）で取り込みます。EFT の起動中は過去ログを同期しません（`eftdetect.GameRunning`: プロセス一覧に EscapeFromTarkov.exe があるか）。起動中はログが書き込まれ続け、ライブの同期も送っているので、そこへ古い状態の一括送信が後から届くと、タスクの状態が逆戻りするおそれがあるためです（例: 再開したタスクが「失敗」に戻る）。起動中はボタンを押せず、その理由を行の下に出します。キーの割り当て時の自動同期は EFT が閉じるまで待ち、閉じたら行います（`watchGame` が 5 秒ごとに確かめる。待っている同期は MAYAK を終了すると消えるが、ボタンが赤いまま残る）。プロフィールごとに最後に過去ログを同期した日時（`historySyncedAt`）を覚えておき、一度も同期していないプロフィールは、ボタンを赤くして行の下に説明を出します。そのプロフィールの最初のセッション以降のログを 1 回の走査で読み（`trackerlog.ProfileTaskHistory`）、タスクごとの最後の状態を `SetTasks` でまとめて送り、送った件数を通知します。1 つのプロフィールは 1 ワイプなので、開始地点は選ばせません。
- **接続状態**: `disabled` / `waiting-profile` / `missing-token` / `connecting` / `connected` / `error`
- **Hideout の進捗**: 検出中のアイデンティティと割り当てたキーから得た `hideoutModulesProgress` を、同じモードのカタログと組み合わせて表示します（読み取り専用）。Hideout のログイベントから TarkovTracker へ書き込むことはありません。状態は `disabled` / `waiting-profile` / `missing-token` / `waiting-progress` / `waiting-catalog` / `ready` です。詳細は [hideout.md](hideout.md) を参照してください。
- 内蔵ブラウザでは、TarkovTracker（`https://tarkovtracker.org/`）がサイドバーにピン留めした既定のブックマークです（以前は固定タブ）。

## tarkov.dev Remote Control

- `internal/remote` が `wss://socket.tarkov.dev` に接続し、`remoteTargets` の各 ID にマップ、タスク、位置を送ります。
  - `map` 役のターゲット: マップと位置を受け取る
  - `tasks` 役のターゲット: タスクを受け取る
- 内蔵ブラウザで開いた tarkov.dev のマップページ（`/map/…` と `/maps/`。どのタブでも）は `browserRemoteId` で自動接続します。ドキュメントスクリプトが `?connection=<ID>` を付与し、保存するタブの URL からは取り除きます。アドレスに `?connection=` があればその ID を優先します。この ID はマップと位置だけを受け取り、タスクはシェルが自分でタブを開きます。マップの検出そのものは、MAYAK の [マップ](browser-shell.md#マップ) に出ます。
- ID への接続に失敗しただけ（例: `websocket: bad handshake`）なら、その ID に何もつながっていないというだけなので、警告をログに出すだけです。接続状態をエラーにせず、通知音も鳴らしません。接続したあとの送信の失敗はエラーとして扱います（[tasks-and-maps.md](tasks-and-maps.md#接続の失敗)）。
- 起動時、`remoteTargets` があれば接続テストをバックグラウンドで実行します。
- 詳細は [tasks-and-maps.md](tasks-and-maps.md) を参照してください。

## Host / Client モード

シェルの `connection.mode` で決まります。

| モード | 意味 |
|---|---|
| `local` | 「Host：このPCでタルコフを起動する」（Host モード）。Windows のみ選択可。監視、認識、Host 設定を使える |
| `client` | 「Client：タルコフは別のPC（Host）で起動する」（Client モード）。Host とは中継サーバー経由の暗号化したリンクでつながる |

- Windows 以外で `local` が保存されている場合は `client` にします。認識できないモードと、前の版の「使わない」（`off`。ペアリングしていない Client と同じ動きだった）は `client` にします。Windows 以外では選ぶものが Client だけなので、選択欄は出さずに説明だけを出します。検出を止めたい Host は、監視のスイッチを切ります。WebRTC で直接つないでいたころの `webrtc` は `client` に読み替えます。旧 `remote`（LAN 受信）モードで保存されていた場合は `off` で起動します（`restore()`）。当時の接続先 URL とトークンは読み込まず、保存もしません。
- Go 側は `BrowserSetMode` でモード（`local` / `client` / `off`）を受け取り、`local` 以外のときは Client として扱います（`browserClient`）。Windows では起動時に `browser.json` を読み、`client`（旧 `webrtc`）/ `off` なら Client として起動します（Windows 以外は常に Client）。このとき OCR やフォルダの初期化、自動監視は行いません。
- 1 台の Host に、Client を何台でもつなげます（中継の部屋の上限は後述）。LAN は使わないので、同じネットワークにいる必要も、macOS のローカルネットワークの許可もいりません。

### ペアリング

`frontend/src/browser/peer-code.js`（招待コード）と `api.js`（`pairing`、`startLink`、`unpair`、`cancelCode`）が担当します。接続コードの受け渡しには、既定で `https://mayak.ich.sh/api/pair`（`site/worker/index.js`、コードごとの Durable Object）を使い、長いコードを手で渡すこともできます。

1. Host（`local`）が「接続コードを発行」を押すと、32 バイトの乱数の鍵を作り、招待を `POST /api/pair` に預けて 8 桁の接続コードを受け取ります（`peerInvite`）。Host は同時にその鍵のリンクに入り、相手を待ちます。
2. Client（`client`）は接続コードを入力します。`GET /api/pair/<code>` で招待を取り、その鍵でリンクに入ります（`peerJoin`）。中継に届かないとき（`relayError`）や「コードを手で渡す」を開いたときは、Host の長い招待コードをコピーして Client に貼り付けます（`peerAccept`）。応答のコードはありません。
3. 両方がリンクで会うと、ペアリングが決まります。両方が鍵と自分の役割を `connection.link`（`{key, role: "host" | "client"}`、この PC のデータ）に保存し、Host は接続コードを中継から消します（`DELETE /api/pair/<code>`）。以後は起動するたびに、コードなしで同じリンクにつなぎ直します。
4. 招待コードの形式は `MAYAK1.` + base64url(JSON `{version:2, type:"link", id, createdAt, key}`) です。`key` は鍵の base64url（43 文字）です。

- **期限**: 招待は 10 分で失効します。それまでに会えなければ、作りかけのペアリングも捨てます（`expireIn`）。「キャンセル」でも捨てます（`cancelCode`）。
- **中継サーバー**: 招待の文字列だけを、接続コードごとに 10 分間保持します（`MAYAK1.` で始まる 100000 文字以内のものだけ受け付けます）。招待には鍵が入っているので、10 分のあいだに 8 桁を当てて招待を取った人は、そのあともリンクに入れます。8 桁は総当たりに強くはないので、10 分の有効期限以上の保護はありません。
- **入力欄**: Client の接続コードは 1 桁ずつの 8 つの枠（4 桁ずつ。`code-boxes.js`、分隊コードと同じ部品）に打ちます。打つと次の枠へ進み、Backspace で前の枠に戻り、貼り付けると枠を埋めます。8 桁目を打つと、そのままつなぎます。
- **Host のゲームモード**: Host は、つながったときとゲームモード（TarkovTracker やログから判断したもの）が変わったときに、自分のモードを Client に送ります（`host:info`、`BrowserCatalogMode`）。Client の Go 側はそれを `BrowserSetHostMode` で受け取り、ボスと Goons、マップの印、アイテムの検索を Host と同じモードで読みます（`effectiveCatalogMode`。Client にはゲームのログが無いので、自分では判断しない）。Host が最新の状態を持ち、Client はそれを受け取って見る、という考え方です。
- **Host のマップと位置**: 同じ `host:info` に、Host が遊んでいるマップ（`map`）、レイド中か（`raid`）、最後の位置（`position`: `x`・`y`・`z`・`rot`・`at`）も載せ、どれかが変わるたびに送り直します（`shareHostInfo`。受ける側は `transport.js` の `hostInfoOf` で確かめる）。Client はそれを `hostView` に持ち、マップ画面は Host 自身と同じように、そのマップを出し（自動のとき）、分隊に入っていなければ Host の位置を自分の矢印として出します（`view-map.js` の `hostOf`。分隊では Host は仲間の 1 人として出るので出さない）。
- **お互いの PC の名前**: つながると、お互いのコンピューター名（`BrowserHostname`、`os.Hostname`）を出します。Client には「Host の PC」、Host には「つながっている PC」（Client ごと）、両方に「この PC」です。名前はリンクの `hello` に入れて送ります（暗号化される。名前のない古い版の相手は出さない）。
- **伏せ字**: 8 桁の接続コード、長い招待コード、Client の入力欄は「•」で出し、横の目のボタンで表示します（[browser-shell.md](browser-shell.md#マップ) の分隊コードと同じ `revealButton`）。コピーのボタンは伏せ字のままでも本当のコードをコピーします。
- **別の PC を追加**: ペアリング済みの Host は、同じ鍵で新しい接続コードを出せます。初めて会った Client（接続コードで入った PC）が来た時点でそのコードは消し、すでにペアリング済みの Client がつなぎ直しただけでは消しません。
- **解除**: Host で「すべての PC とのペアリングを解除」を押すと、つながっている Client すべてに伝わり、どの Client もペアを忘れます。Client で解除すると、その Client だけが抜けます。全員が同じ鍵を持つので、Host から 1 台だけを外すことはできません（外すときは全部を解除してつなぎ直す）。モードを切り替えると、作りかけのペアリングは捨て、保存したペアリングはそのモードが自分の役割（Host なら `local`、Client なら `client`）のときだけつなぎます。

### リンク

`frontend/src/browser/transport.js` の `MayakLink` が、中継の Worker「mayak-relay」（`relay/worker/index.js` の `LinkRoom`、[分隊ルーム](#分隊ルーム) と同じ Worker）の部屋 `wss://mayak-relay.ich.sh/link/<room>` に WebSocket でつなぎます。行き先は Go の `BrowserLinkRelay` から受け取ります。

- **部屋と暗号化**: room は `"mayak-link-room\0"` と鍵を続けた SHA-256 の 16 進 64 文字です。メッセージはすべて、鍵から HKDF-SHA256（info `"mayak-link-key v1"`）で作った鍵の AES-256-GCM で封じます。中継は読めない文字列を部屋の中で転送するだけです。
- **あいさつ**: 部屋に入ると、先にいる相手と `hello` を交わし、鍵で開けられた相手だけを仲間として数えます。Client どうしは互いを無視するので、Host は Client から、Client は Host からだけ受け取ります。
- **つなぎ直し**: 切れたら 2 秒、4 秒…と間隔を倍にしながら（最大 1 分）つなぎ直します。30 秒ごとに `ping` を送り（中継のランタイムが `pong` を返すので、Durable Object は起きません）、75 秒何も届かなければ切ってつなぎ直します。
- **中継の部屋**: `LinkRoom` は 10 本まで（Host と Client に、中継がまだ気づいていない切れた接続の分の余裕を足したもの）、1 メッセージ 128 KB まで、1 本 10 秒に 120 件まで。何も保存せず、あとから入った人に前のメッセージを送り直すこともしません。
- **共有する内容**: Host から Client へは `browser:task`（タスク）、`browser:map`（マップ）、`browser:position`（位置）、`browser:item`（アイテム情報）を送ります（アプリ側で 64 KiB を超えるものは送りません）。参加中の分隊（`squad:sync`）は両方向です（[browser-shell.md](browser-shell.md#マップ)）。設定、API キー、トークンは送りません。
- **Client で出すもの**: 各 Client は、Host から届いたもののうちこの PC で開くもの（タスク・マップと位置・アイテム）を選べます（`connection.receive`、この PC のデータ）。選ばなかったものは Client で捨てます。1 台はマップ、もう 1 台はタスク、のように分けられます。
- **状態**: 接続しています → 相手を待っています（Host には「相手の PC を待っています」、Client には「Host を待っています」）→ 接続中（Host では台数）、中継に届かないときは「つなぎ直します」。

## 分隊ルーム

マップ（[browser-shell.md](browser-shell.md#マップ)）の分隊の仲間は、Cloudflare Worker「mayak-relay」（`relay/worker/index.js`、`https://mayak-relay.ich.sh`）の部屋で会います。同じ Worker が Host と Client の [リンク](#リンク)（`/link/<room>`、`LinkRoom`）も受け持ちます。ランディングページの Worker（`mayak`）と分けてあるのは、Worker を deploy すると Durable Object が再起動して接続が切れるためです。サイトを更新しても分隊とリンクは切れません。

- **部屋**: `wss://mayak-relay.ich.sh/squad/<room>`。room は分隊コードの SHA-256（`squad.RoomID`）で、部屋ごとに Durable Object `SquadRoom` が 1 つあります。WebSocket Hibernation API を使うので、誰も送らないあいだは眠っています。`ping` にはランタイムが `pong` を返すので、DO は起きません。
- **中継の動き**: 接続ごとに乱数の ID を付け、`welcome`（自分の ID と、部屋にいる人の ID と最後のメッセージ、中継の版 `v`）、`join`、`leave`、`msg`（誰かのメッセージ）を送ります。メッセージは 4 KB まで、1 人 10 秒に 480 件まで（`welcome` の `rate` で知らせる）、1 人 1 分に 4 MB（文字数）まで（`bytes` で知らせる。スクショやスナップノートの絵は 100 件を超えるので、件数だけでは重いものが続けて流れる。超えると件数と同じく切る）、保存の同じ枠への書き込みは 5 秒に 1 回まで（早すぎると 429 と `Retry-After`）、部屋は 10 人まで（11 人目は HTTP 409）です。後から入った人に渡す「最後のメッセージ」は位置の報告だけで、先頭に `~` の付いたメッセージ（使い捨て。分隊ペン）は渡さず、流すだけです。全員が抜けると部屋の接続は消えます（同じコードでまた集まれます）。リンクの部屋も同じ仕組みで、上限と、最後のメッセージを送り直さないところ、保存を持たないところが違います。
- **分隊の保存**: 分隊の部屋は、メンバーごとの枠を HTTP で持ちます（`/squad/<room>/store`。`GET` で全部の枠を `{枠: 封じた中身}` で、`PUT /squad/<room>/store/<枠>` で置く。空で置くと消す）。枠の名前はメンバー鍵のハッシュ（`sha256("mayak-squad-slot\0" + 鍵)`。中継には鍵に戻せない）、中身は分隊コードの鍵で封じたもの（`{v: 2, d}`）なので、中継には読めません。1 枠 512 KB（Go は封じる前で 360 KB まで）、24 枠まで（25 人目は 409）。誰かが入るか、読むか置くたびに 1 週間先の alarm を置き直し、1 週間だれも使わなければ（その時まだつながっている人がいれば延ばす）全部消します。分隊ペンの線と消去を入れます（[browser-shell.md](browser-shell.md#分隊ペン)）。Go は `Client.Store` / `Client.Stored`（`internal/squad/store.go`）、シェルには `SquadStore` / `SquadStored` で出します。保存の無い古い中継では読むと 404 で、何も無いのと同じに扱います。
- **暗号化**: メッセージはすべて、分隊コードから HKDF-SHA256 で作った鍵を使い、AES-256-GCM で封じます（`internal/squad/seal.go`）。中継が見るのは room の ID と暗号文だけで、コード・名前・分隊カラー・位置は読めません。コードは 8 文字（約 40 bit）なので、総当たりへの強さはその程度です。
- **メッセージの種類**: 封じた中身の `v` が 1 なら位置の報告（`Report`: 表示名、分隊カラー、メンバー鍵、見るだけか、マップ・位置・時刻、キャラクター画面「全般」を読んでいればその要約 `profile`：ゲーム内の名前・レベル・レイド・キル・生還率・K/D・プレイ時間・画像の時刻、TarkovTracker と同期していればその要約 `tracker`：表示名・レベル・モード・完了と失敗のタスク数・ユーザー ID（`/token` の `owner`。トークンごとに 1 度だけ聞いて覚える）。同期のたびに変わっていれば送り直す。見るだけの PC は送らない）、2 ならシェルのメッセージ（分隊ペン。[browser-shell.md](browser-shell.md#分隊ペン)）です。2 は使い捨て（`~`）で送り、Go は中身を見ずに `squad:message` でシェルへ渡します（`SquadSend`）。2 を知らない版は開けないメッセージとして捨てるので、位置の共有は壊れません。
- **中継の版**: `welcome` の `v` が 2 以上の中継だけが、使い捨てのメッセージと 10 秒 120 件を受け付けます。中継を deploy しても、眠っている部屋の接続は切れずに残ることがあり、そのときは `welcome` が来ないので、古い版と思ったままになります。そこで古い版の中継につながっているあいだは、10 分ごとと、分隊ペンのボタンを押したとき（`SquadRecheck`）に、すぐつなぎ直して版を聞き直します。それより前の中継（30 件、使い捨てなし）では分隊ペンを押せなくし（`State.Drawing`）、線を送りません（送ると多すぎて切られ、位置の報告も後から入った人に渡らなくなるため）。
- **中継の上限の知らせ方**: `welcome` の `limits`（`rate`・`rateWindow`（ms）・`bytes`・`bytesWindow`（ms）、分隊では `storeEvery`（ms））で、上限をすべて知らせます（`rate` と `bytes` は前の版のためにも残す）。アプリはそれを読んで 6 分の 5 以内に収め、中継に断られてから直すのではなく、先に待ちます。リンクの部屋は 1 人 1 分に 16 MB まで（10 秒 120 件は同じ）で、リンク（`transport.js` の `say`）も `paceWait` で同じように待ってから送ります。
- **つなぎ過ぎ**: 同じ IP アドレスからの接続と保存の読み出し（GET）は 1 分に 60 回までです（Workers の Rate Limiting、`RELAY_CONNECTS`）。超えると部屋を起こす前に 429 と `Retry-After: 60` を返し、分隊のクライアントはその秒数だけ待ってからつなぎ直します（リンクはもともと 2・4・8… 秒、最長 1 分あけてつなぎ直す）。
- **保存の間隔**: 分隊のクライアントは、同じ枠を `storeEvery`（知らせない中継には 5 秒）あけて書きます（`pace`）。429 が来たら `Retry-After` だけ待って 1 回だけ書き直します。
- **送る数**: クライアントは、どの 10 秒でも中継の上限の 6 分の 5 を超えないように送ります（`Client.take`、`limits`。上限を知らせない中継は 120 件とみなして 100 件）。ペンの位置と描いている途中の点は、その 6 分の 5 までで止めて捨て、残りを線・消去・全消去・共有のために空けておきます（それらは空くまで待つ）。文字数も、どの 1 分でも中継の `bytes` の 6 分の 5（知らせない中継には 4 MB とみなして同じ）を超えないように送り、超えそうなら捨てられるものは捨て、ほかは空くまで待ちます（`byteLimit`。重いものが続くと、送り終わるまでに少し待つ）。
- **メンバー鍵**: PC ごとに 1 度作る 32 桁の 16 進（`squad-key.txt`）で、報告に入れます（`Report.Key`）。中継の ID はつなぎ直すたびに変わるので、分隊ペンの線の持ち主はこの鍵で覚えます。
- **クライアント**（`internal/squad/client.go`）: 切れたら、5 秒から 15 秒まで間隔を延ばしながらつなぎ直し（レイド中に長く離れないよう）、つながるたびに自分の最新の報告を送り直します。30 秒ごとに `ping` を送り、75 秒何も届かなければ切れたとみなします。部屋が変わるたびに `squad:state` をシェルへ送ります。
- **開発**: `task relay:dev` で中継（分隊とリンクの両方）をローカル（`ws://127.0.0.1:8787/squad/`、`ws://127.0.0.1:8787/link/`）に立て、開発版の MAYAK を環境変数 `MAYAK_SQUAD_RELAY=ws://127.0.0.1:8787/squad/`（分隊）や `MAYAK_LINK_RELAY=ws://127.0.0.1:8787/link/`（リンク。`BrowserLinkRelay`）付きで起動すると、そちらにつなぎます。どちらの変数も nightly 版と開発版（`version.IsPrerelease`）でだけ効きます。公開は `task relay:deploy` です（`wrangler login` が必要）。

### 地図データ（`internal/mapdata`）

- 地図の一覧と座標変換は tarkov.dev の `src/data/maps.json`（MIT、`raw.githubusercontent.com`）、地図の絵は `assets.tarkov.dev` の SVG（Shebuka ほか、CC BY-NC-SA 4.0）とタイルです。maps.json と SVG は最新を取り、`%AppData%\Mayak\maps\`（キャッシュ）に置いて、1 日ごとに ETag で確かめます。取れないときや形がおかしいときは、前に取れたものを使います。タイルは Leaflet が tarkov.dev から直接読みます。
- 使うのは、`projection: "interactive"` の地図の `transform`・`coordinateRotation`・`bounds`・`svgBounds`・`svgPath`・`svgLayer`・`tilePath`・`tileSize`・`heightRange`・`minZoom`・`maxZoom`・`author`・`authorLink`・`labels`（地名）・`layers[]`（`svgLayer`・`tilePath`・`show`・`extents`: 階の高さと範囲）と、`altMaps`（night-factory、ground-zero-21 などの別名）です。SVG かタイルのどちらかがある地図を残すので、タイルしかない地図（The Lab・Labyrinth・Icebreaker）も入ります。高さの無い地名は `Label.Ground`（地上の地名）にします。
- `BrowserSquadMapImage(map, layer, fade)` は SVG の先頭に `<style>` を足して、地上（`svgLayer` と `data-keep-with-group` のグループ）だけ、または選んだ階と `fade`% に薄くした地上を表示させ、data URL で返します。
- `BrowserMapTile(url)` は `assets.tarkov.dev/maps/…` のタイル 1 枚を data URL で返します（マップをスナップノートにするとき用。tarkov.dev のタイルは CORS を返さないため）。
- `BrowserMapMarkers(name, language, mode)` はカタログの `maps`・`items`・`tasks`（と表示言語の名前）から地点を作ります（`mapdata.Markers`、`internal/mapdata/markers.go`）。モード・マップ・言語と TarkovTracker のタスク状態ごとに 10 分間覚えます。

## ログ

- **アプリのログ**（`internal/applog`）: メモリ上に最大 500 件を保持し、`%APPDATA%\MAYAK\mayak.log` に 1 行 1 件で追記します。ファイルの行数がメモリ上の上限の 2 倍（1000 行）に達すると、メモリに残っている項目だけで書き直すので、ファイルは際限なく大きくなりません。ファイルへの書き込みは読み出し側のロックを持たずに行います（書き込み順は専用のロックで保ちます）。起動時にこのファイルから読み込みます。レベルは `Error` / `Warn` / `Info` / `Debug` です。
- 新しい項目は `log:entry` イベントで設定ページに送ります。設定ページも最新 500 件を保持します。
- **ログページ**（`logs` セクション）:
  - 上部の指標: 現在のマップ、レイド経過時間、ランスルーまでの残り時間、直前のマッチ待ち時間
  - フォルダを開くボタン: Screenshots、EFT Logs、デバッグ用フォルダ（`Mayak-Debug`）
  - フィルタ: カテゴリ（すべて / Hideout）、レベル、文字列検索
  - 件数表示と、新しい順の一覧
- Hideout のイベントは、`debug` がオンのとき、またはカテゴリを Hideout にしたときに一覧に混ぜて表示します。レベルは failed が Error、unknown が Warn、それ以外が Info です。
- 「ログを消去」（`ClearLogs`）は `mayak.log` と Hideout の履歴表示を空にし、`log:clear` を送ります。

## データの保存場所

すべて `%APPDATA%\MAYAK\`（`os.UserConfigDir()` 配下の `MAYAK`）に保存します。例外は `Mayak-Debug` だけで、EFT の Screenshots フォルダ内に作ります。

| パス | 内容 |
|---|---|
| `settings.json` / `settings.json.bak` | Host 設定と、その直前の内容 |
| `window.json` | メインウィンドウの位置 |
| `popup.json` | ポップアップウィンドウの位置 |
| `browser.json` | シェルの状態（ブラウザ設定、タブ、ブックマーク） |
| `processed-screenshot.json` | 最後に処理したスクリーンショットの指紋（パス、サイズ、更新時刻） |
| `tracker-tokens.dat` | TarkovTracker のキーとプロフィールの割り当て（Windows では DPAPI で暗号化） |
| `mayak.log` | アプリのログ |
| `hideout\events.json` | Hideout イベントの履歴（最大 500 件・90 日）。診断ボタンでこのフォルダを開く。書き込みは変更から 0.5 秒後にまとめて 1 回（`hideoutlog.Store`。起動時のログ再生で数百件が続けて来るため）、終了時に残りを書く。重複判定は指紋の集合で行い、全件走査はしない |
| `catalog\<mode>.json` | モード別のカタログキャッシュ（[catalog.md](catalog.md)） |
| `favicons\` | サイトアイコンのキャッシュ |
| `thumbs\` | スクリーンショットの縮小画像のキャッシュ（最大 1,500 件、古い順に間引き。[browser-shell.md](browser-shell.md)） |
| `adblock\<list>.txt` | 広告ブロックのフィルタ（`easylist`、`easyprivacy`、`adguard-japanese`。4 日で期限切れ、失敗時は 6 時間後に再試行） |
| `browser-webdata\` | 内蔵ブラウザ（WebView2）のプロファイル |
| `<Screenshots>\Mayak-Debug\` | `saveRecognitionDebug` で保存する原画像、crop、JSON |
