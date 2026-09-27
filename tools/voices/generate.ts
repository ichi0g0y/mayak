// Writes the built-in voices' WAVs (internal/sound/voices/<pack>/<kind>.wav)
// from each pack.json's lines, with the VOICEVOX engine running locally
// (the VOICEVOX app starts it on 127.0.0.1:50021).
//
//	task voices              every pack
//	task voices -- tsumugi   the packs named
//
// A line is its text, or {text, style} for another of the speaker's styles,
// {kana} to give its reading and accents in AquesTalk-like notation (as the
// engine's /audio_query returns them), and {query} for the synthesis
// settings of that line (speedScale, pitchScale, intonationScale…). A pack's
// voicevox.query sets them for all of its lines. The WAVs are IMA ADPCM
// (adpcm.ts), a quarter of the engine's 16-bit PCM.
import { readdirSync } from 'node:fs'
import { join } from 'node:path'
import { adpcmWave, pcmSamples } from './adpcm'

const engine = process.env.VOICEVOX_URL ?? 'http://127.0.0.1:50021'
const root = join(import.meta.dir, '../../internal/sound/voices')
// The notifications (internal/sound/sound.go Kinds).
const kinds = [
  'quest',
  'taskNotMatched',
  'item',
  'itemNotMatched',
  'error',
  'remoteError',
  'hideoutError',
  'matchFound',
  'raidStart',
  'runThrough',
  'questItems',
  'restartTasks',
]

type Line = string | { text: string; style?: number; kana?: string; query?: Record<string, number> }
type Pack = {
  voicevox?: { speaker: number; query?: Record<string, number> }
  lines: Record<string, Line>
}

async function post(path: string, body?: unknown) {
  const res = await fetch(engine + path, {
    method: 'POST',
    headers: body === undefined ? undefined : { 'content-type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${path}: ${res.status} ${await res.text()}`)
  return res
}

try {
  await (await fetch(engine + '/version')).text()
} catch {
  console.error(`The VOICEVOX engine does not answer at ${engine}: start the VOICEVOX app.`)
  process.exit(1)
}

const wanted = process.argv.slice(2)
const ids = readdirSync(root, { withFileTypes: true })
  .filter((e) => e.isDirectory() && (wanted.length === 0 || wanted.includes(e.name)))
  .map((e) => e.name)
for (const id of wanted) if (!ids.includes(id)) throw new Error(`no pack ${id}`)

for (const id of ids) {
  const pack: Pack = await Bun.file(join(root, id, 'pack.json')).json()
  if (!pack.voicevox) continue
  for (const kind of kinds) {
    const raw = pack.lines[kind]
    if (!raw) throw new Error(`${id}: no line for ${kind}`)
    const line = typeof raw === 'string' ? { text: raw } : raw
    const speaker = line.style ?? pack.voicevox.speaker
    const query = await (
      await post(`/audio_query?speaker=${speaker}&text=${encodeURIComponent(line.text)}`)
    ).json()
    if (line.kana) {
      query.accent_phrases = await (
        await post(`/accent_phrases?speaker=${speaker}&is_kana=true&text=${encodeURIComponent(line.kana)}`)
      ).json()
    }
    // Short silences around the line: a notification starts at once.
    Object.assign(query, { prePhonemeLength: 0.05, postPhonemeLength: 0.1 }, pack.voicevox.query, line.query)
    const wav = await (await post(`/synthesis?speaker=${speaker}`, query)).arrayBuffer()
    const { rate, samples } = pcmSamples(wav)
    await Bun.write(join(root, id, `${kind}.wav`), adpcmWave(rate, samples))
    console.log(`${id}/${kind}: ${query.kana}`)
  }
}
