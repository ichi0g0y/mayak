# スクリーンショット認識

MAYAK は EFT のスクリーンショットキーでゲーム自身が保存した画像を監視し、画面の種類を判定して OCR でタイトルを読み取り、カタログと照合します。この章は現在のコードに基づいてその流れをまとめたものです。

## 設計方針: ゲームが書き出したファイルだけを読む

- ゲームプロセスには一切触れません(メモリ読み取り、画面キャプチャ、入力フック、オーバーレイはありません)。入力はゲームが自分で書き出すファイルだけです。
  - `Screenshots` フォルダの画像(EFT のスクリーンショットキーで保存される)
  - `Logs` フォルダのログ(レイド状態やマップの判定に使う。`internal/logdetect`)
  - ゲーム設定 `%AppData%\Battlestate Games\Escape from Tarkov\Settings\Game.ini`(表示言語のみ。`eftdetect.GameLanguage`)
  - ランチャー設定 `%AppData%\Battlestate Games\BsgLauncher\settings` の `gamesRootDir`(フォルダの自動検出のみ。`eftdetect.Detect`)
- 解析はプレイヤーが撮ったスクリーンショットに対してだけ行うので、ゲームとのやり取りはすべてプレイヤーの操作が起点になります。アンチチートの観点でも、外部ツールがゲームを観察していると見なされる経路を持ちません。
- 画面認識も OCR もローカルで完結します(同梱 Tesseract または Windows OCR)。ネットワークを使うのはカタログ取得とブラウザ連携だけです([catalog.md](catalog.md)、[settings-and-integrations.md](settings-and-integrations.md))。

## 全体の流れ

```text
Screenshots/*.png|jpg ─▶ watcher ─▶ processScreenshot(sequence++、前の解析を cancel)
   ├─ ファイル名に座標なし ─▶ handleTaskScreenshot
   │      itemdetect ─(IsItem)─▶ handleItemAnalysis
   │      └─ taskdetect ─▶ handleTaskAnalysis
   └─ ファイル名に座標あり ─▶ handleCoordinateScreenshot
          itemdetect ─(IsItem)─▶ handleItemAnalysis
          taskdetect(IsTasks かつ照合 ≥ .78)─▶ handleTaskAnalysis
          それ以外 ─▶ レイド中なら位置情報(position)
   └─ 完了後 afterScreenshotAnalysis: デバッグ保存・自動削除
```

分析関数はすべて `internal/app/app.go` にあり、後処理は同じパッケージの `app_screenshots.go` にあります。

## スクリーンショットの監視 (`internal/watcher`)

- `StartMonitoring` が設定の `ScreenshotDirectory` を `watcher.New` で監視します(Windows Host モードのみ。ログ監視 `startLogDetector` も同時に開始)。フォルダは `eftdetect.Detect` が自動検出します(`Documents\Escape from Tarkov\Screenshots`、OneDrive 配下、ランチャーの `gamesRootDir`、各ドライブの `Battlestate Games` / `Games` 配下)。
- 対象拡張子は `.png` / `.jpg` / `.jpeg` です。
- fsnotify の `Create` / `Rename` / `Write` を受けます。Windows では通知がまとめられたり欠けたりするので、1 秒ごとのポーリングで「最新ファイル 1 件」も確認します(更新から 2 分以内のもののみ)。
- 起動時には、更新から 15 分以内の最新スクリーンショットを 1 件だけ再生します。解析中にアプリを再起動しても画像を取りこぼさないためです。
- 書き込み完了待ち: 150 ms 間隔で最大 20 回 `Stat` し、サイズが 0 より大きく前回と同じで、かつ開ける状態になったらコールバックします。最後まで読めなかったファイルは既読マークを外し、ポーリングで再試行できるようにします。
- 同じパスは `seen` マップで 1 回だけ処理します。さらに `processScreenshot` がパス・サイズ・更新時刻の指紋(`config.FingerprintScreenshot`)を永続化し、前回処理した画像は再起動後もスキップします(手動の「最新を解析」`AnalyzeLatestScreenshot` は `force` で再解析)。

## 解析の順序付けとキャンセル

- `processScreenshot` は新しい画像ごとに `analysisSequence` を 1 増やし、1 つ前の解析の `context` をキャンセルします。
- 各分析関数は状態を書き込む前に「`analysisSequence` が自分の番号のままか、`status.LastScreenshot` が自分のパスか」を確認し、古ければ何もせずに戻ります。連写しても表示されるのは最新の画像の結果だけです。
- リモート送信も `runRemoteIfCurrent` で同じ確認をしてから行います。
- 状態は `status:update` イベントでフロントエンドに送られ、`AnalysisStage` に進行状況(例: 「画像を分類中」「アイテム詳細を検出・OCR実行中」「解析完了」「解析エラー」)が入ります。

## 分類の順序

画像はつねに次の順で判定します(`handleCoordinateScreenshot` / `handleTaskScreenshot`)。

1. **アイテム詳細ウィンドウ / フリマの出品作成ウィンドウ** (`itemdetect`): 開いているウィンドウの情報を、その背後の画面より優先します。
2. **タスク一覧** (`taskdetect`)
3. **位置情報**: ファイル名に座標が含まれる場合だけ(`position.ParseFilename`)。

座標付きのファイル名は、EFT がメニュー画面にも古い座標を付けることがあるため、必ず先に画像を調べます。タスク一覧らしく見えても照合が .78 未満なら採用せず(`Rejected Tasks-like raid image`)、位置情報として扱います。位置情報を使うのはログがレイド中(`logdetect.RaidState`)を示す場合だけで、マップは `logdetect.LatestMap` で更新します。レイド外なら「レイド外の座標メタデータを無視しました」として捨てます。

### アイテム詳細ウィンドウの検出 (`internal/itemdetect`)

- 解像度はスクリーンショットの大きさから自動で判定します（設定はありません）。判定の規則は横幅 2560 px の画面で測った値です。EFT は UI の大きさを画面の横幅に合わせて決めるので、スクリーンショットは縦横比を保ったまま横幅 2560 px に拡大・縮小してから判定します（`internal/screenscale`）。16:9（1920×1080、3840×2160 など）は 2560×1440 に、16:10（1920×1200 など）は 2560×1600 になります。16:10 で縦に増えた分は、同じ配置の下に余白として伸びるだけです。
  - ウインドウの判定用には、画素の色を混ぜない拡大・縮小（最近傍）を使います。細い枠線の色がそのまま残るからです。
  - OCR にかける題名の切り出しには、滑らかな拡大・縮小（Catmull-Rom）を使います。
  - 受け付ける縦横比は 16:10〜16:9 です（1366×768 のような端数のある大きさも含みます。高さ 360 px 未満は対象外）。それより横長の画面（ウルトラワイド）は UI の配置が違うので対応していません。`unsupported screenshot resolution` のエラー（`screenscale.ErrUnsupported`。タスク一覧の検出も同じエラーを返します）になります。
  - 実機の 1920×1080 と 1920×1200 のスクリーンショットで、アイテムの詳細画面とタスク一覧を認識できることを確かめています。
- **ヘッダーの検出** (`findHeaders`): y = 40 から高さ −120 まで、x = 20 から幅 −20 までを走査し、枠線色のピクセルが続く横線を探します。
  - 枠線色 (`isBorderPixel`): RGB の最大と最小の差が 16 以下、輝度が 34〜115(半透明のため、青いアイテムの上では (58,62,71) 程度になる。1080p では細く描かれるので (37,40,41) 程度まで暗くなる)。
  - 横線の長さは 100 px 以上。左へ伸ばすときは 3 px までの切れ目を許します(`expandHeaderLeft`)。幅は 480〜1700 px で、その下が暗いタイトルバー(平均輝度 < 60)であることが条件です。
  - 近い位置のヘッダー(y 差 6 以下、x 差 12 以下、幅の差 24 以下)は 1 つにまとめます。
- **閉じるボタンのスコア** (`closeButtonScore`): 右端 44 px の範囲で赤ピクセル(r > 45、r > 2g、r > 1.5b)の比率と明るいピクセル(輝度 > 155、× のグリフ)の比率を数えます。赤が 10% 未満か明るいピクセルが 1.2% 未満なら 0、それ以外は `min(1, 赤×3.2 + 明×2.5 + 幅/3200)` です。
- **判定**: スコアが **.72 以上**、かつ左右の枠線を下にたどったウィンドウの高さが **200 px 以上**(`minWindowHeight`)であること。枠線はヘッダー両端の −2 / 0 / +2 px の位置で調べ、長い方を高さとします(`windowHeight`。拡大・縮小で側面の枠線が 1〜2 px ずれるため)。スタッシュの格子線と、その端にある赤いアイテム(除細動器など)の組み合わせを誤検出しないための条件です。
- **重なりの規則** (`findInspectWindow` / `covered`): 複数のウィンドウが開いていると、フォーカスの有無ではヘッダーの見た目が変わりません。そのため前後関係は隠れ方から判断します。
  - ウィンドウ B のヘッダーが A の本体の中(A の上端 +10 より下、A の高さの範囲内、x が 8 px 以上重なる)に描かれていれば、A は B の後ろにあるとみなします。
  - A のヘッダーの端がちょうど B の側面の枠線(±8 px)で止まっていれば、A は B の後ろにあるとみなします。
  - 閉じるボタンが見えない(スコア < .72)線が A の幅の内側にある場合は、A 自身のレイアウト(出品ウィンドウのパネルなど)として無視します。
  - 覆われている数が最も少ないウィンドウを選び、同数ならスコアの高い方、さらに同じなら画面の下にある方(多くの場合は後から開いた方)を選びます。
- **切り出し範囲** (`CropRect`):

| ウィンドウ | 判定 | 切り出し範囲 | `DetectionLayout` |
|---|---|---|---|
| アイテム詳細 | 幅はおよそ 1068 px | `X+33, Y+2, 幅 W−97, 高さ 26`(タイトル行のみ。左の虫眼鏡アイコン、閉じるボタン、下のカテゴリ行や重量は含めない) | `item-detail` |
| 出品作成 (Offer creation) | 幅が 1560〜1640 px(1440p では 1602 px) | `X+W−682, Y+392, 幅 600, 高さ 38`(OFFER パネルの画像下の品名。日本語の字は英字より背が高いので、上下に余裕を持たせています。ウィンドウが左端からはみ出すことがあるので右端から測る) | `flea-offer` |

- 切り出し幅が 120 px 未満なら `item inspection window is too narrow` のエラーになります。

### タスク一覧の検出 (`internal/taskdetect`)

- プリセットは `Preset2560`（2560×1440）だけです。アイテムと同じく、先に横幅 2560 px に拡大・縮小します。16:10 で縦に長い分は、判定する領域（上からの位置）に影響しません。

| 領域 | Rect (X, Y, W, H) |
|---|---|
| `Anchor`(トレーダー画面のタブ) | 150, 18, 240, 55 |
| `CharacterAnchor`(キャラクター画面のタブ) | 1430, 5, 300, 60 |
| `RaidCharacterAnchor`(レイド中のキャラクター画面) | 1170, 5, 260, 60 |
| `LeftPanel` | 10, 410, 740, 950 |
| `RightPanel` | 770, 410, 1750, 950 |
| `Title`(トレーダー画面のタスク名) | 810, 315, 720, 85 |

- 4 px 間隔で暗部(輝度 < 95)、明部(> 155)、エッジ(隣接差 > 38)の比率を求め、次のスコアを計算します(値は 0〜1 に丸める)。
  - `traderScore = Anchor.明×.72 + (左右のエッジ)×1.2 + (左右の暗部)×.08`
  - `characterScore = max(CharacterAnchor, RaidCharacterAnchor) の 明×.9 + エッジ×.8 + 暗部×.05`
- 高い方を採用し、レイアウトは `trader-tasks` か `character-tasks` になります。`MinScore` の **.42 以上**ならタスク一覧です。
- `character-tasks` の切り出し (`selectedCharacterTitle`): x 300〜800、y 150〜1280 の範囲で、64 px 幅の帯の平均輝度が最も高い帯(選択中の行)を探し、その中心から高さ 96 px を切り出します。右端は行内の縦の区切り線(x < 1300)まで広げます(`nameColumnEnd`)。長いタスク名が途中で切れないようにするためです。

## OCR エンジン (`internal/ocr`)

### エンジンの選択

設定 `OCREngine` は `tesseract`(既定)か `windows` です。これ以外の値は `tesseract` に正規化されます。

- 起動時の移行: `OCRDefaultRevision < 1` のとき、`windows` を選んでいて Tesseract が使える環境では 1 回だけ `tesseract` に切り替えます。
- Tesseract が見つからず `TesseractPath` も空の場合は `windows` に切り替えます。
- Tesseract の探索順 (`tesseractEngine`): `TesseractPath` → 同梱版(`Mayak.exe` と同じフォルダの `tesseract\tesseract.exe` と `tesseract\tessdata`、`ocr.BundledTesseract`)→ `PATH` 上の `tesseract`。
- Tesseract が失敗し、`TesseractPath` が指定されていなければ Windows OCR にフォールバックします(`recognizeQuestTitle` / `recognizeTitleCandidates`)。

### 同梱 Tesseract

- `task tesseract:bundle`(`tools/tessbundle`)が `build/bin/tesseract` に `eng` / `jpn` と MAYAK 独自のモデル `build/tessdata/eft.traineddata`、`eftjpn.traineddata` をまとめます。
- 実行時の引数は `<png> stdout -l <言語> --psm 7 [--tessdata-dir <dir>]` です(1 行モード)。プロセスはウィンドウを出さずに起動します。
- **前処理** (`preprocess`。学習用に `PrepareForTesseract` として公開):
  1. 明るいパネル(平均輝度 ≥ 90)では、左 1/4 にある暗い縦線(高さの 70% 以上が輝度 < 60)より右だけを使います。クエストアイコンとタイトルの区切り線です。
  2. Otsu 法で 2 値化のしきい値を決めます。
  3. 2 倍に拡大し、12 px の白い余白を付けます。
  4. 文字が背景より暗くなければ反転し、つねに白地に黒文字にします(`darkerMinority`)。
  5. 黒ピクセルの範囲に余白を付けて切り詰めます(`trimToInk`)。

### Windows OCR ワーカー

- `windows_ocr.ps1`(埋め込み)を実行する PowerShell プロセスを 1 つだけ常駐させ、標準入出力で 1 行ずつ JSON をやり取りします(`{"path","language"}` → `{"ok","text"|"error"}`)。PowerShell と WinRT の起動コスト(数百 ms)を毎回払わないためです。
- 要求は直列に処理します。1 回のタイムアウトは 30 秒で、10 分間使われなければ終了します。通信の失敗、タイムアウト、キャンセルのあとはプロセスを止め、次の要求で起動し直します(OCR 自体のエラー応答ではそのまま使い続けます)。アプリ終了時には `StopWindowsWorker` で止めます。
- 画像は 2 倍に拡大し、角の色で 16 px の余白を付けます(文字が端に接していると Windows OCR は何も返さないため)。
- `RecognizeAlternatives` は、しきい値 110 / 140 / 170 で 2 値化した 3 枚を読みます。`RecognizeCandidates` は元画像の読み取りとこの 3 枚を合わせ、重複を除いて返します。
- 言語が `auto` の場合は `TryCreateFromUserProfileLanguages`(Windows の表示言語)を使います。言語パックがなければエラーになります。

### 言語

- `ocrLanguage`: 設定 `GameLanguage` が `en` か `internal/locale` の言語(現在は `ja`)ならそのまま使います。`auto`(既定。それ以外の値も `auto` に正規化)なら `eftdetect.GameLanguage()` が `Game.ini` の `Language` から判定します(`jp`/`ja` は `ja`、`en`/`es`/`fr`/`de`/`pl` などのラテン文字言語は `en`)。判定できなければ `auto` になります。日本語版 Windows でも英語クライアントのタイトルを英語として読むためです。
- 言語表 (`internal/ocr/language.go`):

| MAYAK コード | Windows OCR | Tesseract 標準 | MAYAK モデル |
|---|---|---|---|
| `en` | `en-US` | `eng` | `eft` |
| `ja` | `ja-JP` | `jpn` | `eftjpn` |

- tessdata に `<モデル>.traineddata` があれば MAYAK モデルを優先し、なければ標準モデルを使います(`tesseractModel`)。tessdata のフォルダを調べるのは同梱版だけで、`TesseractPath` や `PATH` 上の Tesseract ではつねに標準モデルを使います。
- 英語以外の言語では英語も併用します(例: `-l eftjpn+eft`)。タイトルに英単語、数字、固有名が混ざるためです。表にない言語は英語だけで読み、Windows OCR は `auto` になります。
- 表に行を追加してモデルを同梱すれば、その言語を読めるようになります。言語全般については [languages.md](languages.md) を参照してください。

### MAYAK モデル `eft` / `eftjpn`

- `eft` は `tessdata_best` の `eng` を、`eftjpn` は `jpn` を、ゲームのフォント(Bender、日本語は Meiryo / Yu Gothic)で描いたカタログのタスク名・アイテム名で追加学習したものです。学習用の画像も `PrepareForTesseract` を通すので、実際に読む画像と同じ見た目で学習します。
- 手順は [`tools/ocrtrain/README.md`](../tools/ocrtrain/README.md) にあります。

## 照合

### `questmatch.Normalize`

1. NFKC 正規化と小文字化。
2. 英語の `Part ろ` を `Part 3` に直します(日本語の「ろ」自体はそのまま残す)。
3. 数字と記号の取り違えを直します: `0`→`o`、`1`→`i`、`|`→`i`。
4. 文字と数字以外を空白にし、前後の空白を削ります。
5. 日本語(漢字・ひらがな・カタカナ)に隣接する空白を削除します(Windows OCR は文字の間に空白を入れるため)。英単語の区切りは残します。

### `questmatch.Similarity`

- 正規化した文字列どうしの編集距離から `1 − 距離 / 長い方の長さ` を計算します。`i` と `l` の置換コストは .2 です(EFT の細い大文字フォントでよく取り違えられるため)。
- **先頭のゴミの除去**: 先頭の単独の `i`(切り出し端や虫眼鏡アイコン)を除いたもの、および 1〜2 文字のトークンを先頭から最大 3 個除いたもの(例: `. だ ん`)も候補にします。これらの候補のスコアからは .001 を引きます。
- **部分一致**: 7 文字以上の読み取りが対象に含まれていれば `.84 + .12 × 読み取り長 / 対象長` を下限にします(Windows OCR が表の区切り線の後ろの先頭部分を落とすことがあるため)。`Part 1` のような短い断片は自動で一致しません。
- `questmatch.Match` は英語名と別名(`internal/locale` の言語の名前。`questapi` が同じタスク ID で付与)の最高値をスコアにし、降順(同点は名前順)に並べます。

### `itemmatch.Match`

- 英語名 `Name`、他言語名 `Aliases`(`internal/locale` の言語。現在は日本語。`itemapi` が付与)に対して `Similarity` を計算します。
- 略称 `ShortName` と `ShortAliases` のスコアは **×0.97** します。略称は短く偶然一致しやすいので、正式名より少し不利にしています。
- 表示には `Names`(言語別の名前)を使います。カタログについては [catalog.md](catalog.md) を参照してください。

### しきい値と再読み取り

| 対象 | 処理 | 値 |
|---|---|---|
| アイテム | 一致として採用・表示 | ≥ .82 |
| アイテム | Tesseract で読んだ最良値が .82 未満(または候補なし)なら、Windows OCR の `RecognizeCandidates` で再読み取り | < .82 |
| タスク | 一致として採用 | ≥ .78 |
| タスク | Tesseract の最良値が .9 未満なら、Windows OCR の `RecognizeAlternatives` で再読み取り | < .9 |
| タスク | エンジンが `windows` で最良値が .98 未満なら、2 値化した画像で再読み取り | < .98 |

- 再読み取りの結果は元の読み取りと合わせ、最も高い一致を得た読み取りを採用します(`bestItemReading` / `bestQuestReading`)。
- アイテムは `recognizeTitleCandidates` を使います。Windows OCR で読んだ場合(設定が `windows` のときと、同梱 Tesseract が失敗して代わりに読んだとき)は最初から元画像と 2 値化画像の全候補を読むので、Windows OCR での再読み取りはしません。
- 候補は上位 5 件を `ItemCandidates` / `QuestCandidates` に保存し、読み取った生の文字列を `OCRRaw` に保存します。どの読み取りも一致しなかったときは、最初の(空でない)読み取りを `OCRRaw` に残します。

## 照合したあとの処理

- **アイテム**(≥ .82): `LastItem`、`ItemID`、アイコンなどを設定し、`showBrowserItem` がカタログの情報を `browser:item` で送ります。そのあと最新価格を取得して送り直します(タイムアウト 45 秒)。これでアイテムサイドバーが開きます([item-panel.md](item-panel.md))。.82 未満なら警告ログだけを出します。
- **タスク**(≥ .78):
  - `LastQuest`、トレーダー、マップ、目標、`https://tarkov.dev/task/<normalizedName>`、Wiki の URL を設定します。
  - 認識音を鳴らします(同じキーは `recognitionSoundCooldown` の間は鳴らさない)。一致しなかった場合はエラー音を鳴らします。
  - `showBrowserTask` が設定 `QuestSite`(tarkov-dev / official-wiki / japanese-wiki)のページを `browser:task` で開きます。
  - `tasks` の役割を持つ Remote Control ターゲットには `task/<slug>` を送ります。slug がなければ `map/<QuestMap>` を送ります。詳しくは [tasks-and-maps.md](tasks-and-maps.md) と [settings-and-integrations.md](settings-and-integrations.md) を参照してください。
- **レイド状態**: 各分析では `logdetect.RaidState` で `RaidActive` を更新します。位置情報を使うのはレイド中だけです。
- エラー時は `updateAnalysisError` が「解析エラー」を設定し、ログとエラー音で知らせます。

## スクリーンショットの保持 (`internal/screenshotstore`)

- 設定 `ScreenshotCleanup`(既定はオフ)が有効な場合、スクリーンショットフォルダ直下の画像を更新時刻の新しい順に並べ、次のどちらかに当てはまるものを削除します。
  - `ScreenshotRetainCount` 件目より後(既定 500、範囲 0〜100000、0 は無制限)
  - `ScreenshotRetainHours` 時間より古い(既定 168、範囲 0〜87600、0 は無制限)
- 実行タイミングは、各解析のあと(解析中の画像は保護)、起動時と 30 分ごと(`watchScreenshotMaintenance`)、保持設定を変えたときです。ブラウザクライアントモードでは実行しません。
- サブフォルダ(`Mayak-Debug` など)は削除しません。相対パスやボリュームのルートは対象にしません。

## デバッグ出力 (`Mayak-Debug`)

- 設定 `SaveRecognitionDebug`(既定はオフ)が有効な場合、各解析のあとに `<Screenshots>\Mayak-Debug\` に次のファイルを保存します(`screenshotstore.SaveDebug`)。
  - `<yyyyMMdd-HHmmss.fff>_<種類>_<スコア×1000>_source.<拡張子>`: 元画像のコピー
  - `…_crop.png`: OCR にかけた切り出し画像
  - `….json`: `Metadata`(種類、レイド状態、マップ、レイアウト、スコア、段階、`ocrRaw`、クエストまたはアイテムと候補、位置、エラー)。`detectionDetails` には `taskdetect` と `itemdetect` の両方のスコアと切り出し範囲を記録します(画像のデコードは両方で 1 回だけ。デコードできなければ `error` だけを記録)。
- 保存先はスクリーンショットフォルダの直下にある画像に限ります。書き込みは一時ファイルからの rename で行います。`OpenDebugDirectory` でフォルダを開けます。
- このデータは `cmd/ocreval` の評価用データとしてそのまま使えます。

## ツール

| ツール | 用途 |
|---|---|
| `cmd/ocrharvest` | スクリーンショットフォルダ(`.png` / `.jpg` / `.jpeg`)から、アプリと同じ順序(アイテム → タスク)で切り出し画像を集めます。Windows OCR、`eft`(同梱 tessdata の MAYAK モデル。日本語なら `eftjpn+eft`)、`eng`(tessdata を指定しない Tesseract 標準モデル。日本語なら `jpn+eng`)の 3 エンジンで読み、`-min-confidence`(既定 .9)以上で 2 エンジン以上が同じ名前に一致したもの、または 1 エンジンが完全一致(≥ .999)したものだけを `<kind>_<n>.png` / `.json` として保存します。ラベルには英語名と別名のうち読み取りに近い方(画面に表示された名前)を使います。`-lang`(既定 `en`。MAYAK の言語コード。言語表にない言語は英語として読む)、`-screens`、`-catalog`、`-tesseract`、`-out` を指定します。 |
| `cmd/ocreval` | `Mayak-Debug` の `*_tasks_*` の記録(または `ocrharvest` の出力)を正解として、Windows OCR(毎回起動 / 常駐ワーカー)と任意の Tesseract(`-tesseract`、`-tessdata`)について、完全一致数、.9 以上の一致数、平均時間を比べます。`-lang` は MAYAK の言語コード(既定 `en`。`ja` なら `jpn`、`-tessdata` に `eftjpn` があればそれと英語を併用)です。`-min-confidence` の既定は .8、`-winlang` の既定は `auto` です。 |
| `tools/ocrtrain` | カタログの名前をゲームのフォントで描き、`PrepareForTesseract` を通した行画像、正解テキスト、学習用・評価用のリストを作ります(`-lang` 既定 `en`。ほかの言語はそのコードで、カタログの `items_<コード>` / `tasks_<コード>` の名前を描く、`-latin-fonts`、`-variants` 既定 2、`-eval-percent` 既定 5)。学習と入れ替えの手順は [`tools/ocrtrain/README.md`](../tools/ocrtrain/README.md) を参照してください。 |

ビルドと開発の環境については [development.md](development.md) を参照してください。
