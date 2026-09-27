// Writes the built-in voices' WAVs (internal/sound/voices/<pack>/<kind>.wav)
// from each pack.json's lines: Japanese with the VOICEVOX engine running
// locally (the VOICEVOX app starts it on 127.0.0.1:50021), English with
// Kokoro-82M (kokoro-js, which downloads the model on first use).
//
//	task voices              every pack
//	task voices -- tsumugi   the packs named
//
// A VOICEVOX line is its text, or {text, style} for another of the speaker's
// styles, {kana} to give its reading and accents in AquesTalk-like notation
// (as the engine's /audio_query returns them), and {query} for the synthesis
// settings of that line (speedScale, pitchScale, intonationScale…). A pack's
// voicevox.query sets them for all of its lines. A Kokoro line is its text,
// or {text, speed}; kokoro.speed sets the pack's. The WAVs are IMA ADPCM
// (adpcm.ts), a quarter of 16-bit PCM.
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

type Line = string | { text: string; style?: number; kana?: string; query?: Record<string, number>; speed?: number }
type Pack = {
  voicevox?: { speaker: number; query?: Record<string, number> }
  kokoro?: { voice: string; speed?: number }
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

async function voicevox(pack: NonNullable<Pack['voicevox']>, line: Exclude<Line, string>) {
  const speaker = line.style ?? pack.speaker
  const query = await (await post(`/audio_query?speaker=${speaker}&text=${encodeURIComponent(line.text)}`)).json()
  if (line.kana) {
    query.accent_phrases = await (
      await post(`/accent_phrases?speaker=${speaker}&is_kana=true&text=${encodeURIComponent(line.kana)}`)
    ).json()
  }
  // Short silences around the line: a notification starts at once.
  Object.assign(query, { prePhonemeLength: 0.05, postPhonemeLength: 0.1 }, pack.query, line.query)
  const wav = await (await post(`/synthesis?speaker=${speaker}`, query)).arrayBuffer()
  console.log(`  ${query.kana}`)
  return pcmSamples(wav)
}

let kokoroModel: any
async function kokoro(pack: NonNullable<Pack['kokoro']>, line: Exclude<Line, string>) {
  if (!kokoroModel) {
    // The phonemizer kokoro-js uses needs DecompressionStream, which Bun lacks.
    if (!(globalThis as any).DecompressionStream) {
      const zlib = await import('node:zlib')
      const { Duplex } = await import('node:stream')
      ;(globalThis as any).DecompressionStream = class {
        readable: ReadableStream
        writable: WritableStream
        constructor(format: string) {
          const z =
            format === 'gzip' ? zlib.createGunzip() : format === 'deflate' ? zlib.createInflate() : zlib.createInflateRaw()
          const web = Duplex.toWeb(z) as unknown as { readable: ReadableStream; writable: WritableStream }
          this.readable = web.readable
          this.writable = web.writable
        }
      }
    }
    const { KokoroTTS } = await import('kokoro-js')
    kokoroModel = await KokoroTTS.from_pretrained('onnx-community/Kokoro-82M-v1.0-ONNX', { dtype: 'fp32', device: 'cpu' })
  }
  const audio = await kokoroModel.generate(line.text, { voice: pack.voice, speed: line.speed ?? pack.speed ?? 1 })
  const samples = Int16Array.from(audio.audio as Float32Array, (v) => Math.max(-32768, Math.min(32767, Math.round(v * 32767))))
  return { rate: audio.sampling_rate as number, samples }
}

const wanted = process.argv.slice(2)
const ids = readdirSync(root, { withFileTypes: true })
  .filter((e) => e.isDirectory() && (wanted.length === 0 || wanted.includes(e.name)))
  .map((e) => e.name)
for (const id of wanted) if (!ids.includes(id)) throw new Error(`no pack ${id}`)
const packs = await Promise.all(ids.map(async (id) => [id, (await Bun.file(join(root, id, 'pack.json')).json()) as Pack] as const))

if (packs.some(([, pack]) => pack.voicevox)) {
  try {
    await (await fetch(engine + '/version')).text()
  } catch {
    console.error(`The VOICEVOX engine does not answer at ${engine}: start the VOICEVOX app.`)
    process.exit(1)
  }
}

for (const [id, pack] of packs) {
  for (const kind of kinds) {
    const raw = pack.lines[kind]
    if (!raw) throw new Error(`${id}: no line for ${kind}`)
    const line = typeof raw === 'string' ? { text: raw } : raw
    console.log(`${id}/${kind}: ${line.text}`)
    const { rate, samples } = pack.voicevox
      ? await voicevox(pack.voicevox, line)
      : pack.kokoro
        ? await kokoro(pack.kokoro, line)
        : (() => {
            throw new Error(`${id}: no voicevox or kokoro`)
          })()
    await Bun.write(join(root, id, `${kind}.wav`), adpcmWave(rate, samples))
  }
}
