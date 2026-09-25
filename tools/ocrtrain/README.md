# Tarkov OCR model (`eft.traineddata`)

MAYAK reads quest and item titles with the bundled Tesseract. The `eft`
model is Tesseract's English LSTM model fine-tuned on Escape from Tarkov
titles in the game's font; `internal/ocr` uses it instead of `eng` whenever the
bundled tessdata directory contains it. The model lives in
`build/tessdata/eft.traineddata` and `task tesseract:bundle` copies it next to
Mayak.exe.

## Retraining

Tools come from the UB Mannheim build that `task tesseract:bundle` unpacks into
`build/tesseract-cache/extracted` (`tesseract.exe`, `combine_tessdata.exe`,
`lstmtraining.exe`, `lstmeval.exe`). Paths below are examples.

1. Inputs:
   - Bender font (SIL OFL), e.g. from 1001fonts.com, unpacked to `fonts/`.
   - `eng.traineddata` from `tesseract-ocr/tessdata_best` (only the "best"
     float models can be fine-tuned).
   - A MAYAK catalog snapshot: `%AppData%\Mayak\catalog\pve.json`.
2. Generate line images, ground truth and box files. Every image goes through
   `ocr.PrepareForTesseract`, the preprocessing used when reading:

       go run ./tools/ocrtrain -catalog %AppData%\Mayak\catalog\pve.json -fonts fonts -out data

3. Turn each line into an `.lstmf` file:

       tesseract data/lines/NAME.png data/lines/NAME --psm 13 lstm.train

4. Extract the LSTM network and fine-tune it:

       combine_tessdata -e eng_best.traineddata eng.lstm
       lstmtraining --continue_from eng.lstm --traineddata eng_best.traineddata \
         --model_output out/eft --train_listfile data/train.txt --eval_listfile data/eval.txt \
         --max_iterations 8000

5. Write the integer (fast) model and compare it with the original:

       lstmtraining --stop_training --convert_to_int --continue_from out/eft_checkpoint \
         --traineddata eng_best.traineddata --model_output eft.traineddata
       lstmeval --model eng.lstm --traineddata eng_best.traineddata --eval_listfile data/eval.txt
       lstmeval --model out/eft_checkpoint --traineddata eng_best.traineddata --eval_listfile data/eval.txt
       go run ./cmd/ocreval -dir "<Screenshots>/Mayak-Debug" -tesseract build/bin/tesseract/tesseract.exe \
         -tessdata <dir with eft.traineddata> -lang en

   Replace `build/tessdata/eft.traineddata` only when both the held-out lines
   and the real screenshots (`cmd/ocreval`) do not get worse.

## Japanese model (`eftjpn.traineddata`)

`internal/ocr` reads Japanese titles with `eftjpn` instead of `jpn` when the
bundled tessdata has it. It is `jpn` from `tessdata_best` fine-tuned the same
way, on the catalog's Japanese item and task names:

1. Inputs:
   - Japanese fonts like the game's (it embeds Noto Sans CJK and falls back to
     Meiryo): `meiryo.ttc`, `YuGothR.ttc` and `YuGothM.ttc` from
     `C:\Windows\Fonts` in `fonts-ja/`. Variable fonts (e.g. NotoSansJP-VF)
     draw at their thinnest weight here: leave them out.
   - Bender in `fonts/`: the game draws the Latin letters and digits in
     Japanese names with it.
   - `jpn.traineddata` from `tessdata_best`, saved as `jpn_best.traineddata`,
     and a tessdata directory with it as `jpn.traineddata` plus `configs/`
     (for `lstm.train`).
   - A catalog snapshot with `items_ja` and `tasks_ja` (MAYAK fetches both).
2. Generate the lines:

       go run ./tools/ocrtrain -lang ja -catalog %AppData%\Mayak\catalog\pve.json -fonts fonts-ja -latin-fonts fonts -out data-ja

3. Make the `.lstmf` files with `--tessdata-dir <that directory> -l jpn`, then
   extract `jpn.lstm` from `jpn_best.traineddata` and run `lstmtraining` as
   above with `--max_iterations 8000`. Convert the best checkpoint with
   `--convert_to_int` to `eftjpn.traineddata`.
4. Compare with `lstmeval` against `jpn.lstm`, and on real Japanese screenshots
   with `go run ./cmd/ocrharvest -lang ja -tesseract <dir with tesseract.exe
   and tessdata incl. eftjpn>`. Keep it only when neither gets worse.
