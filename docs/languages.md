# 多言語対応

MAYAK が扱う「言語」は2種類あり、別々に管理しています。

| 種類 | 何のためか | 今の対応 |
|---|---|---|
| ゲームの言語 | スクリーンショットの文字を読み、名前を照合する。アイテム欄に、その言語の名前を出す | 英語、日本語 |
| 表示の言語 | MAYAK 自身の画面（ボタンや設定など）の文言 | 英語、日本語 |

ゲームの言語は、名前のデータ（tarkov.dev）と OCR のモデルがあれば足せます。表示の言語は、画面の文言をすべて翻訳する必要があるので手間がかかります。

## ゲームの言語

### 名前のデータ

tarkov.dev は、名前を言語ごとの翻訳ファイルで公開しています。中身はゲーム内のテキストがもとなので、表記はゲーム画面と同じです。

| リソース | 内容 |
|---|---|
| `items_<言語>` | アイテム名と略称（例: `items_ja`） |
| `tasks_<言語>` | タスク名（例: `tasks_ja`） |

英語以外の言語は `internal/locale/locale.go` の `Languages` に並べます（今は `ja` だけ）。ここに並べた言語について、次のことが自動で行われます。

- **カタログの取得:** `items_<言語>` と `tasks_<言語>` を、ほかのリソースと一緒に5分ごとに確認します。取れなくても、英語だけで動き続けます（[ゲームデータ](catalog.md)）。
- **照合:** その言語の名前を、アイテムとタスクの別名に加えます。その言語のゲーム画面から読んだ名前でも照合できます（[スクリーンショット認識](recognition.md)）。
- **アイテム欄:** アイテムとタスクに、言語ごとの名前（`names`）を付けます。表示の言語と同じ言語の名前があれば、それを表示します（[アイテム欄](item-panel.md)）。
- **アイテム検索:** その言語の名前でも探せます。

照合の結果はアイテムやタスクの ID で決まるので、どの言語で読み取っても、開くページは設定したサイト（英語 Wiki など）になります。

### OCR

`internal/ocr/language.go` の表に、言語ごとの OCR の設定をまとめています。

| 言語 | Windows OCR | Tesseract の元モデル | MAYAK の調整済みモデル |
|---|---|---|---|
| `en` | `en-US` | `eng` | `eft` |
| `ja` | `ja-JP` | `jpn` | `eftjpn` |

- 調整済みモデルが同梱されていれば、元モデルの代わりに使います。
- 英語以外の言語では、英語のモデルも一緒に使います（`eftjpn+eft` など）。名前には英字や数字が混ざるためです。

### ゲームの言語を知る方法

- 設定の「ゲームの表示言語」（`GameLanguage`）は `auto`、`en`、`internal/locale` の `Languages` にある言語のどれかです。それ以外の値は `auto` に直します（`app.go` の `normalizeSettings`）。
- `auto` のときは、EFT の設定ファイル（`%AppData%\Battlestate Games\Escape from Tarkov\Settings\Game.ini`）から読みます（`internal/eftdetect` の `GameLanguage`、`app.go` の `ocrLanguage`）。
- 今は `ja`（または `jp`）を日本語、英字を使う言語（`en`、`es`、`fr`、`ge`／`de`、`it`、`pl` など）を英語として扱います。それ以外の言語（ロシア語や中国語など）と、設定ファイルが読めないときは、Windows の言語設定に合わせます（Windows OCR は `auto`、Tesseract は英語のモデル）。

## 言語を足す手順

例として、ドイツ語（`de`）を足す場合です。

1. **名前のデータ:** `internal/locale/locale.go` の `Languages` に `"de"` を足す。tarkov.dev に `items_de` と `tasks_de` があることを先に確かめてください。これで設定の `GameLanguage` にも `de` を保存できるようになります（`normalizeSettings` と `ocrLanguage` は `Languages` を見ます）。
2. **ゲームの言語の判定:**
   - `internal/eftdetect/detect.go` の `gameLanguage` で、EFT の設定ファイルの言語コード（ドイツ語は `ge` または `de`）を `de` として返すようにする。今は `ge` と `de` が英語（`en`）の行に入っているので、そこから外す。
   - 設定画面の「ゲームの表示言語」の選択肢は、Go の `GameLanguages()`（`en` と `internal/locale` の `Languages`）から自動で並びます。`frontend/src/main.tsx` の `languageNames` に表示名（例: `de:'Deutsch'`）を足すだけです（足さなければコードのまま表示されます）。
3. **OCR:**
   - `internal/ocr/language.go` の表に、Windows OCR の言語タグ（`de-DE`）、Tesseract の元モデル（`deu`）、調整済みモデルの名前（例: `eftdeu`）を足す。
   - Tesseract の元モデルを同梱する（`Taskfile.yml` の `tesseract:bundle` の `-lang` に足す）。
   - Windows OCR でその言語を読むには、使う PC にその言語の Windows OCR 言語パックが入っている必要があります（ないとエラーになります）。
4. **（任意）モデルの調整:** 読み間違いが多ければ、[OCR モデルの学習](ocr-training.md)の手順で `-lang de` の学習データを作り、調整済みモデルを作る。作ったモデル（`build/tessdata/eftdeu.traineddata`）は、`Taskfile.yml` の `tesseract:bundle` の `-tessdata` に足して同梱する。
5. **（文字によっては）照合の正規化:** `internal/questmatch/match.go` には、日本語向けの処理（Windows OCR が入れる字間の空白を取り除く、など）があります。英字以外の文字を使う言語では、同じような処理が要るか確かめる。
6. **確認:** その言語のゲーム画面のスクリーンショットで、`cmd/ocrharvest -lang de` を回して照合できるか確かめる。

ここまでで、その言語のゲームのスクリーンショットを読めるようになります。アイテム欄にその言語の名前を出すには、次の「表示の言語」も必要です。

## 表示の言語

MAYAK の画面の文言は、次の場所にあります。

| 場所 | 内容 |
|---|---|
| `frontend/src/browser/shell.js` の `words` | ブラウザシェルとアイテム欄の文言 |
| `frontend/src/i18n.ts` | 設定画面（Host settings）の文言 |
| `frontend/src/browser/state.js` | 保存された言語の検証（今は `ja` と `en`） |
| `app.go` の `normalizeSettings` | 設定の `Language` の検証（今は `ja` と `en`） |
| `frontend/src/main.tsx` | ブラウザシェルの言語（`<html lang>`）を Host 設定へ移す処理（今は `ja` と `en`） |
| `frontend/src/browser/item.js`、`frontend/src/browser/api.js` | 数値・相対時間の書式と、一部のエラー文 |
| `tray.go` | タスクトレイのメニューの文言 |

表示の言語を足すには、これらすべてに訳文と選択肢を足します。アイテム欄の名前は、表示の言語と同じ言語の名前（`names`）があればそれを出し、なければ英語名を出します。
