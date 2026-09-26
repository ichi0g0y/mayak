import {age} from './item.js';
import {t,state,esc,option,icon,clickHandlers,action} from './shell-core.js';

// The bosses of the current map in the sidebar, the bosses page and the
// Goons report dialog.

// Sidebar section with the bosses: where the Goons were last reported and the
// bosses of the map being played; its icon opens the boss page.
// The heading opens and closes the section, back to how it was open; a row
// under the Goons shows or hides the map's bosses.
let bossOpenView='full';
// The map and the game mode, by hand or (the first choice) following the game.
function bossPick(info){
 const current=info.maps.find(m=>m.key===info.current),mode=info.mode==='pve'?'PvE':'PvP';
 const maps=[['',current?`${t('bossAuto')} (${current.name})`:t('bossAuto')],...info.maps.filter(m=>m.bosses.length).map(m=>[m.key,m.name])];
 const modes=[['',state.bossMode?t('bossAuto'):`${t('bossAuto')} (${mode})`],['regular','PvP'],['pve','PvE']];
 return `<div class="boss-pick"><select data-scope="preferences" data-key="bossMap" aria-label="${esc(t('bossMaps'))}">${maps.map(([v,l])=>option(v,l,state.bossMap)).join('')}</select><select data-scope="preferences" data-key="bossMode" aria-label="${esc(t('bossModeLabel'))}">${modes.map(([v,l])=>option(v,l,state.bossMode)).join('')}</select></div>`;
}
// The Goons report: the map, the account and mode (from the EFT logs) and the
// raid; nothing is sent before the user confirms. goonDraft keeps what was
// chosen across re-renders.
let goonDraft={};
function goonReportDialog(info){
 const r=state.goonReport,raid=r.raid;
 const close=`<button data-action="goonReportClose">${esc(t(r.done?'close':'cancel'))}</button>`;
 if(r.done)return `<div class="goon-dialog-backdrop"><div class="goon-dialog" role="dialog" aria-label="${esc(t('goonReport'))}"><h2>${esc(t('goonReport'))}</h2><p>${esc(t('goonReported'))}</p><div class="actions">${close}</div></div></div>`;
 const maps=info.maps.filter(m=>m.bosses.some(b=>b.id==='bossKnight'));
 const mapKey=goonDraft.map||(raid&&maps.some(m=>m.key===raid.map)?raid.map:maps.some(m=>m.key===(state.bossMap||info.current))?state.bossMap||info.current:maps[0]?.key||'');
 const modeName=mode=>mode==='pve'?'PvE':'PvP';
 const ids=r.identities.map(id=>({value:id.accountId+'|'+id.mode,label:[id.accountId,modeName(id.mode),id.current?t('goonCurrent'):'',id.lastSeen?`${t('goonLastSeen')} ${new Date(id.lastSeen).toLocaleDateString(state.language==='ja'?'ja-JP':'en-US')}`:''].filter(Boolean).join(' · ')}));
 const raidID=raid?raid.accountId+'|'+raid.mode:'';
 const account=goonDraft.account||(ids.some(i=>i.value===raidID)?raidID:ids[0]?.value||'');
 const canRaid=raid&&!raid.reported;
 const when=goonDraft.when??(canRaid?raid.startedAt:'');
 const raidLine=raid?`${raid.active?t('goonRaidNow'):t('goonRaidLast')}: ${maps.find(m=>m.key===raid.map)?.name||raid.map} · ${clock(raid.startedAt)} ${t('goonStarted')}${raid.reported?' · '+t('goonAlready'):''}`:'';
 return `<div class="goon-dialog-backdrop"><form class="goon-dialog" id="goon-form" role="dialog" aria-label="${esc(t('goonReport'))}"><h2>${esc(t('goonReport'))}</h2>
 <p class="hint">${esc(t('goonReportOnly'))}</p>
 <label class="field"><span>${esc(t('shotInfoMap'))}</span><select id="goon-map">${maps.map(m=>option(m.key,m.name,mapKey)).join('')}</select></label>
 <label class="field"><span>${esc(t('goonAccount'))}</span>${ids.length?`<select id="goon-account">${ids.map(i=>option(i.value,i.label,account)).join('')}</select>`:`<p class="notice">${esc(t('goonNoAccount'))}</p>`}</label>
 <fieldset class="goon-when"><legend>${esc(t('goonWhen'))}</legend>${raid?`<label class="check"><input type="radio" name="goon-when" value="${esc(raid.startedAt)}" ${when===raid.startedAt?'checked':''} ${canRaid?'':'disabled'}>${esc(raidLine)}</label>`:''}<label class="check"><input type="radio" name="goon-when" value="" ${when===''?'checked':''}>${esc(t('goonWhenNow'))}</label></fieldset>
 <p class="goon-consent">${esc(t('goonConsent'))}</p>
 ${r.error?`<p class="notice" role="alert">${esc(r.error)}</p>`:''}
 <div class="actions">${close}<button class="primary" data-action="goonReportSend" ${!ids.length||!maps.length||r.busy?'disabled':''}>${esc(t(r.busy?'goonSending':'goonSend'))}</button></div></form></div>`;
}
const bossName=b=>b.id==='bossKnight'?'Goons':b.name;
const pct=value=>Math.round((Number(value)||0)*100)+'%';
// How fresh a Goons report is: within an hour (a raid or two), three hours,
// or older.
export function goonFresh(time){const minutes=(Date.now()-Date.parse(time))/60000;return !(minutes>=0)?'old':minutes<=60?'new':minutes<=180?'recent':'old';}
const goonLabel=goon=>goon?`Goons · ${goon.map} · ${age(goon.time,state.language)}`:`Goons · ${t('noGoons')}`;
export function bossSection(){
 const info=state.bosses;
 const open=state.tabs.find(tab=>tab.id===state.active)?.kind==='bosses';
 const view=state.bossesView,goon=info?.goons[0];
 let body='';
 if(view!=='closed'){
  const latest=!info?`<p class="shot-empty">${esc(t('bossesLoading'))}</p>`:goon?`<button class="goon-latest" data-action="bosses" data-fresh="${goonFresh(goon.time)}" title="${esc(goonLabel(goon))}"><span class="goon-dot"></span><span class="goon-name">Goons</span><span class="goon-age" data-goon-time="${esc(goon.time)}">${esc(age(goon.time,state.language))}</span><b>${esc(goon.map)}</b></button>`:`<p class="shot-empty">Goons · ${esc(t('noGoons'))}</p>`;
  const map=info?.maps.find(m=>m.key===(state.bossMap||info.current));
  const here=!info?'':`<div class="boss-here">${bossPick(info)}${!map?`<p class="shot-empty">${esc(t('bossPickHint'))}</p>`:map.bosses.length?map.bosses.slice(0,5).map(b=>`<div class="boss-line"><span>${esc(bossName(b))}</span><b>${pct(b.chance)}</b></div>`).join(''):`<p class="shot-empty">${esc(t('noBosses'))}</p>`}</div>`;
  body=`<div class="boss-section">${latest}<button class="boss-more" data-action="bossDetail" aria-expanded="${view==='full'}">${esc(t('bossMapBosses'))}${icon('chevron','section-chevron')}</button>${view==='full'?here:''}</div>`;
 }
 return `<div class="section-label boss-section-label ${open?'active':''}"><button class="section-link" data-action="toggleBossSection" aria-expanded="${view!=='closed'}" title="${esc(t(view==='closed'?'expandSection':'collapseSection'))}">${esc(t('bosses'))}${icon('chevron','section-chevron')}</button><span class="section-actions"><button class="new-tab goon-report-open" data-action="goonReportFromSidebar" title="${esc(t('goonReport'))}" aria-label="${esc(t('goonReport'))}">${icon('flag')}</button><button class="new-tab bosses-open" data-action="bosses" title="${esc(info?goonLabel(goon):t('allBosses'))}" aria-label="${esc(t('allBosses'))}" aria-pressed="${open}">${icon('skull')}</button></span></div>${body}`;
}
// The boss page: the Goons reports and the maps they spawn on, then the
// bosses of one map (the one being played, or the one chosen).
export function bossesPage(){
 const info=state.bosses;
 if(!info)return `<div class="page bosses-page"><p class="empty-tabs">${esc(t('bossesLoading'))}</p></div>`;
 const goons=info.goons.slice(0,10).map((g,i)=>`<li class="${i===0?'latest':''}" data-fresh="${goonFresh(g.time)}"><span class="goon-dot"></span><b>${esc(g.map)}</b><span class="goon-age" data-goon-time="${esc(g.time)}">${esc(age(g.time,state.language))}</span><time>${esc(clock(g.time))}</time></li>`).join('');
 const goonMaps=info.maps.map(m=>({m,b:m.bosses.find(b=>b.id==='bossKnight')})).filter(x=>x.b).sort((a,b)=>b.b.chance-a.b.chance).map(({m,b})=>`<button class="boss-chip" data-action="bossMap" data-id="${esc(m.key)}">${esc(m.name)} <b>${pct(b.chance)}</b></button>`).join('');
 const withBosses=info.maps.filter(m=>m.bosses.length);
 const chosen=withBosses.find(m=>m.key===(state.bossMap||info.current))||withBosses[0];
 const maps=withBosses.map(m=>`<button class="${m===chosen?'selected':''}" data-action="bossMap" data-id="${esc(m.key)}" aria-pressed="${m===chosen}">${esc(m.name)}${m.key===info.current?' ●':''}</button>`).join('');
 const cards=chosen?chosen.bosses.map(b=>`<article class="boss-card"><header>${b.portrait?`<img src="${esc(b.portrait)}" alt="" referrerpolicy="no-referrer" loading="lazy">`:`<span class="boss-portrait">${icon('skull')}</span>`}<h3>${esc(bossName(b))}${b.id==='bossKnight'?`<small>${esc([b.name,...b.escorts.map(e=>e.name)].join(' · '))}</small>`:''}</h3><strong>${pct(b.chance)}</strong></header>${b.locations.length?`<ul class="boss-places">${b.locations.map(l=>`<li><span>${esc(l.name)}</span><b>${pct(l.chance)}</b></li>`).join('')}</ul>`:''}${b.escorts.length?`<p class="boss-escorts">${esc(t('escorts'))}: ${b.escorts.map(e=>esc(e.name)+' ×'+(e.min===e.max?e.max:e.min+'–'+e.max)).join(', ')}</p>`:''}</article>`).join(''):'';
 return `<div class="page bosses-page"><div class="bookmarks-head"><h1>${esc(t('bosses'))}</h1>${bossPick(info)}</div>
 <section class="panel goon-panel"><div class="goon-head"><h2>Goons</h2><button data-action="goonReportOpen">${icon('flag')}<span>${esc(t('goonReport'))}</span></button></div>${goons?`<ul class="goon-list">${goons}</ul>`:`<p class="shot-empty">${esc(t('noGoons'))}</p>`}<p class="hint">${esc(t('goonHint'))}</p>${goonMaps?`<h3 class="goon-maps-head">${esc(t('goonMaps'))}</h3><div class="boss-chips">${goonMaps}</div>`:''}</section>
 <section class="boss-maps"><div class="boss-map-tabs" role="group" aria-label="${esc(t('bossMaps'))}">${maps}</div><div class="boss-grid">${cards}</div></section></div>${state.goonReport?goonReportDialog(info):''}`;
}
const clock=time=>{const d=new Date(time);return Number.isNaN(d.getTime())?'':d.toLocaleTimeString(state.language==='ja'?'ja-JP':'en-US',{hour:'2-digit',minute:'2-digit',hour12:state.clock==='12'});};

document.addEventListener('change',event=>{const input=event.target;if(input.id==='goon-map')goonDraft.map=input.value;if(input.id==='goon-account')goonDraft.account=input.value;if(input.name==='goon-when')goonDraft.when=input.value;});
clickHandlers.push(async(type,id,button,event)=>{
  if(type==='toggleBossSection'){if(state.bossesView!=='closed')bossOpenView=state.bossesView;void action('preferences',{bossesView:state.bossesView==='closed'?bossOpenView:'closed'});return true;}
  if(type==='bossDetail'){void action('preferences',{bossesView:state.bossesView==='full'?'goons':'full'});return true;}
  if(type==='goonReportFromSidebar'){goonDraft={};await action('bosses');void action('goonReportOpen');return true;}
  if(type==='goonReportOpen'){goonDraft={};void action('goonReportOpen');return true;}
  if(type==='goonReportSend'){event.preventDefault();const [accountId,mode]=(document.querySelector('#goon-account')?.value||'').split('|');void action('goonReportSend',{map:document.querySelector('#goon-map')?.value||'',accountId,mode,startedAt:document.querySelector('input[name=goon-when]:checked')?.value||''});return true;}
  if(type==='bossMap'){void action('preferences',{bossMap:id});return true;}
  return false;
});
