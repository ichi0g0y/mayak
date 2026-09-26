# 開発ガイド

MAYAK は Go + Wails v3（`v3.0.0-beta.24` 固定）で作られた Windows 向けの Escape from Tarkov コンパニオンアプリです。フロントエンドは Vite でビルドし、設定画面は React、ブラウザシェルは素の JavaScript で書かれています。この章では、リポジトリの構成、ビルドと開発ループ、テスト、守るべき制約をまとめます。

## 前提ツール

| ツール | 用途 |
| --- | --- |
| [mise](https://mise.jdx.dev/) | 下の Go、bun、Task をリポジトリ直下の `mise.toml` のバージョンで用意する |
| Go（`go.mod` は `go 1.26.4`、`toolchain go1.26.8`） | アプリ本体、各種コマンド |
| [Task](https://taskfile.dev/)（または `wails3 task`） | `Taskfile.yml` の実行 |
| bun | フロントエンドの依存インストール、Vite の実行、`bun test` |
| 7-Zip（`C:\Program Files\7-Zip\7z.exe`） | `task tesseract:bundle` がインストーラーを実行せずに展開するため |
| Node.js と wrangler | ランディングページ（`site/`）を Cloudflare に公開する（`task site:deploy`）。Node は wrangler を動かすためだけに入れる |

`mise install` で `mise.toml` のツールが入ります（Node と wrangler も含みます）。mise の shims フォルダ（Windows では `%LOCALAPPDATA%\mise\shims`）を PATH に入れておくと、どのシェルからでもこのバージョンが使われます。アプリとフロントエンドのビルドに Node.js は使いません（wrangler を動かすためだけに入ります）。`task dev` は最初に `bun install --frozen-lockfile` を実行するので、新しいワークツリーでもそのまま起動できます。

Wails CLI はインストール不要です。`Taskfile.yml` は `go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.24` を固定バージョンで呼び出します。フロントエンドの `@wailsio/runtime` も同じ `3.0.0-beta.24` に揃えています。

## リポジトリ構成

### ルートの `main.go` とアプリ本体（`internal/app`）

プロジェクト直下の Go ファイルは `main.go`（`package main`）だけです。`go:embed` は自分より上の階層を埋め込めないので、`frontend/dist` とトレイアイコン（`build/appicon.png`）を埋め込むこのファイルは直下に置き、`app.Run` に渡します。Windows リソース（`mayak_windows_*.syso`）も main パッケージと同じ直下で `go build` にリンクされます。

アプリ本体は `internal/app`（`package app`）です。`App` 型は Wails のサービスとして 1 つだけ登録され、公開メソッドがフロントエンドのバインディング（`frontend/bindings/github.com/local/mayak/internal/app`）になります。ファイルは責務ごとに分かれています。

| ファイル | 責務 |
| --- | --- |
| `main.go`（直下） | `frontend/dist` とトレイアイコンを `go:embed` で埋め込み、`app.Run` を呼ぶ |
| `run.go` | `application.New` でアプリを作成し、`App` をサービス登録、多重起動を防ぎ（2 つ目の起動は既存ウィンドウを表示）、フレームレスのメインウィンドウを作る。保存済みの位置・サイズはネイティブウィンドウ生成前のオプションで渡し、`startMinimized` なら配置の復元後に最小化する |
| `tray.go` | Wails 標準の `SystemTray` によるトレイアイコンとメニュー（開く／終了、UI 言語に追従） |
| `app.go` | `App` 本体。起動・終了、ログ、状態の通知、監視の開始・停止、ゲームログの追跡（マップ・レイド・走り抜け警告）、通知音 |
| `app_settings.go` | 設定の読み書き（`PersistSettings` は即時保存、`SaveSettings` は稼働中の監視へも反映。どちらも `saveSettings`）、フォルダの自動検出と選択、値の正規化 |
| `app_recognition.go` | スクリーンショット認識：位置、タスク・アイテムのタイトル、OCR エンジンの選択（[recognition.md](recognition.md)） |
| `app_tracker.go` | TarkovTracker 連携：キーとプロフィール、進捗の取得、認識したタスクの同期（[settings-and-integrations.md](settings-and-integrations.md)） |
| `app_remote.go` | 別の MAYAK へのマップ・タスク・位置の送信と接続テスト |
| `app_lifecycle.go` | Wails v3 の `ServiceStartup` / `ServiceShutdown` を既存の起動・終了処理へ接続 |
| `app_events.go` | Wails アプリ内イベントの発行（外部へのブリッジは起動しない） |
| `app_browser.go` | ブラウザシェルの状態（`browser.json`）、ホスト／クライアントモード切替 |
| `app_browser_remote.go` | 内蔵ブラウザ用 tarkov.dev Remote Control ID の生成と、マップページへの自動接続スクリプト |
| `app_adblock.go` | 内蔵ブラウザの広告ブロック設定 |
| `app_favicon.go` | サイトアイコンのディスクキャッシュ |
| `app_catalog.go` | ゲームデータカタログの更新と定期監視（[catalog.md](catalog.md)） |
| `app_screenshots.go` | スクリーンショット解析後の処理、デバッグ用メタデータ、古いスクリーンショットの整理 |
| `app_item.go` | 認識したアイテムをアイテム欄へ送る。アイテム検索と詳細取得 |
| `app_popup.go` / `app_popup_windows.go` / `app_popup_other.go` | アイテムポップアップウィンドウ。Windows ではメインウィンドウをオーナーにする |
| `app_quest_site.go` | タスクページの行き先（tarkov.dev／公式 Wiki／日本語 Wiki）の URL 組み立て |
| `app_hideout.go` | ハイドアウト進捗と通知 |
| `app_tracker_names.go` | TarkovTracker キーの名前（メタデータ）の再取得 |
| `app_window.go` | ウィンドウ位置の保存と復元（ディスプレイ移動・最大化を含む） |
| `app_window_theme.go` / `_windows.go` / `_other.go` | タイトルバーと枠の色をシェルのテーマに合わせる（Windows は DWM） |

`mayak_windows_*.syso` は `task build:windows` が生成する Windows リソースで、Git では無視されます。

### `internal/` パッケージ

| パッケージ | 概要 |
| --- | --- |
| `adblock` | AdGuard `urlfilter` と EasyList 系リストで内蔵ブラウザの広告・トラッカーをブロック |
| `applog` | アプリ内ログのメモリ上リングバッファ |
| `autostart` | Windows のスタートアップ登録（他 OS では無効） |
| `browserview` | 外部サイト用のネイティブビュー（Windows: WebView2、macOS: WKWebView、Linux: WebKitGTK）と、Windows のリクエストフィルタ（[browser-shell.md](browser-shell.md)） |
| `catalog` | tarkov.dev のデータ（アイテム・マップ・タスク・ハイドアウト）の取得とキャッシュ、定期更新 |
| `config` | `settings.json`、ウィンドウ位置（`window.json`／`popup.json`）、処理済みファイルの記録 |
| `eftdetect` | EFT のフォルダとゲーム表示言語の自動検出 |
| `hideoutlog` | EFT ログからのハイドアウト操作イベント検出 |
| `itemapi` | `json.tarkov.dev` からのアイテム一覧取得 |
| `itemdetect` | スクリーンショット内のアイテム詳細ウィンドウの検出 |
| `iteminfo` | アイテム欄の表示内容（フリマ・トレーダー価格、必要なタスク・ハイドアウト、価格履歴） |
| `itemmatch` | OCR 結果からアイテム名への照合（日本語名を含む） |
| `locale` | 英語以外に読むゲーム言語の一覧（[languages.md](languages.md)） |
| `logdetect` | EFT ログからのレイド状態の判定 |
| `model` | 共有データ型、ハイドアウト進捗の計算 |
| `ocr` | OCR エンジン（同梱 Tesseract、Windows OCR の PowerShell ワーカー）と前処理 |
| `position` | スクリーンショットのファイル名からの座標・向きの解析 |
| `questapi` | タスク一覧、日本語名・公式 Wiki による補完 |
| `questmatch` | OCR 結果からタスク名への照合 |
| `remote` | tarkov.dev Remote Control（`wss://socket.tarkov.dev`）への送信 |
| `remoteid` | ブラウザの Local Storage のコピーから Remote ID を自動検出（Windows のみ） |
| `screenshotstore` | 認識デバッグ用の画像保存と古いスクリーンショットの削除 |
| `screenscale` | 16:10〜16:9 のスクリーンショットを、判定の基準の横幅 2560 px に拡大・縮小する（[recognition.md](recognition.md)） |
| `sound` | 通知音（Windows のみ再生） |
| `taskdetect` | スクリーンショット内のタスク一覧画面の検出と、タスク名領域の切り出し |
| `tracker` | TarkovTracker API クライアント |
| `trackerlog` | EFT ログからのアカウント／プロファイル／モードの検出と履歴 |
| `trackerstore` | TarkovTracker キーとプロファイル割り当ての保存（Windows では DPAPI で保護） |
| `update` | GitHub Releases からの自動アップデート: 最新リリースの取得、semver の比較、チェックサム検証付きのダウンロードと展開、実行中のファイルの差し替え、新しい版での再起動（[設定と連携](settings-and-integrations.md#自動アップデート)） |
| `version` | ビルドに埋め込む版（`task build` が `-ldflags -X` で設定。未設定なら `dev`） |
| `watcher` | スクリーンショットフォルダの監視（`fsnotify`） |

認識処理の詳細は [recognition.md](recognition.md) を参照してください。

### `cmd/` と `tools/`（開発用コマンド）

アプリ本体には含まれない、手元の検証・データ作成用のコマンドです。

| コマンド | 概要と使い方 |
| --- | --- |
| `cmd/analyze` | スクリーンショットにタスク／アイテム検出と OCR・照合をかけて結果を表示。`go run ./cmd/analyze <screenshot>...` |
| `cmd/catalogcheck` | モードごとのカタログを取得して、件数とそこから作るタスクを表示（ネットワークを使う）。`go run ./cmd/catalogcheck [regular\|pve\|pvp-season\|auto]...` |
| `cmd/remotecheck` | 設定の Remote ID（タスク役の先頭ターゲット、なければメインの ID）へ、tarkov.dev Remote Control でタスクコマンドを 1 件送信（ネットワークを使う）。`go run ./cmd/remotecheck <task-slug>` |
| `cmd/ocreval` | 認識デバッグデータ（`Mayak-Debug`）で OCR エンジンを比較評価 |
| `cmd/ocrharvest` | スクリーンショットから、複数エンジンの読みが一致したタイトル画像をラベル付きで収集 |
| `tools/tessbundle` | UB Mannheim 版から同梱用 Tesseract ランタイムを作る（必要な DLL だけコピーし、デバッグ情報を除去） |
| `tools/icon` | 原画 `tools/icon/mark.png`（六角形と M、1024px）を塗り替えて、`build/appicon.png`（トレイと macOS の ICNS の元）、`build/windows/icon.ico`（16〜256px）、`frontend/public/favicon-32.png` と `favicon-256.png`（About のロゴ）、サイトの `mayak-mark.png` と白抜きの `mayak-mark-white.png` を作る。形は原画のまま、装いだけ `go run ./tools/icon`（公式ランチャー風: 角丸の濃いグレーのグラデーション地に、白〜シルバーのグラデーションのマーク。色と角丸は `-bg-top` `-bg-bottom` `-fg-top` `-fg-bottom` `-corner` で変更）。 |
| `tools/release` | `build/bin` をリリース用アーカイブ（`build/dist/Mayak-<os>-<arch>.zip` / `.tar.gz`）に固め、SHA-256 を書く。`build/dist` にインストーラーがあればその SHA-256 も書く（[リリース](#リリース)） |
| `tools/nsis` | Windows のインストーラー（`build/windows/nsis/mayak.nsi`）を `build/dist/Mayak-Setup-<version>-windows-amd64.exe` に組む。`makensis` が無ければ NSIS の配布 zip をチェックサム検証付きで `build/nsis-cache` に一度だけ取得する |
| `tools/ocrtrain` | Tesseract LSTM 学習データの生成（手順は `tools/ocrtrain/README.md` と [ocr-training.md](ocr-training.md)） |
| `tools/capture-window.ps1` | MAYAK のウィンドウを PNG に保存する PowerShell スクリプト |

各コマンドの doc comment にある使い方:

```text
go run ./cmd/ocreval -dir "<Screenshots>/Mayak-Debug" [-tesseract path\to\tesseract.exe] [-tessdata dir] [-lang en]

go run ./cmd/ocrharvest -screens "<Documents>\Escape from Tarkov\Screenshots" \
  -catalog %AppData%\Mayak\catalog\pve.json -tesseract build\bin\tesseract -out <dir>

tessbundle -fetch -out build/bin/tesseract -lang eng,jpn
tessbundle -src <extracted installer> -out build/bin/tesseract [-lang eng,jpn] [-tessdata extra.traineddata...]

ocrtrain -catalog %AppData%\Mayak\catalog\pve.json -fonts <dir with Bender*.otf> -out <dir>
```

`ocrtrain` は `-lang ja` で日本語名を描画し、ラテン文字には `-latin-fonts` の Bender を使います。

### `frontend/`

| パス | 内容 |
| --- | --- |
| `index.html` | メインウィンドウ（ブラウザシェル）。`src/browser/shell.js` を読み込み、設定画面用の `iframe#host-settings` を持つ |
| `settings.html` | 同一オリジンの iframe で開く設定画面。`src/main.tsx`（React）を読み込む |
| `popup.html` | アイテムポップアップのヘッダー。`src/browser/popup.js` を読み込む |
| `src/browser/shell.js` | シェルの描画（タブ、ブックマーク、アイテム欄、メニュー） |
| `src/browser/words.js` | シェル・アイテム欄・ポップアップの文言（[languages.md](languages.md)） |
| `src/browser/shell-core.js` | シェルの共有部分（状態、`action`、`render` の入口、HTML の部品） |
| `src/browser/view-bosses.js` / `view-screenshots.js` / `view-item.js` / `view-tutorial.js` | ボス、スクリーンショット、アイテム欄、チュートリアルの描画とイベント処理 |
| `src/browser/api.js` | バックエンド呼び出しとイベント処理、状態の保存。設定画面用のブリッジ `window.mayakDesktop` を用意する |
| `src/browser/state.js` | タブ・ブックマークなどの状態操作（純粋関数中心） |
| `src/browser/item.js` | アイテム欄の入力検証と価格・履歴の整形 |
| `src/browser/peer-code.js` / `transport.js` | WebRTC 手動ペアリングのコード化と、データチャネルでの送受信 |
| `src/browser/tab-drag.js` | タブのドラッグ並べ替え |
| `src/browser/popup.js` | ポップアップのヘッダー描画 |
| `src/main.tsx` | React の設定画面の `App`: 状態の読み込みと保存、各セクションの描画（shadcn 風 UI、Radix） |
| `src/settings-model.ts` | 設定画面のモデル: Go 側が送る型、既定値、`normalizeStatus`、セクション一覧、マーカーの一覧 |
| `src/TrackerSection.tsx` / `LogsSection.tsx` / `Metric.tsx` | TarkovTracker セクション、ログセクション、ステータスの数値カード。状態は `App` が持ち props で渡す（Radix の Tabs は非表示のセクションをアンマウントするため） |
| `src/Hideout.tsx` / `i18n.ts` / `components/ui` | Hideout の型と文言、設定画面の文言、UI 部品 |
| `src/desktop.ts` | 設定画面から親ドキュメントのブリッジを使うための型付きラッパー。設定画面は Wails ランタイムを自分では初期化しない |
| `bindings/` | `wails3 generate bindings` が生成する TypeScript（手で編集しない。Git には含める） |
| `vite.config.ts` | `index.html`・`settings.html`・`popup.html` の 3 エントリをビルド |

シェルの仕様は [browser-shell.md](browser-shell.md) を参照してください。

### `third_party/go-webview2`

`github.com/wailsapp/go-webview2@v1.0.19` のフォークで、`go.mod` の `replace` で差し替えています。上流のライセンスを保持し、由来は `third_party/go-webview2/MAYAK.md` に記録しています。MAYAK 独自の拡張は次のとおりです（`pkg/edge/browser_tabs.go`、`browser_filter.go`、`chromium.go`）。

- `BrowserIsolation`: ホストオブジェクトもドキュメントスクリプトも登録せず、Web メッセージングを無効にし、リダイレクト後もローカル／カスタムプロトコルを拒否する。ナビゲーションイベント（URL、タイトル、戻る／進む可否、ポップアップ、favicon、読み込み中フラグ `Loading`）を通知する
- `BrowserCommand`: navigate／back／forward／reload／close。reload は DevTools プロトコル `Page.reload`（`ignoreCache`）で行い、失敗時は通常の再読み込み
- `devToolsCall`: 結果を待たずに DevTools プロトコルのメソッドを呼ぶ（UI スレッド上で実行）
- リクエストフィルタ: `FilterAllRequests`、`GetResourceContext`、`Source`、`NavigatingURL`。上流と違い、プロセスを終了せずエラーを返す（タブを閉じる途中でも呼ばれるため）

Wails 本体はフォークせず公式モジュールを使います。

### `build/`

| パス | Git | 内容 |
| --- | --- | --- |
| `config.yml` | 管理 | Wails v3 のアプリ情報と `dev_mode` 設定 |
| `appicon.png`、`windows/`（`icon.ico`、`info.json`、`wails.exe.manifest`） | 管理 | アイコンと Windows リソース |
| `tessdata/eft.traineddata`、`tessdata/eftjpn.traineddata` | 管理 | MAYAK 独自の OCR モデル（英語・日本語） |
| `bin/` | 無視 | ビルド成果物（`Mayak.exe`、`Mayak-dev.exe`、`tesseract/`） |
| `tesseract-cache/` | 無視 | `tessbundle -fetch` のダウンロードと展開先 |
| `ocrtrain-*/` | 無視 | OCR 学習の作業ディレクトリ |
| `pending/` | 無視 | 手元の作業用の置き場 |
| `mayak-*.png`、`devtools*.png` など | 無視 | 手元のキャプチャ |

### その他

- `client/`: 以前の Electron クライアントの残骸（無視対象の依存・ビルド出力のみ）。現在のアプリにもビルドにも関係しません
- `docs/`: この仕様書（Markdown のみ。目次は `SUMMARY.md`）と英語の開発メモ。公開はせず、開発時と AI の参照用に置いている
- `site/`: ランディングページ（Vite + React + TypeScript + Tailwind + shadcn/ui + jotai）と Cloudflare の設定

## Task

タスクはすべて `Taskfile.yml` にあります（`build/` 配下に追加の Taskfile はありません）。

| タスク | 内容 |
| --- | --- |
| `task bindings` | `wails3 generate bindings -ts -d frontend/bindings` |
| `task frontend:install` | `bun install --frozen-lockfile` |
| `task frontend:build` | バインディング生成 → 依存インストール → `bun --bun run build`（`tsc && vite build`） |
| `task tesseract:bundle` | `build/bin/tesseract` に同梱 Tesseract を作る（Windows のみ）。固定版の UB Mannheim インストーラーをチェックサム検証付きで一度だけ取得し、`eng,jpn` と `build/tessdata` のモデルを入れる。入力が変わらなければスキップ |
| `task build` | `frontend:build` → `tesseract:bundle` → `notices` → `build:{{OS}}`。アプリは起動しない |
| `task notices` | `build/bin/THIRD_PARTY_NOTICES.txt` を生成する。exe にリンクされる Go モジュールとフロントエンドの依存のライセンス文、同梱の OCR データ、実行時に取得するフィルタリストの出典（`tools/notices`） |
| `task build:windows` | `.syso` 生成と `go build`。`DEV=true` でなければ `-tags production -ldflags="-s -w -H windowsgui"` で `build/bin/Mayak.exe` を出力。版（`VERSION`、既定は `git describe --tags`）を `internal/version` に埋め込む |
| `task build:darwin` / `build:linux` | `production` タグ付きで `build/bin/Mayak` を出力（版の埋め込みは同じ） |
| `task installer` | Windows のインストーラーを `build/dist` に組む（`tools/nsis`。`task build` の後に実行） |
| `task release:archive` | `build/bin` をリリース用アーカイブと SHA-256 にする（`build/dist/`、`tools/release`）。`TARGET_ARCH=amd64` で CPU を指定 |
| `task dev:web` | ランディングページ（`site/`、Vite + React）をローカルの Web サーバー（http://localhost:5173）でホットリロード付きで動かす |
| `task site:build` | ランディングページを `site/dist` にビルドする |
| `task site:deploy` | ランディングページをビルドして Cloudflare に公開する（`wrangler deploy`、設定は `site/wrangler.jsonc`） |
| `task dev` | 依存インストール → ホットリロード付き開発モード（下記）。アプリを起動する |
| `task build:dev` | `dev` が使う開発ビルド。バインディング生成、Tesseract 同梱、`production` タグなしで `build/bin/Mayak-dev.exe` |
| `task check:offline` | アプリを開かない検証。全パッケージのコンパイル、`TestBrowser*`、bun のテスト |
| `task run` | ビルド済みアプリを起動（明示的に頼まれたときだけ使う） |

`go build` はすべて `-p 2 -buildvcs=false` で実行します。

### `task dev` の仕組み

`task dev` は `frontend:install` のあと `wails3 dev -config ./build/config.yml -port 9245` を実行します。`build/config.yml` の `dev_mode` に従って次の 3 つが動きます。

1. **Vite 開発サーバー**（background）: `bun --bun frontend/node_modules/vite/bin/vite.js frontend --host 127.0.0.1 --port 9245 --strictPort`
2. **開発ビルド**（blocking）: `wails3 task build:dev`
3. **アプリ本体**（primary）: `build\bin\Mayak-dev.exe`

- `production` タグなしのビルドでは、アセットは埋め込みの `frontend/dist` ではなく Vite 開発サーバーから配信されます。
- WebUI（`frontend/`）の変更は Vite の HMR／リロードで反映されます。そのため `frontend` は Wails の監視対象から外しています。
- `public/` の静的ファイル（`frontend/public/*.svg` など）は Vite が開いているページに差し込み直さないので、参照する URL に版（`?v=2`）を付けるか、Go ファイルを書き換えて再起動させます。
- **UI が固まって見えるとき**: Process Lasso の ProBalance のような優先度管理ツールがあると、起動直後の CPU 使用（カタログの読み込み、フィルタリストの解析）で MAYAK が BelowNormal に下げられ、そのあいだに生まれた WebView2 のブラウザプロセスが低い優先度を引き継いだまま戻りません（ツールが戻すのは本体だけ）。`task dev` は再ビルドのたびに起動をやり直すので毎回起きます。設定 → 起動とウィンドウ →「ウィンドウの優先度を通常に保つ」（`keepPriority`）をオンにすると、MAYAK が `guardPriority`（`priority_windows.go`）で起動 3 秒後と以後 10 秒ごとに、自分と直下の `msedgewebview2.exe` が Idle / BelowNormal なら Normal に戻します（そうしたツールがある環境だけの話なので既定はオフ）。それでも重いときは、ツール側で `Mayak.exe` と `Mayak-dev.exe` を除外してください。
- Go（`*.go`）の変更は、1000 ms のデバウンス後にバインディング生成と開発ビルドをやり直し、アプリを再起動します。一時ファイルに書いてから置き換えるアトミック保存では、Windows の監視が置き換え（名前変更）を再読み込みの対象にしないため、一時ファイル（`*.go.tmp.*`、GoLand の `*.go___jb_tmp___`）への書き込みでも開発ビルドを始めます。監視しないディレクトリは `.git`、`node_modules`、`frontend`、`client`、`third_party`、`build`、`docs`、`tools` です。`.gitignore` の対象も監視しません。

開発ループの注意点（`build/config.yml` のコメントより）:

- Vite は `127.0.0.1` に明示的にバインドします。Wails のアセットプロキシは `127.0.0.1` に接続しますが、`localhost` は `::1` だけに解決されることがあるためです。
- ポート `9245` は `task dev` の `-port` と Vite の `--port` で一致させています。`--strictPort` なので、ポートが使用中なら別ポートへ逃げずに失敗します。
- Vite は開発ビルドより先に起動し、アプリが開発サーバーを見つけられるようにしています。
- primary コマンドは Windows では `cmd.exe` 経由で実行されるため、パスはバックスラッシュで書きます。
- `third_party/` の変更は監視されません。go-webview2 フォークを変更したら `task dev` を再起動します。
- `task dev`、`task run`、`wails3 dev` はアプリを起動します。下記の制約に注意してください。

## リリース

配布物は GitHub Releases です。アプリはそこから自分自身を更新します（[設定と連携](settings-and-integrations.md#自動アップデート)）。

1. `build/config.yml` と `build/windows/info.json` の版を上げてコミットします。
2. `CHANGELOG.md` と `CHANGELOG.en.md` の「開発中」を `## v1.2.3 (日付)` に改名して、使う人向けの変更点を整えます（公開ページ https://mayak.ich.sh/changelog とアプリの「変更点」がここを表示します。サイトは `task site:deploy` で反映）。
3. `v1.2.3` の形のタグを打って push します: `git tag -a v1.2.3 -m "MAYAK 1.2.3" && git push origin v1.2.3`
4. `.github/workflows/release.yml` が Windows（amd64）、macOS（arm64、amd64）、Linux（amd64）で `task build VERSION=v1.2.3` と `task release:archive` を実行し（Windows では間に `task installer`。macOS では `release:archive` が `Mayak.app` を組んで ad-hoc 署名し、`hdiutil` で `Mayak-darwin-<arch>.dmg` も作る）、`SHA256SUMS.txt` を付けてリリースを公開します。リリースノートは GitHub が自動生成します。

アーカイブの名前（`Mayak-0.1.6-windows-amd64.zip`、`Mayak-0.1.6-darwin-arm64.tar.gz`、`Mayak-0.1.6-darwin-amd64.tar.gz`、`Mayak-0.1.6-linux-amd64.tar.gz`。バージョンは `internal/update` の `ArchiveName` が付け、更新側は `IsArchive` でバージョンを問わず OS と CPU の接尾辞で選ぶ）と `SHA256SUMS.txt` は `internal/update` が探すものなので変えないでください。Windows のアーカイブには `Mayak.exe`、`tesseract/`、`THIRD_PARTY_NOTICES.txt` が、ほかには `Mayak` と `THIRD_PARTY_NOTICES.txt` がルートに入ります。プレリリース（`prerelease` にチェック）とドラフトは「latest」に含まれないため、自動アップデートの対象になりません。

### Windows インストーラー

`Mayak-Setup-<version>-windows-amd64.exe`（`build/windows/nsis/mayak.nsi`、NSIS 3）はランディングページが案内する主な配布物で、zip はポータブル版兼自動アップデート用です。インストーラーの動きは次のとおりです。

- ユーザー単位（`%LOCALAPPDATA%\Programs\MAYAK`）にインストールし、管理者権限を求めません。このフォルダなら `internal/update` の差し替えがそのまま動きます。
- 起動中の MAYAK は単一起動のミューテックス（`com.ichi0g0y.mayak-sim`）で検出し、終了を求めます。それでも残っていた `Mayak.exe` は上書きではなく `.mayak-old` に改名してから置きます（アプリが次回起動時に消します）。
- スタートメニューにショートカットを作り、完了ページで「MAYAK を起動する」「デスクトップにショートカットを作成する」を選べます。`設定 > アプリ` からアンインストールできます（`HKCU\...\Uninstall\MAYAK`）。
- WebView2 ランタイムはレジストリで確認するだけで同梱しません（Windows 11 とほとんどの Windows 10 にあります）。無ければ入手先を案内します。
- アンインストール時は自動起動の登録（`HKCU\...\Run\Mayak`）を消し、`%APPDATA%\Mayak` の設定は残すか尋ねます（サイレント時は残す）。
- 表示言語は Windows の言語に従い、日本語と英語を用意しています。
- 署名はありません。初回実行時に SmartScreen の警告が出ますが、インストール後の `Mayak.exe` にはインターネット由来の印が付かないため、以後の起動で警告は出ません。

サイレント実行: `Mayak-Setup-0.1.6-windows-amd64.exe /S /D=C:\path`（`/D` は最後、引用符なし）、アンインストールは `uninstall.exe /S`。

macOS／Linux のビルドは CI でコンパイルしているだけで、動作は検証していません（[プラットフォーム](#プラットフォーム)）。

### ランディングページ

`site/` は Vite + React + TypeScript のページで、Tailwind v4 と shadcn/ui（`src/components/ui`）、状態は jotai（`src/state.ts`: 表示言語と最新リリース）で作っています。「はじめかた」の各手順の右に出す実機のスクリーンショットは `site/public/screenshots/` に置き、`src/sections/GettingStarted.tsx` の `stepImages` で手順ごとに列挙します（`setup-5.png`、`setup-5-2.png` のように複数可。無いファイルは表示しない）。アプリの表示言語に依存する画像は `ja/` と `en/` に分け（日本語以外のページ言語は `en/` を使う。`src/lib/screenshots.ts`）、インストーラー・SmartScreen・EFT の設定画面のように言語に依らないものは直下に置きます。TarkovTracker の節は `<lang>/tracker.png` です。文言は `src/i18n/`（`ja.ts` が型の基準、`en.ts`、`ru.ts`、`de.ts`、`zh.ts`）に 5 言語で持ち、初回はブラウザの `navigator.languages` から選び、ヘッダーのドロップダウンで切り替えて `localStorage` に保存します（`<html lang>` も追従）。ダウンロードボタンは GitHub の latest リリース（`releases/latest` API）から版と各 OS のアーカイブの URL・サイズを取り、取れないときはリリース一覧へのリンクになります。

Cloudflare には Workers の静的アセット（`site/wrangler.jsonc`、Worker 名 `mayak`。同じ Worker の `site/worker/index.js` が `/api/pair` のペアリング中継も受け持ちます）として公開し、独自ドメイン https://mayak.ich.sh（`routes` の `custom_domain`。ゾーン `ich.sh` は同じアカウント）と mayak.ichi0g0y.workers.dev で配信します。初回だけ `wrangler login` でサインインし（CI なら `CLOUDFLARE_API_TOKEN` と `CLOUDFLARE_ACCOUNT_ID`）、あとは `task site:deploy`（`bun run build` → `wrangler deploy`）です。wrangler はリポジトリ内で使ってください（Node はプロジェクトの `mise.toml` でだけ有効です）。仕様書（`docs/`）は公開しません。

## テスト

| コマンド | 対象 |
| --- | --- |
| `go test ./...` | Go の全ユニットテスト |
| `go test -run '^$' ./...` | コンパイルのみ |
| `go test -run '^TestBrowser' ./internal/app` | ブラウザ状態の一時ファイル保存、特権ナビゲーション・ID の拒否、内蔵ブラウザ用 Remote ID の生成と送信先 |
| `bun test ./frontend/src/browser/state.test.js ./frontend/src/browser/item.test.js ./frontend/src/browser/words.test.js` | ブラウザシェルの純粋ロジック |
| `task check:offline` | 上記のコンパイル、`TestBrowser*`、bun のテストをまとめて実行 |

主なテスト内容:

- **ルートパッケージ**: 古い解析結果のリモート送信を捨てる処理、マップ設定の移行、設定と TarkovTracker 割り当ての保存、ウィンドウ位置の復元（負の座標、切断されたディスプレイ、最大化中のモニター移動）、タイトルバー色、タスクページ URL、アイテム検索（日本語名を含む）、favicon キャッシュ、ハイドアウト通知、Remote ID
- **`internal/*`**: 検出器（`taskdetect`、`itemdetect`、`logdetect`、`hideoutlog`、`trackerlog`）、検出器と OCR 前処理が共有する画素アクセス・輝度・切り出し・data URL（`imaging`。`image.Image.At` はピクセルごとにインターフェース呼び出しと色変換が入るので、RGBA のバイト列を直接読む）、スクリーンショットの拡大・縮小（`screenscale`）、OCR 前処理（`ocr`）、照合（`questmatch`、`itemmatch`）、カタログ、設定、保存（`trackerstore`、`screenshotstore`、`applog`）、座標解析、広告ブロック、サウンド波形など。HTTP を使うものは `httptest` のローカルサーバーを使います
- **bun**: `state.test.js`（ブックマークの統合、タスク／マップタブの再利用とピン留め、状態の復元、URL 検証、ペアリングコード、STUN 設定の検証など 25 件）、`item.test.js`（アイテム情報の検証、最良の売却先、価格と経過時間の整形、履歴グラフなど 6 件）

環境変数で有効にするテスト（既定ではスキップ）:

- `MAYAK_EFT_LOGS=<EFT のログフォルダ>`: `hideoutlog` の実ログ検証
- `MAYAK_LIVE_CATALOG=1`: `catalog` の公開 API 確認（ネットワークを使う）

起動処理のプロファイル: `MAYAK_CPUPROFILE=<出力ファイル>` を付けてアプリ（`build\bin\Mayak-dev.exe` など）を起動すると、起動から 30 秒の CPU プロファイルを書きます（`run.go`、`runtime/pprof`）。`go tool pprof -top build\bin\Mayak-dev.exe <出力ファイル>` で内訳を見られます。単一インスタンスなので、動いている MAYAK を止めてから起動してください。

## アーキテクチャ上の制約

詳細は [architecture-constraints.md](architecture-constraints.md)、[wails-browser.md](wails-browser.md)、[wails-v3-migration.md](wails-v3-migration.md)（英語）を参照してください。要点は次のとおりです。

- **Go + Wails を維持する**: Electron や Electron ベースのクライアントは導入しません（ユーザーが明確に却下しています）。旧 Electron ソースはリポジトリ外へ退避済みで、`client/` の残骸は使いません。
- **Wails v3 はフォークしない**: `v3.0.0-beta.24` の公式モジュールを使います。ネイティブのタブ実装は `internal/browserview` に置き、v3 の公開 API（ネイティブウィンドウ、UI スレッドへのディスパッチ）だけを使います。go-webview2 の小さな拡張だけを `third_party` に置きます。
- **外部サイトは分離する**: 外部サイトは同じウィンドウ内の独立したネイティブビューに表示します。Windows では別のブラウザデータディレクトリを使い、Wails のスクリプトは注入せず、Web メッセージングを無効にし、権限要求は拒否します。外部ページは Go バインディングに触れません。同一オリジンの iframe を使うのは、同梱の信頼できる `settings.html` だけです。
- **ナビゲーションの制限**: 許可するのは HTTP/HTTPS だけで、認証情報付き URL と Wails 自身のホストは除外します。リダイレクトもネイティブ側で再チェックします。ポップアップは受け入れた場合もタブになり、Wails バインディング付きのドキュメントにはなりません。
- **ブリッジを作らない**: ローカル HTTP／WebSocket ブリッジやヘルパープロセスは使いません。WebRTC（`RTCPeerConnection`）は信頼できるメイン文書が持ち、受信側はタスク／マップ表示のメッセージだけを受け付けます。TURN は使わず、STUN は経路探索だけに使います。
- **保存**: ブラウザ状態は `Mayak/browser.json` に一時ファイル経由で置き換え保存します。トークンや SDP は保存しません。設定は即時に保存します。
- **既存の改善を残す**: 日本語 OCR／カタログの改善、タスクサイトの選択、設定の即時保存を維持します。
- **ユーザーはプレイ中に開発を進めている**: アプリの起動・再起動、ウィンドウ操作、ライブ OCR、ネットワーク接続テストは、実際のテストを明示的に頼まれたときだけ行います。それ以外は静的チェック、コンパイル、ネットワークを使わない分離されたユニットテスト（`task check:offline` など）で確認します。後片付けのために実行中のプロセスを止めてはいけません。

## プラットフォーム

Windows が主な対象です。macOS／Linux 向けのコードもありますが、コンパイルも実行も検証していません。

| 領域 | Windows | macOS / Linux |
| --- | --- | --- |
| ネイティブタブ（`internal/browserview`） | WebView2（`view_windows.go`、`filter_windows.go`） | WKWebView（`view_darwin.go`）、GTK4/WebKitGTK 6.0（`view_linux.go`、`-tags gtk3` で GTK3/WebKitGTK 4.1）。広告ブロックのフィルタは Windows のみ |
| ポップアップの所有関係 | `app_popup_windows.go` でメインウィンドウをオーナーにする | `app_popup_other.go` は何もしない |
| タイトルバーの色 | DWM（`app_window_theme_windows.go`） | 何もしない |
| スタートアップ登録（`autostart`） | 対応 | 有効化するとエラー |
| Remote ID 自動検出（`remoteid`） | 対応 | エラーを返す |
| 通知音（`sound`） | 再生 | ファイル検証のみ |
| TarkovTracker キーの保護（`trackerstore`） | DPAPI | 平文 |
| 動作モード | ホスト（監視・認識）とクライアント | 受信専用のクライアントとして起動 |

OS ごとのファイルはファイル名の接尾辞（`_windows.go`／`_darwin.go`／`_linux.go`）か `//go:build !windows`（`_other.go`）で分けます。

## コーディング規約

- **コード、コメント、コミットメッセージは英語**で書きます。ドキュメント（`docs/` の仕様章）は日本語です。
- **コメントは「なぜ」を書きます**。何をしているかではなく、理由や前提、はまりどころを残します。例: `main.go` の「`ServiceStartup` はネイティブウィンドウ生成中に走ることがあるので、初期位置は生成前のオプションで渡す」、`build/config.yml` の IPv4 バインドの理由、`browser_tabs.go` の「通常のリロードは壊れたキャッシュを使い続けるので `ignoreCache` で再読み込みする」。
- 公開する型・関数には、名前で始まる doc comment を付けます。コマンドの doc comment には使い方の行を入れます。
- コミットメッセージは、変更内容を平叙文で表す 1 行の件名（例: `Match titles past short scraps read before them`）と、必要なら理由を説明する本文にします。
- `frontend/bindings` は生成物です。Go の公開メソッドを変えたら `task bindings` で再生成します（`task build`／`task dev` でも生成されます）。
