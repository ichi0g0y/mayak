import {webURL,sites} from './state.js';

// Width of the item sidebar beside the page views; the user can drag it
// between min and max.
export const itemPanelWidths={min:260,max:560,default:320};
export const clampItemPanel=value=>Number.isFinite(value)?Math.round(Math.min(itemPanelWidths.max,Math.max(itemPanelWidths.min,value))):itemPanelWidths.default;
// Its height when placed under the page views (itemDock "bottom").
export const itemPanelHeights={min:180,max:560,default:280};
export const clampItemPanelHeight=value=>Number.isFinite(value)?Math.round(Math.min(itemPanelHeights.max,Math.max(itemPanelHeights.min,value))):itemPanelHeights.default;

const text=(value,max=160)=>typeof value==='string'?value.slice(0,max):'';
const count=value=>Number.isFinite(value)?Math.max(0,Math.round(value)):0;
const list=(value,max)=>Array.isArray(value)?value.slice(0,max).filter(v=>v&&typeof v==='object'):[];
const time=value=>typeof value==='string'&&!Number.isNaN(Date.parse(value))?value:'';

// itemInfo validates item details from the Host (or a peer) before the shell
// renders them, and returns null for anything that is not an item.
// names keeps a map of names by language (short codes like "ja") to text.
export function names(raw,max=160){
 if(!raw||typeof raw!=='object'||Array.isArray(raw))return {};
 return Object.fromEntries(Object.entries(raw).filter(([lang,name])=>/^[a-z]{2}(-[a-z]{2})?$/.test(lang)&&typeof name==='string'&&name.trim()).slice(0,20).map(([lang,name])=>[lang,name.trim().slice(0,max)]));
}
export function itemInfo(raw){
 if(!raw||typeof raw!=='object'||!text(raw.id)||!text(raw.name))return null;
 const flea=raw.flea&&typeof raw.flea==='object'?{lastLow:count(raw.flea.lastLow),avg24h:count(raw.flea.avg24h),low24h:count(raw.flea.low24h),high24h:count(raw.flea.high24h),changePercent:Number.isFinite(raw.flea.changePercent)?raw.flea.changePercent:0,offers:count(raw.flea.offers),minLevel:count(raw.flea.minLevel)}:null;
 return {
  id:text(raw.id,40),mode:['regular','pve','pvp-season'].includes(raw.mode)?raw.mode:'regular',name:text(raw.name),shortName:text(raw.shortName,60),names:names(raw.names),shortNames:names(raw.shortNames,60),
  iconUrl:webURL(raw.iconUrl)||'',link:webURL(raw.link)||'',wikiLink:webURL(raw.wikiLink)||'',
  width:Math.max(1,count(raw.width)),height:Math.max(1,count(raw.height)),basePrice:count(raw.basePrice),flea,
  traders:list(raw.traders,12).map(s=>({trader:text(s.trader,40),price:count(s.price),currency:text(s.currency,8),priceRub:count(s.priceRub)})),
  tasks:list(raw.tasks,80).map(s=>({id:text(s.id,40),name:text(s.name),names:names(s.names),trader:text(s.trader,40),count:count(s.count),foundInRaid:!!s.foundInRaid,state:['completed','failed','uncompleted'].includes(s.state)?s.state:'',urls:Object.fromEntries(sites.map(site=>[site,webURL(s.urls?.[site])||'']))})),
  hideout:list(raw.hideout,80).map(s=>({levelId:text(s.levelId,80),station:text(s.station,60),level:count(s.level),count:count(s.count),foundInRaid:!!s.foundInRaid,complete:typeof s.complete==='boolean'?s.complete:null})),
  questSite:sites.includes(raw.questSite)?raw.questSite:'tarkov-dev',live:!!raw.live,pricedAt:time(raw.pricedAt),fetchedAt:time(raw.fetchedAt),
 };
}

const symbols={RUB:'₽',USD:'$',EUR:'€'};
export function price(value,currency='RUB',language='ja'){
 const number=Math.round(value).toLocaleString(language==='ja'?'ja-JP':'en-US');
 return currency==='RUB'?`${number} ₽`:`${symbols[currency]||''}${number}`;
}

// bestSale is where the item sells for the most roubles: the flea market's
// lowest offer or the best trader.
export function bestSale(item){
 const trader=item.traders[0];
 const flea=item.flea?.lastLow||0;
 if(flea>(trader?.priceRub||0))return {where:'flea',priceRub:flea};
 return trader?{where:trader.trader,priceRub:trader.priceRub}:null;
}

// age describes how long ago a timestamp was, in whole minutes or hours.
export function age(iso,language='ja',now=Date.now()){
 const at=Date.parse(iso);if(Number.isNaN(at))return '';
 const minutes=Math.max(0,Math.floor((now-at)/60000));
 // Past two days in days, past two months in months: an item off the flea
 // market keeps a price from long ago.
 const hours=Math.floor(minutes/60),days=Math.floor(hours/24),months=Math.floor(days/30);
 if(language==='ja')return minutes<1?'たった今':minutes<60?`${minutes}分前`:hours<48?`${hours}時間前`:days<60?`${days}日前`:`${months}か月前`;
 return minutes<1?'just now':minutes<60?`${minutes} min ago`:hours<48?`${hours} h ago`:days<60?`${days} days ago`:`${months} months ago`;
}

// itemPageURL is the item's page on a task site: tarkov.dev, the official
// wiki, or the Japanese wiki (which titles item pages by their English name).
export function itemPageURL(item,site){
 if(site==='official-wiki'&&item.wikiLink)return item.wikiLink;
 if(site==='japanese-wiki'&&item.name)return 'https://wikiwiki.jp/eft/'+encodeURIComponent(item.name);
 return item.link||item.wikiLink||'';
}

// historyPoints validates a price history from the Host: [{t,price,min}],
// oldest first.
export function historyPoints(raw){
 if(!Array.isArray(raw))return [];
 return raw.slice(-5000).map(p=>({t:Number(p?.t),price:count(p?.price),min:count(p?.min)})).filter(p=>Number.isFinite(p.t)&&p.t>0&&p.price>0).sort((a,b)=>a.t-b.t);
}

export const chartRanges={'7d':7*864e5,'30d':30*864e5,all:Infinity};

const percentile=(sorted,q)=>sorted[Math.min(sorted.length-1,Math.max(0,Math.round(q*(sorted.length-1))))];

// chartSeries picks the points of a range and the scale to draw them with.
// current (the latest lowest offer) extends the lowest-price line to now,
// since the history lags the live price by a few hours. The scale ignores
// the most extreme 2% at each end: single scans can be far off.
export function chartSeries(points,range,now=Date.now(),current=null){
 const span=chartRanges[range]??chartRanges['7d'];
 let pts=points.filter(p=>now-p.t<=span);
 if(current&&current.min>0&&current.t>(pts.at(-1)?.t??0)&&now-current.t<=span)pts=[...pts,{t:current.t,price:0,min:current.min,now:true}];
 if(pts.length<2)return null;
 const values=pts.flatMap(p=>[p.price,p.min]).filter(v=>v>0).sort((a,b)=>a-b);
 let lo=percentile(values,.02),hi=percentile(values,.98);
 if(hi<=lo){lo=lo*.95;hi=hi*1.05||1;}
 const pad=(hi-lo)*.08;
 const prices=pts.filter(p=>p.price>0);
 const first=prices[0]?.price||0,last=prices.at(-1)?.price||0;
 return {pts,lo:Math.max(0,lo-pad),hi:hi+pad,t0:pts[0].t,t1:pts.at(-1).t,
  high:Math.max(...prices.map(p=>p.price)),low:Math.min(...pts.map(p=>p.min).filter(v=>v>0)),
  change:first?(last-first)/first*100:0};
}

// chartPath draws one series in a width×height box; missing values break it.
export function chartPath(series,key,width,height){
 const {pts,lo,hi,t0,t1}=series;
 const x=t=>((t-t0)/Math.max(1,t1-t0))*width;
 const y=v=>height-((Math.min(hi,Math.max(lo,v))-lo)/Math.max(1,hi-lo))*height;
 let d='',pen=false;
 for(const p of pts){
  const v=p[key];
  if(!(v>0)){pen=false;continue;}
  d+=`${pen?'L':'M'}${x(p.t).toFixed(1)} ${y(v).toFixed(1)}`;pen=true;
 }
 return d;
}
