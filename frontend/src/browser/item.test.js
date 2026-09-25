import test from 'node:test';
import assert from 'node:assert/strict';
import {itemInfo,bestSale,price,age,historyPoints,chartSeries,chartPath,itemPageURL,names} from './item.js';

test('item details are validated before rendering',()=>{
 assert.equal(itemInfo(null),null);
 assert.equal(itemInfo({id:'x'}),null);
 const item=itemInfo({id:'gpu',name:'Graphics card',mode:'evil',iconUrl:'javascript:alert(1)',link:'https://tarkov.dev/item/graphics-card',width:2,height:1,flea:{lastLow:700000,avg24h:'many'},traders:[{trader:'Therapist',price:100980,currency:'RUB',priceRub:100980}],tasks:[{id:'t',name:'Half-Empty',count:10,foundInRaid:1,state:'weird'}],hideout:[{station:'Lav',level:2,count:1,complete:true}],pricedAt:'nope'});
 assert.equal(item.mode,'regular');
 assert.equal(item.iconUrl,'');
 assert.equal(item.link,'https://tarkov.dev/item/graphics-card');
 assert.equal(item.flea.avg24h,0);
 assert.equal(item.tasks[0].state,'');
 assert.equal(item.tasks[0].foundInRaid,true);
 assert.equal(item.hideout[0].complete,true);
 assert.equal(item.pricedAt,'');
});

test('best sale compares the flea market with the best trader',()=>{
 const base={traders:[{trader:'Therapist',priceRub:100000}]};
 assert.deepEqual(bestSale({...base,flea:{lastLow:700000}}),{where:'flea',priceRub:700000});
 assert.deepEqual(bestSale({...base,flea:null}),{where:'Therapist',priceRub:100000});
 assert.equal(bestSale({traders:[],flea:null}),null);
});

test('prices and ages are formatted for the language',()=>{
 assert.equal(price(1234567,'RUB','en'),'1,234,567 ₽');
 assert.equal(price(575,'USD','en'),'$575');
 const now=Date.parse('2026-09-23T13:00:00Z');
 assert.equal(age('2026-09-23T12:55:00Z','ja',now),'5分前');
 assert.equal(age('2026-09-23T10:00:00Z','en',now),'3 h ago');
 assert.equal(age('',"ja",now),'');
 assert.equal(age('2026-09-20T13:00:00Z','ja',now),'3日前');
 assert.equal(age('2025-12-19T13:00:00Z','ja',now),'9か月前');
 assert.equal(age('2025-12-19T13:00:00Z','en',now),'9 months ago');
});

test('price history becomes a chart series',()=>{
 assert.deepEqual(historyPoints([{t:3,price:10,min:9},{t:1,price:8,min:7},{t:2,price:0},'x']),[{t:1,price:8,min:7},{t:3,price:10,min:9}]);
 const day=864e5,now=100*day;
 const points=[{t:now-40*day,price:500,min:400},{t:now-6*day,price:100,min:90},{t:now-3*day,price:120,min:100},{t:now-day,price:110,min:95}];
 const week=chartSeries(points,'7d',now,{t:now,min:105});
 assert.equal(week.pts.length,4);
 assert.equal(week.pts.at(-1).now,true);
 assert.equal(week.high,120);
 assert.equal(week.low,90);
 assert.equal(Math.round(week.change),10);
 assert.equal(chartSeries(points,'all',now).pts.length,4);
 assert.equal(chartSeries([points[0]],'7d',now),null);
 const path=chartPath(week,'price',100,50);
 assert.match(path,/^M0\.0 [\d.]+L/);
 assert.equal((path.match(/[ML]/g)||[]).length,3);
});

test('the item page follows the task site',()=>{
 const item={name:'Graphics card',link:'https://tarkov.dev/item/graphics-card',wikiLink:'https://escapefromtarkov.fandom.com/wiki/Graphics_card'};
 assert.equal(itemPageURL(item,'tarkov-dev'),item.link);
 assert.equal(itemPageURL(item,'official-wiki'),item.wikiLink);
 assert.equal(itemPageURL(item,'japanese-wiki'),'https://wikiwiki.jp/eft/Graphics%20card');
 assert.equal(itemPageURL({...item,wikiLink:''},'official-wiki'),item.link);
});

test('names by language keep language codes and text only',()=>{
 assert.deepEqual(names({ja:' グラフィックボード ',fr:'',bad_code:'x',de:3}),{ja:'グラフィックボード'});
 assert.deepEqual(names(null),{});
 assert.deepEqual(names(['ja']),{});
});
