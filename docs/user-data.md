# ユーザーデータの保存

MAYAK が `%AppData%\Mayak`（`internal/appdir`）に保存するデータの置き場所と形式です。将来クラウド同期（有料になる可能性あり）を入れても困らないよう、データを次の 3 つに分けて持ちます。

- **ユーザーデータ**: 別の PC でも同じであってほしいもの。同期の対象です。
- **この PC のデータ**: フォルダの場所、ウィンドウの位置、接続の ID など、PC ごとに違うもの。同期しません。
- **キャッシュ**: ログやネットから作り直せるもの。同期しません。

新しくデータを保存するときも、まずこのどれに当たるかを決めてください。

## 同期できる形

ユーザーデータは、2 台で別々に変えても混ぜ合わせられる形で持ちます（`internal/userdata`）。

- **設定（項目ごと）**: `userdata.Keyed`。値と、その項目が最後に変わった日時（`updatedAt`）を項目ごとに持ちます。混ぜるときは項目ごとに新しい方を取ります（`Merge`）。値が変わったときだけ日時が進みます。
- **一覧（1 件ずつ）**: `userdata.Records`。1 件ごとに `id`・並び順（`order`）・更新日時・削除済みの印（`deleted`）を持ちます。消した件は中身を捨てた「削除済み」の記録を半年残すので、古いコピーと混ぜても消したものが戻りません。
- **絶対パスを入れない**: 別の PC では意味がないため。ファイルはデータフォルダに取り込んで、その中の名前で指します。
- **秘密は同期しない**: TarkovTracker のトークンは PC ごとに暗号化しているため、そのままでは同期しません。

どのファイルも一時ファイルに書いてから置き換え、直前の版を `.bak` に残します（`userdata.WriteWithBackup`）。

## ファイル一覧

| ファイル | 分類 | 形式・内容 |
|---|---|---|
| `preferences.json` | ユーザーデータ | Host 設定の好み（言語、通知の声・音量、自動更新など）。`Keyed` |
| `settings.json` | この PC | Host 設定のうち PC に属するもの（`config.deviceKeys`: スクショ・ログのフォルダ、Tesseract、OCR エンジン、tarkov.dev 連携の ID、ブラウザのリモート ID、自動起動、優先度、古いウィンドウ位置） |
| `browser-preferences.json` | ユーザーデータ | ブラウザシェルの好み（テーマ、時計、レイアウト、ツールの並び、分隊の表示名 `squadName` など。`app_browser.go` の `browserPreferenceKeys`）。`Keyed` |
| `bookmarks.json` | ユーザーデータ | ブックマーク。`Records`（値はシェルのブックマーク `{id, name, url, group, sidebar}`） |
| `browser.json` | この PC | ブラウザシェルのそれ以外（タブ、パネルの大きさ、アイテム欄、Host／クライアントの役割、ファビコンの対応表、入っている分隊のコード `squadCode`） |
| `snapnotes/<id>/` | ユーザーデータ | スナップノート。`note.json`（`createdAt`・`updatedAt`・`changedAt`、削除すると `deleted` の記録だけ残る）、`base.png`、`thumb.jpg` |
| `sounds/<hash>-<名前>` | ユーザーデータ | 通知のカスタム音声。選んだファイルを取り込んだもの。設定はこの名前（`sounds/…`）で指す |
| `tracker-tokens.dat` | この PC（秘密） | TarkovTracker のキーとプロフィール。DPAPI で暗号化 |
| `window.json`・`popup.json` | この PC | ウィンドウの位置と大きさ |
| `processed-screenshot.json` | この PC | 最後に処理したスクショ（同じものを二度処理しないため） |
| `screenshots.json` | この PC | この PC のスクショ フォルダの解析結果 |
| `hideout/events.json` | この PC | ゲームのログから読んだハイドアウトの記録 |
| `catalog/`・`thumbs/`・`favicons/`・`adblock/`・`updates/`・`maps/` | キャッシュ | tarkov.dev のデータ、サムネイル、アイコン、フィルター、更新ファイル、分隊マップの地図（`maps.json` と SVG） |
| `browser-webdata/` | この PC | ページタブの WebView2 プロファイル（ログイン情報など） |
| `mayak.log` | この PC | アプリのログ |

## 読み書きの流れ

- **Host 設定**: アプリの中では今まで通り 1 つの `config.Settings` です。`config.Load` が `settings.json` と `preferences.json` を合わせて読み、`config.Save` が 2 つに分けて書きます（変わった項目だけ `updatedAt` を進める）。
- **ブラウザシェル**: シェル（`state.js`）からは今まで通り 1 つのオブジェクトです。`BrowserSave` が好み・ブックマーク・残りの 3 つに分けて書き、`BrowserLoad` が合わせて返します。ブックマークはシェルが送ってくる一覧との差分から、変わった件の日時を進め、消えた件を削除済みにします。
- **スナップノート**: `changedAt` は何かが変わるたびに（星や削除も）進みます。一覧の並びに使う `updatedAt` は、描画とタイトルが変わったときだけ進みます。
- **カスタム音声**: 「ファイル選択」で選んだファイルを `sounds/` にコピーし、設定にはその名前を入れます（`app_sound_files.go`）。

## 以前のデータからの移行

どれも起動時や次の保存のときに自動で移ります。古いファイルもそのまま読めます。

- 分ける前の `settings.json`（全部入り）は、そのまま読めます。次に保存したとき、好みが `preferences.json` に移ります。
- 分ける前の `browser.json` も同じで、次の保存で好みとブックマークが別のファイルに移ります。
- 絶対パスで選んでいたカスタム音声は、起動時に `sounds/` に取り込みます。ファイルが見つからないものはパスのまま残ります（再生時は声かビープ音になる）。
