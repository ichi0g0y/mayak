import test from 'node:test'
import assert from 'node:assert/strict'
import {
  itemInfo,
  bestSale,
  fleaFee,
  price,
  age,
  historyPoints,
  chartSeries,
  chartPath,
  itemPageURL,
  names,
  offerNeeds,
} from './item.js'

test('item details are validated before rendering', () => {
  assert.equal(itemInfo(null), null)
  assert.equal(itemInfo({ id: 'x' }), null)
  const item = itemInfo({
    id: 'gpu',
    name: 'Graphics card',
    mode: 'evil',
    iconUrl: 'javascript:alert(1)',
    link: 'https://tarkov.dev/item/graphics-card',
    width: 2,
    height: 1,
    flea: { lastLow: 700000, avg24h: 'many' },
    traders: [{ trader: 'Therapist', price: 100980, currency: 'RUB', priceRub: 100980 }],
    tasks: [{ id: 't', name: 'Half-Empty', count: 10, foundInRaid: 1, state: 'weird' }],
    hideout: [{ station: 'Lav', level: 2, count: 1, complete: true }],
    pricedAt: 'nope',
  })
  assert.equal(item.mode, 'regular')
  assert.equal(item.iconUrl, '')
  assert.equal(item.link, 'https://tarkov.dev/item/graphics-card')
  assert.equal(item.flea.avg24h, 0)
  assert.equal(item.tasks[0].state, '')
  assert.equal(item.tasks[0].foundInRaid, true)
  assert.equal(item.hideout[0].complete, true)
  assert.equal(item.pricedAt, '')
})

test('trader offers are validated with the task that unlocks them', () => {
  const item = itemInfo({
    id: 'gpu',
    name: 'Graphics card',
    offers: [
      { trader: 'Prapor', price: 140000, currency: 'RUB', priceRub: 140000, level: 1, playerLevel: 0, reputation: 0 },
      {
        trader: 'Mechanic',
        price: 1000,
        currency: 'USD',
        priceRub: 150000,
        level: 2,
        playerLevel: 12,
        reputation: 0.6,
        buyLimit: 5,
        task: {
          id: 'g1',
          name: 'Gunsmith - Part 1',
          state: 'completed',
          urls: { 'tarkov-dev': 'https://tarkov.dev/task/g1' },
        },
      },
      { trader: 'Skier', level: 'x', reputation: 'high', task: { name: 'no id' } },
      {
        trader: 'Ref',
        level: 3,
        count: 2,
        barter: [
          {
            id: 'gp',
            name: 'GP coin',
            names: { ja: 'GP コイン' },
            iconUrl: 'https://assets.tarkov.dev/gp.webp',
            count: 7.51,
          },
          { id: 'none', name: 'No count' },
          'junk',
        ],
      },
      'junk',
    ],
  })
  assert.equal(item.offers.length, 4)
  const [prapor, mechanic, skier, ref] = item.offers
  assert.deepEqual(prapor.barter, [])
  assert.equal(prapor.count, 1)
  assert.equal(ref.count, 2)
  assert.deepEqual(ref.barter, [
    {
      id: 'gp',
      name: 'GP coin',
      names: { ja: 'GP コイン' },
      shortName: '',
      iconUrl: 'https://assets.tarkov.dev/gp.webp',
      count: 7.51,
    },
  ])
  assert.equal(prapor.task, null)
  assert.equal(prapor.buyLimit, 0)
  assert.equal(offerNeeds(prapor), null)
  assert.equal(mechanic.task.name, 'Gunsmith - Part 1')
  assert.equal(mechanic.task.state, 'completed')
  assert.equal(mechanic.task.urls['tarkov-dev'], 'https://tarkov.dev/task/g1')
  assert.deepEqual(offerNeeds(mechanic), { level: 12, reputation: '0.60' })
  assert.equal(skier.level, 1)
  assert.equal(skier.reputation, 0)
  assert.equal(skier.task, null)
  assert.deepEqual(itemInfo({ id: 'x', name: 'X' }).offers, [])
})

test('best sale compares the flea market with the best trader', () => {
  const base = { traders: [{ trader: 'Therapist', priceRub: 100000 }] }
  assert.deepEqual(bestSale({ ...base, flea: { lastLow: 700000 } }), { where: 'flea', priceRub: 700000, fee: null })
  assert.deepEqual(bestSale({ ...base, flea: null }), { where: 'Therapist', priceRub: 100000 })
  assert.equal(bestSale({ traders: [], flea: null }), null)
})

test('prices and ages are formatted for the language', () => {
  assert.equal(price(1234567, 'RUB', 'en'), '1,234,567 ₽')
  assert.equal(price(575, 'USD', 'en'), '$575')
  const now = Date.parse('2026-09-23T13:00:00Z')
  assert.equal(age('2026-09-23T12:55:00Z', 'ja', now), '5分前')
  assert.equal(age('2026-09-23T10:00:00Z', 'en', now), '3 h ago')
  assert.equal(age('', 'ja', now), '')
  assert.equal(age('2026-09-20T13:00:00Z', 'ja', now), '3日前')
  assert.equal(age('2025-12-19T13:00:00Z', 'ja', now), '9か月前')
  assert.equal(age('2025-12-19T13:00:00Z', 'en', now), '9 months ago')
})

test('price history becomes a chart series', () => {
  assert.deepEqual(historyPoints([{ t: 3, price: 10, min: 9 }, { t: 1, price: 8, min: 7 }, { t: 2, price: 0 }, 'x']), [
    { t: 1, price: 8, min: 7 },
    { t: 3, price: 10, min: 9 },
  ])
  const day = 864e5,
    now = 100 * day
  const points = [
    { t: now - 40 * day, price: 500, min: 400 },
    { t: now - 6 * day, price: 100, min: 90 },
    { t: now - 3 * day, price: 120, min: 100 },
    { t: now - day, price: 110, min: 95 },
  ]
  const week = chartSeries(points, '7d', now, { t: now, min: 105 })
  assert.equal(week.pts.length, 4)
  assert.equal(week.pts.at(-1).now, true)
  assert.equal(week.high, 120)
  assert.equal(week.low, 90)
  assert.equal(Math.round(week.change), 10)
  assert.equal(chartSeries(points, 'all', now).pts.length, 4)
  assert.equal(chartSeries([points[0]], '7d', now), null)
  const path = chartPath(week, 'price', 100, 50)
  assert.match(path, /^M0\.0 [\d.]+L/)
  assert.equal((path.match(/[ML]/g) || []).length, 3)
})

test('the item page follows the task site', () => {
  const item = {
    name: 'Graphics card',
    link: 'https://tarkov.dev/item/graphics-card',
    wikiLink: 'https://escapefromtarkov.fandom.com/wiki/Graphics_card',
  }
  assert.equal(itemPageURL(item, 'tarkov-dev'), item.link)
  assert.equal(itemPageURL(item, 'official-wiki'), item.wikiLink)
  assert.equal(itemPageURL(item, 'japanese-wiki'), 'https://wikiwiki.jp/eft/Graphics%20card')
  assert.equal(itemPageURL({ ...item, wikiLink: '' }, 'official-wiki'), item.link)
})

test('names by language keep language codes and text only', () => {
  assert.deepEqual(names({ ja: ' グラフィックボード ', fr: '', bad_code: 'x', de: 3 }), { ja: 'グラフィックボード' })
  assert.deepEqual(names(null), {})
  assert.deepEqual(names(['ja']), {})
})

test("a key's wiki usage is one of two words, else none", () => {
  const key = (keyUse) => itemInfo({ id: 'k', name: 'Room 323 key', keyUse }).keyUse
  assert.equal(key('open'), 'open')
  assert.equal(key('none'), 'none')
  assert.equal(key('<b>'), '')
  assert.equal(key(undefined), '')
})

test("traders' pictures keep web addresses only", () => {
  const item = itemInfo({
    id: 'x',
    name: 'X',
    traderImages: {
      Prapor: 'https://assets.tarkov.dev/prapor.webp',
      Bad: 'javascript:alert(1)',
      '': 'https://a.b/c.webp',
    },
  })
  assert.deepEqual(item.traderImages, { Prapor: 'https://assets.tarkov.dev/prapor.webp' })
  assert.deepEqual(itemInfo({ id: 'x', name: 'X', traderImages: 'junk' }).traderImages, {})
})

test("the flea market's fee is tarkov.dev's, the Intel Center's level 3 taking 30% off", () => {
  const rates = { offerRate: 0.05, requirementRate: 0.05, intel: null }
  // tarkov.dev's API gives 5968 for this item (base 67397, listed at 47776).
  const item = { basePrice: 67397, flea: { lastLow: 47776 }, fleaFee: rates, traders: [] }
  assert.equal(fleaFee(item), 5968)
  assert.equal(fleaFee({ ...item, fleaFee: { ...rates, intel: true } }), 4178)
  assert.equal(fleaFee({ ...item, fleaFee: { ...rates, intel: false } }), 5968)
  // Either way on purpose, whatever TarkovTracker says.
  assert.equal(fleaFee(item, 47776, true), 4178)
  assert.equal(fleaFee({ ...item, fleaFee: { ...rates, intel: true } }, 47776, false), 5968)
  // Listed above the base price, the asked part grows.
  assert.ok(fleaFee(item, 200000) > fleaFee(item, 100000))
  assert.equal(fleaFee({ ...item, fleaFee: null }), null)
  assert.equal(fleaFee({ ...item, basePrice: 0 }), null)
  assert.equal(fleaFee({ ...item, flea: null }), null)
})

test('the best sale compares what is left after the fee with the best trader', () => {
  const rates = { offerRate: 0.05, requirementRate: 0.05, intel: null }
  const item = {
    basePrice: 67397,
    flea: { lastLow: 47776 },
    fleaFee: rates,
    traders: [{ trader: 'Therapist', priceRub: 45000 }],
  }
  // 47776 is more than the trader's 45000, but 47776 - 5968 is not.
  assert.deepEqual(bestSale(item), { where: 'Therapist', priceRub: 45000 })
  assert.deepEqual(bestSale({ ...item, traders: [{ trader: 'Therapist', priceRub: 40000 }] }), {
    where: 'flea',
    priceRub: 47776 - 5968,
    fee: 5968,
  })
})
