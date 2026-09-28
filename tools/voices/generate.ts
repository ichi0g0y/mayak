// Writes the built-in voices' WAVs (internal/sound/voices/<pack>/<kind>.wav)
// from each pack.json's lines (voices.ts has how).
//
//	task voices              every pack
//	task voices -- tsumugi   the packs named
//
// task dev:voice opens an editor of the lines that writes them too (server.ts).
import { kinds, loadPack, packIDs, voicevoxReady, writeLine } from './voices'

const wanted = process.argv.slice(2)
const ids = wanted.length ? wanted : packIDs()
const packs = await Promise.all(ids.map(async (id) => [id, await loadPack(id)] as const))
if (packs.some(([, pack]) => pack.voicevox) && !(await voicevoxReady())) {
  console.error('The VOICEVOX engine does not answer: start the VOICEVOX app.')
  process.exit(1)
}
for (const [id, pack] of packs) {
  for (const kind of kinds) {
    const kana = await writeLine(id, pack, kind)
    console.log(`${id}/${kind}: ${kana || (typeof pack.lines[kind] === 'string' ? pack.lines[kind] : '')}`)
  }
}
