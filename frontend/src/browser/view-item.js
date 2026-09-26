import {price,chartSeries,chartPath,itemPanelHeights,itemPanelWidths,itemPageURL,bestSale,age} from './item.js';
import {t,state,esc,icon,siteChoices,render,action,clickHandlers} from './shell-core.js';

// The item panel: details, prices and the price chart, the item search.

// The chart's range is a per-viewer convenience, remembered in this browser.
let chartRange='7d',chart=null;
try{chartRange=localStorage.getItem('mayak.chartRange')||'7d';}catch{}
if(!['7d','30d','all'].includes(chartRange))chartRange='7d';
// The plot keeps chartPad free above and below its lines; four grid lines
// with prices mark the scale.
const chartWidth=288,chartHeight=132,chartPad=14,chartTicks=4;
const shortPrice=v=>v>=1e6?`${(v/1e6).toFixed(v>=1e7?0:1)}M`:v>=1e3?`${Math.round(v/1e3)}k`:String(Math.round(v));
const chartDate=(t,withTime=false)=>new Date(t).toLocaleString(state.language==='ja'?'ja-JP':'en-US',chartRange==='all'&&!withTime?{year:'numeric',month:'numeric'}:withTime?{month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit',hour12:state.clock==='12'}:{month:'numeric',day:'numeric'});
// The flea market price history: average and lowest offer. The lowest line
// ends at the live price, since the history lags it by a few hours.
function itemChart(item){
 const h=state.itemHistory;chart=null;
 if(!item.flea||!h||h.id!==item.id)return '';
 const money=v=>esc(price(v,'RUB',state.language));
 const ranges=[['7d','range7d'],['30d','range30d'],['all','rangeAll']];
 const head=`<h3>${esc(t('priceHistory'))}<span class="chart-ranges" role="group">${ranges.map(([k,l])=>`<button data-action="chartRange" data-id="${k}" class="${chartRange===k?'selected':''}" aria-pressed="${chartRange===k}">${esc(t(l))}</button>`).join('')}</span></h3>`;
 const at=Date.parse(item.pricedAt);
 const s=h.points.length?chartSeries(h.points,chartRange,Date.now(),item.flea.lastLow?{t:Number.isNaN(at)?Date.now():at,min:item.flea.lastLow}:null):null;
 if(!s)return `<section class="item-section item-chart">${head}<p class="item-empty">${esc(t(h.loading?'chartLoading':h.failed?'chartFailed':'chartEmpty'))}</p></section>`;
 chart=s;
 const change=s.change;
 return `<section class="item-section item-chart">${head}
 <div class="chart-stats"><span>${esc(t('chartHigh'))} <b>${money(s.high)}</b></span><span>${esc(t('chartLow'))} <b>${money(s.low)}</b></span><span class="${change>0?'up':change<0?'down':''}">${change>0?'+':''}${change.toFixed(1)}%</span></div>
 <div class="chart-box">${chartGrid(s)}<svg class="price-chart" viewBox="0 0 ${chartWidth} ${chartHeight}" preserveAspectRatio="none" aria-hidden="true">${Array.from({length:chartTicks},(_,i)=>{const y=chartPad+(chartHeight-2*chartPad)*i/(chartTicks-1);return `<line class="chart-grid" x1="0" x2="${chartWidth}" y1="${y}" y2="${y}"/>`;}).join('')}<g transform="translate(0 ${chartPad})"><path class="chart-min" d="${chartPath(s,'min',chartWidth,chartHeight-2*chartPad)}"/><path class="chart-avg" d="${chartPath(s,'price',chartWidth,chartHeight-2*chartPad)}"/></g><line class="chart-cursor" x1="0" x2="0" y1="0" y2="${chartHeight}"/></svg><div class="chart-tip" hidden></div></div>
 <div class="chart-axis"><span>${esc(chartDate(s.t0))}</span><span class="chart-legend"><i class="avg"></i>${esc(t('avgLine'))}<i class="min"></i>${esc(t('minLine'))}</span><span>${esc(chartDate(s.t1))}</span></div></section>`;
}
// Price labels for the grid lines, in a gutter left of the plot and as HTML
// so the stretched SVG does not distort them. The top line is the highest
// price of the scale.
function chartGrid(s){
 return Array.from({length:chartTicks},(_,i)=>{
  const top=(chartPad+(chartHeight-2*chartPad)*i/(chartTicks-1))/chartHeight*100;
  const value=s.hi-(s.hi-s.lo)*i/(chartTicks-1);
  return `<span class="chart-label" style="top:${top.toFixed(2)}%">${esc(shortPrice(value))}</span>`;
 }).join('');
}
// Hovering the chart shows the nearest sample without re-rendering the page.
document.addEventListener('pointermove',event=>{
 const box=event.target.closest?.('.chart-box');
 if(!box||!chart)return;
 // Positions come from the plot, which starts after the price labels.
 const r=box.querySelector('.price-chart').getBoundingClientRect(),offset=r.left-box.getBoundingClientRect().left,frac=Math.min(1,Math.max(0,(event.clientX-r.left)/r.width));
 const t0=chart.t0,span=Math.max(1,chart.t1-t0),at=t0+frac*span;
 let p=chart.pts[0];for(const q of chart.pts)if(Math.abs(q.t-at)<Math.abs(p.t-at))p=q;
 const x=(p.t-t0)/span;
 const line=box.querySelector('.chart-cursor');line.setAttribute('x1',x*chartWidth);line.setAttribute('x2',x*chartWidth);line.style.opacity=1;
 const tip=box.querySelector('.chart-tip'),money=v=>price(v,'RUB',state.language);
 tip.textContent=`${p.now?t('chartNow'):chartDate(p.t,true)} · ${p.price?`${t('avgLine')} ${money(p.price)} / `:''}${t('minLine')} ${money(p.min)}`;
 tip.hidden=false;
 tip.style.left=`${Math.min(Math.max(0,offset+x*r.width-tip.offsetWidth/2),box.clientWidth-tip.offsetWidth)}px`;
});
document.addEventListener('pointerout',event=>{
 const box=event.target.closest?.('.chart-box');
 if(!box||box.contains(event.relatedTarget))return;
 box.querySelector('.chart-tip').hidden=true;box.querySelector('.chart-cursor').style.opacity=0;
});
const modeNames={regular:'PvP',pve:'PvE','pvp-season':'Season'};
// The item sidebar: a search box, then the item (found or recognized):
// prices first (what to do with the item now), then what still needs it.
// Completed tasks and hideout levels sink to the bottom.
export function itemPanel(){
 if(!state.itemOpen)return '';
 const bottom=state.itemDock==='bottom';
 return `<aside class="item-panel" aria-label="${esc(t('itemPanel'))}"><div class="item-resizer" role="separator" aria-orientation="${bottom?'horizontal':'vertical'}" aria-valuemin="${bottom?itemPanelHeights.min:itemPanelWidths.min}" aria-valuemax="${bottom?itemPanelHeights.max:itemPanelWidths.max}" aria-valuenow="${bottom?state.itemPanelHeight:state.itemPanelWidth}" title="${esc(t('resizeSidebar'))}"></div><div class="item-titlebar"><button class="item-button" data-action="itemClose" title="${esc(t('itemHide'))}" aria-label="${esc(t('itemHide'))}">${icon({left:'panelLeft',bottom:'panelBottom'}[state.itemDock]||'panelRight')}</button><button class="item-button ${itemSearchOpen?'selected':''}" data-action="itemSearchToggle" title="${esc(t('itemSearch'))}" aria-label="${esc(t('itemSearch'))}" aria-expanded="${itemSearchOpen}">${icon('search')}</button></div>${itemSearchOpen?itemSearchPopup():''}<div class="item-body"><div class="item-flow">${state.item?itemDetails():`<p class="item-start">${esc(t('itemSearchHint'))}</p>`}</div></div></aside>`;
}
// The search popup floats over the item, so the item does not move.
let itemSearchOpen=false;
function itemSearchPopup(){
 const search=state.itemSearch||{query:'',results:[]};
 const results=search.query.trim()?(search.results.length?`<ul class="item-results">${search.results.map(r=>`<li><button data-action="itemSelect" data-id="${esc(r.id)}" title="${esc(r.name)}">${r.iconUrl?`<img src="${esc(r.iconUrl)}" alt="" referrerpolicy="no-referrer" loading="lazy">`:`<span class="result-icon">${icon('package')}</span>`}<span class="need-name">${esc(itemName(r))}<small>${esc(r.shortName)}</small></span></button></li>`).join('')}</ul>`:`<p class="item-empty">${esc(t('itemNoResults'))}</p>`):'';
 return `<div class="item-search-pop" role="dialog" aria-label="${esc(t('itemSearch'))}"><label class="item-search">${icon('search')}<input id="item-search" type="search" autocomplete="off" spellcheck="false" placeholder="${esc(t('itemSearch'))}" aria-label="${esc(t('itemSearch'))}" value="${esc(search.query)}"></label>${results}</div>`;
}
// The item's page opens on the task site, like the item's tasks.
// A name in the shell's language, when the catalog has one (see
// internal/locale); English otherwise.
const localName=(names,english)=>names?.[state.language]||english;
const itemName=item=>localName(item.names,item.name);
// The item's icon and name open its page, in the popup.
function itemPageLink(item){
 const site=state.localHost&&state.connection.mode==='local'?state.hostQuestSite:state.questSite==='host'?item.questSite:state.questSite;
 const url=itemPageURL(item,site);
 const label=siteChoices().find(([key])=>key===site)?.[1]||'tarkov.dev';
 return url?{url,label:`${t('openItemPage')} · ${label}`}:null;
}
function itemDetails(){
 const item=state.item;
 const lang=state.language,slots=item.width*item.height,best=bestSale(item),flea=item.flea;
 const money=(v,c='RUB')=>esc(price(v,c,lang));
 const row=(label,value,cls='')=>`<div class="item-row ${cls}"><span>${esc(label)}</span><b>${value}</b></div>`;
 const change=flea?.changePercent||0;
 const done=s=>s.state==='completed'||s.complete===true;
 // Rows with an action (tasks) are buttons that open their page.
 const needs=(rows,render,action)=>rows.length?`<ul class="item-needs">${[...rows].sort((a,b)=>done(a)-done(b)).map(s=>{const inner=`${render(s)}${s.foundInRaid?`<span class="fir" title="Found in raid">${t('fir')}</span>`:''}<span class="need-count">×${s.count.toLocaleString()}</span>${done(s)?`<span class="need-done" title="${esc(t('needDone'))}">${icon('check')}</span>`:''}`;return `<li class="${done(s)?'done':''}">${action?`<button class="need-row ${state.popup?.key==='task:'+s.id?'popup-source':''}" data-action="${action}" data-id="${esc(s.id)}" title="${esc(t('openTask'))}">${inner}</button>`:`<div class="need-row">${inner}</div>`}</li>`;}).join('')}</ul>`:`<p class="item-empty">${t('noNeeds')}</p>`;
 const open=rows=>rows.filter(s=>!done(s)).length;
 const unknown=item.tasks.some(s=>!s.state)||item.hideout.some(s=>s.complete===null);
 return `
 <div class="item-head">${(()=>{const link=itemPageLink(item),inner=`${item.iconUrl?`<img class="item-icon" src="${esc(item.iconUrl)}" alt="" referrerpolicy="no-referrer">`:`<span class="item-icon">${icon('package')}</span>`}<span class="item-title"><h2 title="${esc(item.name)}">${esc(itemName(item))}</h2><span>${esc(localName(item.shortNames,item.shortName))} · ${item.width}×${item.height} · ${esc(modeNames[item.mode]||item.mode)}</span></span>`;return link?`<button class="item-page-link ${state.popup?.key==='item:'+item.id?'popup-source':''}" data-action="itemPage" data-id="${esc(link.url)}" title="${esc(link.label)}">${inner}</button>`:`<div class="item-page-link">${inner}</div>`;})()}</div>
 <p class="item-fresh ${item.live?'live':''}"><span class="fresh-dot"></span>${esc(item.live?t('itemLive'):t('itemCatalog'))} · ${esc(t('itemPriced'))} <span class="item-age" data-time="${esc(item.pricedAt)}">${esc(age(item.pricedAt,lang))}</span><button class="item-refresh" data-action="itemRefresh" title="${esc(t('itemRefresh'))}" aria-label="${esc(t('itemRefresh'))}" aria-busy="${!!state.itemBusy}">${icon('reload',state.itemBusy?'icon spin':'icon')}</button></p>
 ${best?`<div class="item-best"><span>${esc(t('bestSale'))} · ${esc(best.where==='flea'?t('flea'):best.where)}</span><strong>${money(best.priceRub)}</strong><small>${money(best.priceRub/slots)} / ${esc(t('perSlot'))}</small></div>`:''}
 ${itemChart(item)}
 <section class="item-section"><h3>${esc(t('flea'))}</h3>${flea?row(t('fleaLow'),money(flea.lastLow))+row(t('fleaAvg'),money(flea.avg24h))+(flea.low24h&&flea.high24h?row(t('fleaRange'),`${money(flea.low24h)} – ${money(flea.high24h)}`):'')+row(t('fleaChange'),`${change>0?'+':''}${change.toFixed(1)}%`,change>0?'up':change<0?'down':'')+row(t('fleaOffers'),flea.offers.toLocaleString())+(flea.minLevel?row(t('fleaLevel'),'Lv.'+flea.minLevel):''):`<p class="item-empty">${t('noFlea')}</p>`}</section>
 <section class="item-section"><h3>${esc(t('traders'))}</h3>${item.traders.map((s,i)=>row(s.trader,s.currency==='RUB'?money(s.price):`${money(s.price,s.currency)} <small>(${money(s.priceRub)})</small>`,i===0?'best':'')).join('')||`<p class="item-empty">${t('noNeeds')}</p>`}</section>
 <section class="item-section"><h3>${esc(t('itemTasks'))}<span class="count">${open(item.tasks)}</span></h3>${needs(item.tasks,s=>`<span class="need-name" title="${esc(s.name)}">${esc(localName(s.names,s.name))}<small>${esc(s.trader)}</small></span>`,'itemTask')}</section>
 <section class="item-section"><h3>${esc(t('itemHideout'))}<span class="count">${open(item.hideout)}</span></h3>${needs(item.hideout,s=>`<span class="need-name">${esc(s.station)}<small>Lv.${s.level}</small></span>`)}</section>
 ${unknown&&(item.tasks.length||item.hideout.length)?`<p class="hint item-hint">${esc(t('progressUnknown'))}</p>`:''}
 `;
}
export function itemToggle(){
 const label=t(state.itemOpen?'itemHide':'itemShow');
 return `<button class="dock-button item-toggle ${state.itemOpen?'selected':''}" data-action="${state.itemOpen?'itemClose':'itemOpen'}" title="${esc(label)}" aria-label="${esc(label)}" aria-pressed="${state.itemOpen}">${icon('package')}</button>`;
}
setInterval(()=>{if(state?.itemOpen&&document.visibilityState==='visible')void action('itemRefresh',{auto:true});},180000);

let itemSearchTimer=0;
function openItemSearch(){itemSearchOpen=true;render();const input=document.querySelector('#item-search');input?.focus();input?.select();}
function closeItemSearch(){if(!itemSearchOpen)return;itemSearchOpen=false;clearTimeout(itemSearchTimer);void action('itemSearch','');}
// A click outside the popup closes it.
document.addEventListener('pointerdown',event=>{if(itemSearchOpen&&!event.target.closest('.item-search-pop,[data-action="itemSearchToggle"]'))closeItemSearch();},true);
document.addEventListener('keydown',event=>{
 if(event.target.id!=='item-search')return;
 if(event.key==='Enter'){const first=state.itemSearch?.results?.[0];if(first){event.preventDefault();clearTimeout(itemSearchTimer);itemSearchOpen=false;void action('itemSelect',first.id);}}
 else if(event.key==='Escape'){event.preventDefault();closeItemSearch();}
});

document.addEventListener('input',event=>{if(event.target.id==='item-search'){const value=event.target.value;clearTimeout(itemSearchTimer);itemSearchTimer=setTimeout(()=>void action('itemSearch',value),150);}});
clickHandlers.push(async(type,id,button,event)=>{
  if(type==='chartRange'){chartRange=id;try{localStorage.setItem('mayak.chartRange',id);}catch{}render();return true;}
  // The popup opens next to the row clicked.
  if(type==='itemTask'||type==='itemPage'){const r=button.getBoundingClientRect();const at={anchor:r.top+r.height/2};void action(type,type==='itemTask'?{id,...at}:{url:id,...at});return true;}
  if(type==='itemSearchToggle'){if(itemSearchOpen)closeItemSearch();else openItemSearch();return true;}
  if(type==='itemSelect'){itemSearchOpen=false;void action('itemSelect',id);return true;}
  // Opened without an item, the sidebar starts with the search.
  if(type==='itemOpen'&&!state.item){await action('itemOpen');openItemSearch();return true;}
  return false;
});
