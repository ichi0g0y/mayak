# 設定と連携

MAYAK の設定は保存場所の異なる 2 系統に分かれています。

- **Host 設定**: Go 側の `internal/config.Settings`。`settings.json` に保存され、React の設定ページ（`frontend/src/main.tsx`）で編集します。このページはブラウザシェルの設定タブ内に `<iframe id="host-settings">`（`/settings.html#<section>`）として埋め込まれます。
- **ブラウザ設定**: シェル側の状態（`frontend/src/browser/state.js`）。`browser.json` に保存されます。

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
- 設定ページは項目を変更するたびに `PersistSettings` を呼んで即時保存します。実行中のサービスは再起動しません。明示的な保存（エラー時の「再試行」）は `SaveSettings` を使い、監視中にフォルダが変わった場合はウォッチャーも張り替えます。
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
| `browserRemoteId` | 自動生成 | 内蔵ブラウザのマップビュー用 Remote Control ID（12 文字） |
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
| `soundsEnabled` | `true` | 通知音のマスタースイッチ |
| `questSoundEnabled` / `questSoundPath` | `true` / `""` | クエスト認識に成功したときの音 |
| `errorSoundEnabled` / `errorSoundPath` | `true` / `""` | 認識エラーや接続エラーのときの音 |
| `matchFoundSoundEnabled` / `matchFoundSoundPath` | `false` / `""` | マッチ成立時の音 |
| `raidStartSoundEnabled` / `raidStartSoundPath` | `false` / `""` | レイド開始時の音 |
| `runThroughSoundEnabled` / `runThroughSoundPath` | `false` / `""` | ランスルー時間が過ぎたときの音 |
| `runThroughSeconds` | `430` | ランスルー時間（秒）。1–3599 の範囲外なら 430 |
| `questItemsSoundEnabled` / `questItemsSoundPath` | `false` / `""` | レイド開始時にタスクアイテムの確認を促す音 |
| `restartTasksSoundEnabled` / `restartTasksSoundPath` | `false` / `""` | 失敗したタスクがあるとき、レイド開始時に鳴らす音 |
| `hideoutErrorNotifications` / `hideoutErrorSoundPath` | `false` / `""` | Hideout 操作の失敗を通知する |
| `soundVolume` | `28` | 音量（0–100 に丸める） |
| `autoStartMonitoring` | `true` | 起動時に監視を開始する |
| `openMapOnRaidStart` | `true` | レイド開始時に tarkov.dev を現在のマップに切り替える |
| `navigateMapOnPositionScreenshot` | `true` | 位置スクリーンショットの送信後、そのマップに切り替える |
| `playerMarkerEffect` | `"none"` | 内蔵ブラウザの tarkov.dev マップに出る自分の位置マーカーのエフェクト。`none` / `outline` / `glow` / `pulse` / `beacon`。それ以外は `none` |
| `playerMarkerColor` | `""` | エフェクトの色（`#rrggbb`、小文字に揃える）。空ならエフェクト固有の色（縁取りは白、ほかは `#ff3b30`） |
| `playerMarker` | — | 0.1.15 の単一の選択。読み込み時に `playerMarkerEffect` と `playerMarkerColor` に読み替えて消す（`large` / `outline` → `outline`、`glow-red` → `glow`、`glow-green` → `glow` + `#3dff6e`、`custom` / `default` → `none`） |
| `tarkovTrackerEnabled` | `false` | TarkovTracker との同期 |
| `startMinimized` | `false` | 最小化した状態で起動する（「起動とウィンドウ」）。ウインドウの位置を復元したあとで最小化し、`minimizeToTray` が真ならトレイに入る |
| `minimizeToTray` | `false` | 最小化したときにウィンドウを隠し、タスクバーから消す |
| `closeToTray` | `false` | 閉じるボタンでは終了せず、トレイに常駐する |
| `keepPriority` | `false` | 起動 3 秒後と以後 10 秒ごとに、MAYAK 自身と直下の `msedgewebview2.exe` が Idle / BelowNormal なら Normal に戻す（`guardPriority`、Windows のみ）。Process Lasso の ProBalance などが起動直後の CPU 使用で優先度を下げ、そのあいだに生まれた WebView2 プロセスが低いまま残って UI が固まって見えるときに使う |
| `launchAtStartup` | `false` | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` に登録する |
| `autoUpdate` | `true` | GitHub Releases の新しい版を自動で確認・ダウンロードし、終了時に適用する（[自動アップデート](#自動アップデート)） |
| `windowX` / `windowY` / `windowWidth` / `windowHeight` / `windowConfigured` | `0` / `false` | 旧形式のウィンドウ位置。現在は `window.json` を使う |

効果音のパスはトリムと `filepath.Clean` を通します。空のときは組み込み音を使い、ファイルの再生に失敗した場合も組み込み音にフォールバックします。

### 設定ページのセクション別の項目

| セクション | 項目 | 操作 |
|---|---|---|
| `folders` | `screenshotDirectory`, `logsDirectory`, `screenshotCleanup`（オンのとき `screenshotRetainCount` と `screenshotRetainHours` を表示） | 自動検出（`AutoDetectEFTDirectories`）、フォルダ選択 |
| `recognition` | `gameMode`, `gameLanguage`（選択肢は「自動」と Go の `GameLanguages()` が返す言語。表示名は `main.tsx` の `languageNames`）, `ocrEngine`, `tesseractPath`（`tesseract` のときのみ表示） | — |
| `remote` | `remoteTargets`（名前、ID、役割「マップ」「タスク」）, `map`, `openMapOnRaidStart`, `navigateMapOnPositionScreenshot` | 追加、ブラウザからの ID 自動検出（`AutoDetectRemoteID`）、接続テスト（`TestRemote`） |
| `tracker` | `tarkovTrackerEnabled`、トークンの取り込み、保存済みキー、既知プロフィールへのキーの割り当て、過去ログの同期 | ログからプロフィールを探す、tarkovtracker.org の API 設定を開く |
| `sounds` | `soundsEnabled`。オンのとき、各通知（Hideout エラー、クエスト認識成功、認識・接続エラー、マッチ成立、レイド開始、ランスルー終了、タスクアイテム確認、失敗タスクの再開確認）の ON/OFF・音声ファイル・リセット・試聴、ランスルー時間（分・秒。ランスルー終了の通知がオンのときだけ表示）、`soundVolume` | `ChooseSoundFile`, `PreviewSound` |
| `startup` | `launchAtStartup`, `startMinimized`, `autoStartMonitoring`, `minimizeToTray`, `closeToTray`, `autoUpdate` | — |
| `status` | 表示のみ: Remote 接続状態、現在のマップ、レイド状態、スクリーンショット種別、TarkovTracker の状態、最後の検出結果、カタログの状態、アップデートの状態 | 最新スクリーンショットの解析、TarkovTracker の更新、カタログの更新、タスクページを開く、更新の確認・ダウンロード・再起動して更新、リリースノートを開く |
| `logs` | 表示のみ（[ログ](#ログ) を参照） | フォルダを開く、ログの消去 |
| `debug` | `debug`, `saveRecognitionDebug`。`debug` がオンのときは Hideout 診断フォルダ、候補一覧、crop、目標も表示 | `OpenHideoutDiagnostics` |

## ブラウザ設定（シェル側）

`state.js` の `defaults()` と `restore()` で決まります。`persist()` が保存するのは下表の項目だけで、WebRTC の招待コードや UI のエラーは保存しません。

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
| `connection` | `{mode:"local", stun:"stun:stun.cloudflare.com:3478"}` | 接続方法と STUN サーバー（[Host / Client モード](#host--client-モード)） |
| `bookmarks` / `bookmarkRevision` | 既定のブックマーク / `2` | 最大 100 件。revision 1 でブックマークを追加し、revision 2 で TarkovTracker を `.org` に移行 |
| `favicons` | `{}` | ホスト名ごとのアイコン URL（最大 200 件） |
| `tabs` / `active` | マップ、TarkovTracker、設定 / `map` | 固定タブ `map` と `tracker` は常に先頭。タブは最大 80 個 |

ウィンドウの位置とサイズは `window.json` に保存し、`browser.json` には持ちません（旧形式の `window` キーは読み込まず、次の保存で消えます）。

### セクション別の項目

| セクション | 内容 |
|---|---|
| `appearance` | 表示言語、テーマ、時刻表示、タブとアイテム情報の位置、「チュートリアルを表示」 |
| `tasks` | Host 上（`local`）では Host の `questSite` を直接編集する（`hostQuestSite` アクション。シェルの `questSite` は `host` に戻る）。Client では `questSite`（「Host の設定に従う」を含む）。どちらでも `taskMode` を設定する |
| `adblock` | 有効化のチェックボックス。EasyList、EasyPrivacy、AdGuard 日本語フィルタを使う。tarkov.dev は対象外 |
| `connection` | 接続方法（`local` は Windows のみ / `webrtc` / `off`）、WebRTC のペアリング、STUN サーバー |
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

サイドバーの上部（横並びレイアウトではタブ列の右端）に 2 つのインジケーターを表示します。

| 表示 | 状態 |
|---|---|
| Host アイコン（`mode-status`） | `host`（`local`・Host モード）/ `linked`（Client で WebRTC 接続済み）/ `unlinked`（Client で未接続）/ `off`（受信 OFF）。ツールチップには「Hostモード · このPCで検出する」や「Clientモード · 直接接続中」などを表示する |
| 監視ドット（`monitor-toggle`） | Windows の Host のときだけ表示する。`on` クラスで監視中を示す。ツールチップは「監視中/監視停止中 · マップ · レイド中/外 · TarkovTracker: 状態 · クリックで開始/停止」。クリックで `StartMonitoring` / `StopMonitoring` を呼ぶ |

状態は Host が送る `status:update` イベント（`monitoring`、`currentMap`、`raidActive`、`tracker.connection`）から更新します。

## 通知

### トースト（設定ページ右下）

| 種類 | 表示条件 | 消え方 |
|---|---|---|
| 保存成功 | 自動保存の完了 | 2 秒で消える |
| 保存失敗 | `PersistSettings` / `SaveSettings` のエラー | 次の保存が成功するまで残る（× は無い）。「再試行」ボタン付き |
| 通知（成功） | 各操作の成功メッセージ | 3 秒で消える |
| 通知（エラー） | 各操作の失敗 | × で閉じるまで残る |
| Hideout 警告 | `hideout:alert` イベント | × で閉じるまで残る |

### 通知音

- すべての音は `soundsEnabled` と各項目のスイッチの両方が真のときだけ鳴ります。音量は共通の `soundVolume` です。
- クエスト認識成功と認識エラーは、同じキーで 3 秒以内なら繰り返し鳴らしません（`recognitionSoundCooldown`）。

| 通知 | 鳴るタイミング |
|---|---|
| クエスト認識成功 | タスクを認識したとき |
| 認識・接続エラー | 認識できなかったとき、または実行時エラー |
| マッチ成立 | ログでマッチ成立を検出したとき |
| レイド開始 | ログでレイド開始を検出したとき |
| タスクアイテム確認 | レイド開始時 |
| 失敗タスクの再開確認 | レイド開始時に、TarkovTracker 上で失敗しているタスクがあるとき |
| ランスルー終了 | PvE（設定または自動判定）、またはランスルー判定の対象となるレイドで、開始から `runThroughSeconds` が経過し、まだレイド中のとき |
| Hideout エラー | 下記 |

### Hideout エラー通知

- `hideoutErrorNotifications` が真の場合、新しく追記された失敗イベント（`status == "failed"` かつ過去ログではないもの）に対して、`hideout:alert` トーストを出し、`soundsEnabled` なら `hideoutErrorSoundPath` の音（既定はエラー音）を鳴らします。
- 結果を確認できなかった操作（unknown）の通知と、その設定は削除されました（`shouldNotifyHideout`）。unknown のイベントも Hideout の履歴には残り、ログページでは `Warn` として表示します。
- 詳細は [hideout.md](hideout.md) を参照してください。

## トレイとウィンドウ

- **トレイ**（`tray.go`）: アイコンをクリックまたはダブルクリックすると `showWindow` でウィンドウを表示します。メニューは「MAYAKを開く / Open MAYAK」と「終了 / Quit」で、表示言語は `language` に従います。保存した `language` が変わると、その場でメニューの文言を切り替えます（`setTrayLanguage`、再起動は不要）。
- **最小化**: `minimizeToTray` が真なら、最小化したときにウィンドウを `Hide()` し、タスクバーから消します。ウィンドウのフックは起動処理（`ServiceStartup`）より先に走ることがあるため、`main.go` がウィンドウを作る前に保存済みの設定を入れておきます。起動処理は設定をロックを取って置き換えます。
- **閉じる**: `closeToTray` が真なら、閉じる操作をキャンセルしてウィンドウを隠します。終了はトレイメニューからのみ行えます（`quitting` フラグで区別）。
- **二重起動の防止**: `SingleInstance`（`UniqueID: com.ichi0g0y.mayak`）で制御します。2 つ目のプロセスを起動すると、既存のウィンドウを表示します。
- **ウィンドウ**: フレームレスです。既定サイズは 1120×760、最小サイズは 760×560 です。
- **位置の復元**（`window.json`）:
  - 移動、リサイズ、最大化、最大化解除、閉じる、終了のたびに保存します。最小化中の座標や、最小サイズ未満のサイズは保存しません。
  - ディスプレイ名・ID と作業領域の原点を記録し、ディスプレイ基準の相対座標で復元します。該当するディスプレイが無い場合は、重なりが最大のディスプレイ、なければプライマリディスプレイを使い、作業領域内に収めます。最大化状態も復元します。
  - 保存値が異常な場合（幅 5000 超など）は無視します。
- アイテム欄から開くポップアップウィンドウも、同じ方式で `popup.json` に位置を保存します。

## 自動アップデート

MAYAK は [GitHub Releases](https://github.com/ichi0g0y/mayak/releases) から自分自身を更新します（`internal/update`、`internal/app/app_update.go`）。Windows／macOS／Linux のどれでも同じ仕組みです。

- **版の比較**: ビルドに埋め込まれた版（`internal/version`、`task build` が `git describe` かリリースタグから `-ldflags -X` で入れる）と、GitHub の「latest」リリース（ドラフトとプレリリースは除く）のタグを semver で比べます。タグ直後のコミットを含む開発ビルド（`0.1.0-3-g1a2b3c4`）は `0.1.0` より新しい扱いなので、同じ版の通知は出ません。版が入っていない `dev` ビルドはどのリリースよりも古い扱いです。
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
- **プロフィールとの紐付け**: EFT のログ（`trackerlog`）から account・profile・mode の組を検出して記憶し、組ごとにキーを 1 つ割り当てます。設定ページは Step 1「TarkovTracker のキーを追加する」（Step の丸いラベル付きの見出し。枠で囲まない）（トークン入力、(2) 割り当て待ちのキー、(3) 折りたたみのキーの管理）と (1) Step 2「キーを EFT プロフィールに割り当てる」の順で、両方とも同じ見た目の区切りです。追加したキーは入力のすぐ下に現れ、割り当てるとプロフィール行へ移ります。(1) Step 2「キーを EFT プロフィールに割り当てる」: モードごとのパネル（PvP・Season・PvE の順）に分けたプロフィールごとの行。同じアカウントの同じモードで複数のプロフィールがあるときは最新（ログで最後に見たもの）だけを行に出して「最新」バッジを付け、古いもの（過去のワイプ）は「以前のプロフィール」に畳みます（割り当ては可能だが通常は不要。自動割り当ての候補にもならない: `latestTrackerProfiles`）（プレイ中のプロフィールの行は薄く着色、キーの無い行は「未割り当て」バッジ）に、付いているキー（名前とマスク済みトークン）と、同じモードのキーから選び直す小さなドロップダウン、過去ログ同期のボタン（キーがある行だけ）。そのモードのキーが 1 つも無ければその旨を出す。(2) 「割り当て待ちのキー」: どのプロフィールにも付いていないキーだけを点線の枠で並べ、そのモードで未割り当ての最新プロフィールが 1 つ（または複数のうちプレイ中のもの）なら「最新のプロフィールに割り当て」の 1 クリック、ほかは未割り当てのプロフィールを選ぶドロップダウン、削除ボタン。(3) 「キーの管理（名前・削除）」: 折りたたみに全キー（付け先の表示と削除。付いているキーは削除できない）。どの割り当ても `SetTrackerProfileKey`（「未割り当て」を選ぶとそのキーをプロフィールから外す）です。
- **読み取るデータ**（`Progress`）: `tasksProgress`（完了・失敗）、`hideoutModulesProgress`、`displayName`、`playerLevel`、`meta.gameMode`。モードが一致しない場合はエラーにします。
- **書き込むデータ**: ログで検出したタスクの状態変化（`completed` / `failed` / `uncompleted`）を `SetTask` で送ります。`uncompleted`（再開）は、以前の状態が `failed` のときだけ送ります。過去ログの同期では、選んだブレークポイント（ゲームのバージョンと開始日時）以降のタスク状態を `SetTasks` でまとめて送ります。
- **接続状態**: `disabled` / `waiting-profile` / `missing-token` / `connecting` / `connected` / `error`
- **Hideout の進捗**: 検出中のアイデンティティと割り当てたキーから得た `hideoutModulesProgress` を、同じモードのカタログと組み合わせて表示します（読み取り専用）。Hideout のログイベントから TarkovTracker へ書き込むことはありません。状態は `disabled` / `waiting-profile` / `missing-token` / `waiting-progress` / `waiting-catalog` / `ready` です。詳細は [hideout.md](hideout.md) を参照してください。
- 内蔵ブラウザでは、TarkovTracker（`https://tarkovtracker.org/`）が 2 番目の固定タブとして常に開いています。

## tarkov.dev Remote Control

- `internal/remote` が `wss://socket.tarkov.dev` に接続し、`remoteTargets` の各 ID にマップ、タスク、位置を送ります。
  - `map` 役のターゲット: マップと位置を受け取る
  - `tasks` 役のターゲット: タスクを受け取る
- 内蔵ブラウザのマップビューは `browserRemoteId` で自動接続します。ドキュメントスクリプトが `?connection=<ID>` を付与し、タブの URL からは取り除きます。アドレスに `?connection=` があればその ID を優先します（ID を作り直したあと、古いスクリプトのままのタブを新しい ID で開き直すため）。この ID はマップと位置だけを受け取り、タスクはシェルが自分でタブを開きます。
- 起動時、`remoteTargets` があれば接続テストをバックグラウンドで実行します。
- 詳細は [tasks-and-maps.md](tasks-and-maps.md) を参照してください。

### 現在位置のマーカー（`app_marker.go`）

- tarkov.dev のマップは自分の位置を Leaflet の `divIcon`（class `marker`）で描き、中身は `<img src="/maps/interactive/player-position.png" style="width:24px;height:24px;rotate:Ndeg">`（向きが無いときは `player-position-no-rotation.png`）、アンカーは中心です。レイヤーグループ名は `player-position`、表示中はマップのコンテナに `player-position-shown` が付きます。
- 設定 `playerMarkerEffect` と `playerMarkerColor` に応じたスタイルシートを、ドキュメントスクリプト（`tarkovDevScript`。Remote Control の接続と同じスクリプト。マップページと `/maps/` だけ）が `<style id="mayak-player-marker">` として差し込みます。画像は `img[src$="/player-position.png"]` で選び、マーカーの箱には `:has()` で届きます。`rotate` は独立したプロパティなので、シートの `filter` と合成され、tarkov.dev の向きは保たれます。
- エフェクト（`playerMarkerRules`）: `outline`（細い影 3 枚で作る縁取り）、`glow`（光彩）、`pulse`（箱の `::before` に脈打つリング）、`beacon`（光彩と、箱の `::before` に色が薄れていく放射状の光。`rgba()` で色にアルファを付ける）。色は `playerMarkerColor`、空ならエフェクト固有の色（縁取りは白、ほかは `#ff3b30`）。どれもアイコン自体は tarkov.dev の画像と 24 px のままで、拡大も差し替えもしません（効果だけが外へ広がる）。0.1.15 で試した画像の置き換えはやめました（純正のアイコンのまま、エフェクトと色だけを変える）。
- 設定ページのギャラリーは同じ規則を `.marker-preview[data-effect=…] .marker-icon`（と `img`）向けに、選んでいる色で作ったシート（`PlayerMarkerPreviewCSS(color)`）で、`public/marker-arrow.svg`（tarkov.dev のマーカーに似せた絵）を 35° 回して見せます。色の入力（`<input type="color">`）はプレビューにすぐ反映し、設定への保存は 400 ms 手を止めてからです（保存のたびにマップビューを作り直すため）。
- 保存時にマーカーの設定が変わると `applyBrowserScript` でシェルとポップアップのドキュメントスクリプトを作り直し、`browser:document-script` を送ります。開いているページはスクリプトを持ち替えないので、シェルはマップビューを閉じて作り直します（表示中なら `show`、それ以外は `preload`）。
- 対象は内蔵ブラウザのマップだけです。自分の Chrome で開いた tarkov.dev には効きません（Stylus などで同じ CSS を入れる形になります）。tarkov.dev がファイル名やクラスを変えると効かなくなり、標準の表示に戻ります。

## Host / Client モード

シェルの `connection.mode` で決まります。

| モード | 意味 |
|---|---|
| `local` | このPCで検出する（Host モード）。Windows のみ選択可。監視、認識、Host 設定を使える |
| `webrtc` | インターネット経由の WebRTC 直接接続で、別の PC の Host から受信する（Client モード） |
| `off` | 接続しない（受信 OFF） |

- Windows 以外で `local` が保存されている場合は `webrtc` にします。認識できないモードは `off` にします。旧 `remote`（LAN 受信）モードで保存されていた場合は `off` で起動します（`restore()`）。当時の接続先 URL とトークンは読み込まず、保存もしません。
- Go 側は `BrowserSetMode` でモードを受け取り、`local` 以外のときは Client として扱います（`browserClient`）。Windows では起動時に `browser.json` を読み、`webrtc` / `off` なら Client として起動します（Windows 以外は常に Client）。このとき OCR やフォルダの初期化、自動監視は行いません。

### WebRTC ペアリング

`frontend/src/browser/peer-code.js` と `transport.js` が担当します。offer と answer の交換は、既定では 8 桁の接続コードで `https://mayak.ich.sh/api/pair`（`site/worker/index.js`、コードごとの Durable Object）を経由し、手動でも交換できます。

1. Host（`local`）が「接続コードを発行」を押すと、offer を作成して ICE の収集（最大 10 秒）を待ち、招待コードを `POST /api/pair` に預けて 8 桁の接続コードを受け取ります。以後 2 秒ごとに `GET /api/pair/<code>/answer` で応答を待ちます（`peerInvite`）。
2. 受信側（`webrtc`）は接続コードを入力します。`GET /api/pair/<code>` で招待コードを取り、応答コードを作って `PUT /api/pair/<code>` に置きます（`peerJoin`）。
3. Host が応答を受け取ると `peerAnswer` と同じ手順で接続し、`DELETE /api/pair/<code>` で中継を消します。中継が使えないとき（`relayError`）や「コードを手動で交換する」を開いたときは、従来どおり招待コードと応答コードをコピーして渡します（`peerAccept` → `peerAnswer`）。
4. コードの形式は `MAYAK1.` + base64url(JSON `{version:1,type,id,createdAt,sdp}`) です。応答の `id` と `createdAt` は招待と一致している必要があります。

- **中継サーバー**: 招待と応答の文字列だけを、接続コードごとに 10 分間保持します（`MAYAK1.` で始まる 100000 文字以内のものだけ受け付け、応答は一度だけ）。接続の中身は通りません。8 桁は総当たりに強くはないので、10 分の有効期限と「先に届いた応答を受け付ける」以上の保護はありません。

- **期限**: 招待コードは 10 分で失効します。接続の待ち時間は、送信側が 45 秒、受信側が 120 秒です。
- **検証**: SDP は `m=application` のみを許可し、sha-256 fingerprint を必須とします。音声・映像の m 行と relay 候補を含むものは拒否します。コードの長さは 100000 文字までです。
- **STUN**: 既定は `stun:stun.cloudflare.com:3478` です。`stun:` / `stuns:` 形式だけを受け付け、空欄にすると STUN を使いません。TURN（中継）は使わず、接続後に選ばれた経路が relay であれば切断します（`relay-rejected`）。そのため、回線によっては接続できません。
- **LAN**: 双方の候補が `host` 型であれば、経路を `local`（LAN 内の直接接続）と判定します。それ以外は `direct` です。
- **共有する内容**: ordered DataChannel `mayak-display-v1` で、`browser:task`（タスク）、`browser:map`（マップ）、`browser:item`（アイテム情報）の 3 種類だけを Host から受信側へ一方向に送ります。1 メッセージは 64 KiB までで、送信バッファが 256 KiB を超えると `slow-peer` として切断します。設定、API キー、トークンは送りません。受信側は接続を確認するまで、最大 32 件のメッセージを保留します。
- アプリを再起動した場合や接続が切れた場合は、コードを再交換する必要があります。コードは保存しません。

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
| `hideout\events.json` | Hideout イベントの履歴（最大 500 件・90 日）。診断ボタンでこのフォルダを開く |
| `catalog\<mode>.json` | モード別のカタログキャッシュ（[catalog.md](catalog.md)） |
| `favicons\` | サイトアイコンのキャッシュ |
| `adblock\<list>.txt` | 広告ブロックのフィルタ（`easylist`、`easyprivacy`、`adguard-japanese`。4 日で期限切れ、失敗時は 6 時間後に再試行） |
| `browser-webdata\` | 内蔵ブラウザ（WebView2）のプロファイル |
| `<Screenshots>\Mayak-Debug\` | `saveRecognitionDebug` で保存する原画像、crop、JSON |
