import {test,expect} from 'bun:test';
import {words,t} from './words.js';

test('both languages carry the same keys and no empty text',()=>{
  const ja=Object.keys(words.ja),en=Object.keys(words.en);
  expect(en.filter(k=>!(k in words.ja))).toEqual([]);
  expect(ja.filter(k=>!(k in words.en))).toEqual([]);
  for(const language of ['ja','en'])for(const [key,text] of Object.entries(words[language]))expect(typeof text==='string'&&text.length>0,`${language}.${key}`).toBe(true);
});

test('t falls back to Japanese, then to the key',()=>{
  expect(t('en','close')).toBe('Close');
  expect(t('fr','close')).toBe('閉じる');
  expect(t('ja','no-such-key')).toBe('no-such-key');
});
