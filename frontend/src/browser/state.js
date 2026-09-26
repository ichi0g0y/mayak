function randomUUID(){const b=crypto.getRandomValues(new Uint8Array(16));b[6]=(b[6]&15)|64;b[8]=(b[8]&63)|128;const h=[...b].map(x=>x.toString(16).padStart(2,"0")).join("");return `${h.slice(0,8)}-${h.slice(8,12)}-${h.slice(12,16)}-${h.slice(16,20)}-${h.slice(20)}`;}
import {DEFAULT_STUN} from './peer-code.js';
import {clampItemPanel,clampItemPanelHeight} from './item.js';
const sites = ['tarkov-dev','official-wiki','japanese-wiki'];
function webURL(value) {
  try { const url = new URL(value); return ['https:','http:'].includes(url.protocol) && !url.username && !url.password && url.hostname.toLowerCase()!=='wails.localhost' ? url.href : null; } catch { return null; }
}
// The Host injects ?connection=<ID> into tarkov.dev map pages to enable Remote
// Control. Tabs keep the plain URL so map tabs are still matched and reused.
function pageURL(value) {
  const href = webURL(value); if(!href) return null;
  const url = new URL(href); if(url.hostname!=='tarkov.dev'||!url.searchParams.has('connection')) return href;
  url.searchParams.delete('connection'); return url.href;
}
// Color themes defined in ../themes.css. "system" follows the OS light/dark mode.
const themes = ['system','mayak-dark','mayak-light','catppuccin-latte','catppuccin-frappe','catppuccin-macchiato','catppuccin-mocha','nord','dracula','gruvbox-dark','tokyo-night','solarized-dark','solarized-light'];
// Revision 1 added bookmarks; revision 2 moved TarkovTracker to .org, the site
// the tracker API integration (internal/tracker) uses.
const bookmarkRevision = 2;
const trackerHome = 'https://tarkovtracker.org/';
const addedBookmarks = [
  {id:'tarkov-market',name:'Tarkov Market',url:'https://tarkov-market.com/',group:'items'},
  {id:'database-for-tarkov',name:'Database for Tarkov',url:'https://db4tarkov.com/',group:'items'},
  {id:'eft-ammo',name:'EFT Ammo',url:'https://www.eft-ammo.com/',group:'items'},
];
const defaultBookmarks = [
  {id:'maps',name:'tarkov.dev — Maps',url:'https://tarkov.dev/maps/',group:'maps'},
  {id:'tasks-ja',name:'日本語Wiki — タスク',url:'https://wikiwiki.jp/eft/%E3%82%BF%E3%82%B9%E3%82%AF',group:'tasks'},
  {id:'tasks-en',name:'Official Wiki — Quests',url:'https://escapefromtarkov.fandom.com/wiki/Quests',group:'tasks'},
  {id:'items',name:'tarkov.dev — Items',url:'https://tarkov.dev/items/',group:'items'},
  {id:'tracker',name:'TarkovTracker',url:trackerHome,group:'progress'},
  ...addedBookmarks,
];
// The fixed map tab always leads the tab list. It cannot be closed, pinned or
// moved, and every map detection lands in it so Remote Control stays attached.
const mapTabID = 'map';
// A fixed view's home is where its home button returns after links led away:
// the last detected map, or the site's start page.
function mapTab(url,home) { return {id:mapTabID,kind:'web',url:webURL(url)||'https://tarkov.dev/maps/',home:webURL(home)||'https://tarkov.dev/maps/',title:'tarkov.dev',pinned:false,role:'map',fixed:true}; }
// TarkovTracker is the second fixed view, right after the map.
const trackerTabID = 'tracker';
function trackerTab(url) { return {id:trackerTabID,kind:'web',url:webURL(url)||trackerHome,home:trackerHome,title:'TarkovTracker',pinned:false,role:'tracker',fixed:true}; }
const fixedTabIDs = [mapTabID,trackerTabID];
// Favicons by hostname, learned from visited pages and shown for bookmarks.
const maxFavicons=200;
function restoreFavicons(raw) {
  if(!raw||typeof raw!=='object')return {};
  return Object.fromEntries(Object.entries(raw).filter(([host,url])=>/^[a-z0-9.-]{1,253}$/.test(host)&&webURL(url)).slice(-maxFavicons));
}
function rememberFavicon(state,pageURL,favicon) {
  const url=webURL(favicon),host=hostname(pageURL);
  if(!url||!host||state.favicons[host]===url)return;
  delete state.favicons[host];state.favicons[host]=url;
  const hosts=Object.keys(state.favicons);
  for(const old of hosts.slice(0,Math.max(0,hosts.length-maxFavicons)))delete state.favicons[old];
}
function hostname(value) {try{return new URL(value).hostname.toLowerCase().replace(/^www\./,'');}catch{return '';}}
// Pages translated by Google Translate's proxy: the host's dots become
// hyphens (hyphens doubled) under translate.goog, and _x_tr_* parameters
// name the languages. The built-in browser has no Chrome translation of its
// own (WebView2 lacks it), so a page is translated by opening it there.
const translateSuffix='.translate.goog';
function translatedURL(value,language) {
  const url=webURL(value);if(!url)return null;
  const u=new URL(url);if(u.hostname.endsWith(translateSuffix))return url;
  u.hostname=u.hostname.replace(/-/g,'--').replace(/\./g,'-')+translateSuffix;
  u.searchParams.set('_x_tr_sl','auto');u.searchParams.set('_x_tr_tl',language==='en'?'en':'ja');u.searchParams.set('_x_tr_hl',language==='en'?'en':'ja');
  return u.href;
}
function originalURL(value) {
  const url=webURL(value);if(!url)return null;
  const u=new URL(url);if(!u.hostname.endsWith(translateSuffix))return url;
  u.hostname=u.hostname.slice(0,-translateSuffix.length).replace(/--|-/g,m=>m==='--'?'\u0000':'.').replace(/\u0000/g,'-');
  for(const key of [...u.searchParams.keys()])if(key.startsWith('_x_tr_'))u.searchParams.delete(key);
  return u.href;
}
const isTranslated=value=>{try{return new URL(value).hostname.endsWith(translateSuffix);}catch{return false;}};
// The item sidebar reopens after a restart with the item it showed.
function restoreItemPanel(raw){
 const id=typeof raw?.id==='string'&&/^[0-9a-f]{24}$/.test(raw.id)?raw.id:'';
 return {open:raw?.open===true,id,mode:id&&['regular','pve','pvp-season'].includes(raw.mode)?raw.mode:''};
}
// The sidebar can be resized between these widths (logical pixels).
const sidebarWidths={min:180,max:420,default:224};
const clampSidebar=value=>Number.isFinite(value)?Math.round(Math.min(sidebarWidths.max,Math.max(sidebarWidths.min,value))):sidebarWidths.default;
function defaults() { return {version:1,bookmarkRevision,language:'ja',tutorialDone:false,clock:'24',layout:'vertical',sidebarSide:'left',sidebarCollapsed:false,bookmarksCollapsed:false,screenshotsCollapsed:false,bossesView:'full',bossMap:'',bossMode:'',sidebarWidth:224,itemPanelWidth:320,itemPanelHeight:280,itemDock:'right',itemPanel:{open:false,id:'',mode:''},favicons:{},bookmarkView:'grid',theme:'mayak-dark',adblock:true,taskMode:'new',questSite:'host',translateWiki:false,connection:{mode:'local',stun:DEFAULT_STUN},bookmarks:structuredClone(defaultBookmarks),tabs:[mapTab(),trackerTab(),{id:'settings',kind:'settings'}],active:mapTabID}; }
function restore(raw={}) {
  const state=defaults();
  state.bookmarkRevision=bookmarkRevision;
  // The first-run tutorial (shell.js) shows until it is finished or skipped.
  state.tutorialDone=raw.tutorialDone===true;
  state.language=raw.language==='en'?'en':'ja'; state.layout=raw.layout==='horizontal'?'horizontal':'vertical';state.sidebarSide=raw.sidebarSide==='right'?'right':'left';
  // The settings choose the tab sidebar's place in one: left, right or top.
  if(['left','right','top'].includes(raw.navPosition)){state.layout=raw.navPosition==='top'?'horizontal':'vertical';if(raw.navPosition!=='top')state.sidebarSide=raw.navPosition;}
  state.taskMode=raw.taskMode==='reuse'?'reuse':'new';state.adblock=raw.adblock!==false;const theme={'claude-dark':'mayak-dark','claude-light':'mayak-light'}[raw.theme]||raw.theme;state.theme=themes.includes(theme)?theme:'mayak-dark';state.clock=raw.clock==='12'?'12':'24';state.sidebarCollapsed=raw.sidebarCollapsed===true;state.bookmarksCollapsed=raw.bookmarksCollapsed===true;state.screenshotsCollapsed=raw.screenshotsCollapsed===true;state.bossesView=['full','goons','closed'].includes(raw.bossesView)?raw.bossesView:raw.bossesCollapsed===true?'closed':'full';state.bossMap=typeof raw.bossMap==='string'&&/^[a-z0-9-]{1,40}$/.test(raw.bossMap)?raw.bossMap:'';state.bossMode=['regular','pve'].includes(raw.bossMode)?raw.bossMode:'';state.sidebarWidth=clampSidebar(raw.sidebarWidth);state.itemPanelWidth=clampItemPanel(raw.itemPanelWidth);state.itemPanelHeight=clampItemPanelHeight(raw.itemPanelHeight);state.itemDock=['left','bottom'].includes(raw.itemDock)?raw.itemDock:'right';state.itemPanel=restoreItemPanel(raw.itemPanel);state.bookmarkView=raw.bookmarkView==='list'?'list':'grid';state.favicons=restoreFavicons(raw.favicons);
  state.questSite=['host',...sites].includes(raw.questSite)?raw.questSite:'host';
  state.translateWiki=raw.translateWiki===true;
  // The LAN receiving mode ("remote") is gone; a browser saved in it starts off.
  if (raw.connection && ['local','remote','webrtc','off'].includes(raw.connection.mode)) state.connection={mode:raw.connection.mode==='remote'?'off':raw.connection.mode,stun:typeof raw.connection.stun==='string'?raw.connection.stun:DEFAULT_STUN};
  if (Array.isArray(raw.bookmarks)) state.bookmarks=raw.bookmarks.filter(b=>b && webURL(b.url)).slice(0,100).map(b=>({id:String(b.id||randomUUID()),name:String(b.name||b.url).slice(0,150),url:webURL(b.url),group:bookmarkGroup(b.group),...(b.sidebar?{sidebar:true}:{})}));
  const revision=Number(raw.bookmarkRevision)||0;
  if (revision < 1) {
    for (const bookmark of addedBookmarks) {
      if (state.bookmarks.length < 100 && !state.bookmarks.some(b=>b.id===bookmark.id || b.url===bookmark.url)) state.bookmarks.push({...bookmark});
    }
  }
  if (revision < 2) for (const b of state.bookmarks) if (b.url==='https://tarkovtracker.io/') b.url=trackerHome;
  if (Array.isArray(raw.tabs)) {
    const ids=new Set();let settings=false,bookmarksPage=false,screenshotsPage=false,bossesPage=false,tabsPage=false;
    const savedMap=raw.tabs.find(t=>t?.id===mapTabID),savedTracker=raw.tabs.find(t=>t?.id===trackerTabID);
    state.tabs=raw.tabs.filter(t=>{
      if(!t || typeof t.id!=='string' || !/^[a-zA-Z0-9_-]{1,80}$/.test(t.id) || fixedTabIDs.includes(t.id) || ids.has(t.id))return false;
      if(t.kind==='settings'){if(settings)return false;settings=true;}
      else if(t.kind==='bookmarks'){if(bookmarksPage)return false;bookmarksPage=true;}
      else if(t.kind==='screenshots'){if(screenshotsPage)return false;screenshotsPage=true;}
      else if(t.kind==='bosses'){if(bossesPage)return false;bossesPage=true;}
      else if(t.kind==='tabs'){if(tabsPage)return false;tabsPage=true;}
      else if(t.kind!=='blank' && !(t.kind==='web' && webURL(t.url)))return false;
      ids.add(t.id);return true;
    }).slice(0,79).map(t=>({id:t.id,kind:t.kind,url:t.kind==='web'?webURL(t.url):undefined,title:String(t.title||'').slice(0,160),pinned:!!t.pinned,favicon:t.kind==='web'&&webURL(t.favicon)||undefined,role:t.role==='task'?'task':undefined,task:validTask(t.task)?t.task:undefined}));
    state.tabs.unshift(mapTab(savedMap?.kind==='web'?pageURL(savedMap.url):undefined,savedMap?.home&&pageURL(savedMap.home)),trackerTab(savedTracker?.kind==='web'&&hostname(savedTracker.url)!=='tarkovtracker.io'?savedTracker.url:undefined));
    orderTabs(state);
  }
  // The settings view is not restored as the active tab: a start lands on the map, not on settings.
  const restored=state.tabs.find(t=>t.id===raw.active);
  state.active=restored&&restored.kind!=='settings'?restored.id:state.tabs[0]?.id||'';
  return state;
}
function validTask(task) {return !!task && typeof task.id==='string' && task.id.length>0 && task.id.length<=128 && typeof task.name==='string' && task.name.length<=256 && task.urls && sites.every(s=>webURL(task.urls[s]));}
function receiveTask(state,task) {
  if(!validTask(task))return null;
  const site=state.questSite==='host'?(sites.includes(task.site)?task.site:'tarkov-dev'):state.questSite;
  // The official wiki (English) opens translated when asked to.
  const url=site==='official-wiki'&&state.translateWiki?translatedURL(task.urls[site],state.language):webURL(task.urls[site]);
  // Reconnect/repeated screenshots focus the same task, without a tab explosion.
  let tab=state.tabs.find(t=>t.kind==='web' && t.task?.id===task.id && t.url===url);
  if(!tab && state.taskMode==='reuse')tab=state.tabs.find(t=>t.role==='task'&&!t.pinned);
  if(!tab){if(state.tabs.length>=80)return null;tab={id:randomUUID(),kind:'web',role:'task'};state.tabs.push(tab);}
  Object.assign(tab,{url,title:task.name,task:structuredClone(task)});state.active=tab.id;return tab;
}
function receiveMap(state,name) {
  if(typeof name!=='string'||!name.match(/^[a-z0-9-]{1,60}$/))return null;
  if(name==='ground-zero-21')name='ground-zero';
  const url=`https://tarkov.dev/map/${name}`;
  let tab=state.tabs.find(t=>t.id===mapTabID);
  if(!tab){tab=mapTab();state.tabs.unshift(tab);}
  tab.kind='web';tab.url=url;tab.home=url;state.active=tab.id;return tab;
}
// Tab order: the fixed map, then pinned tabs, then the rest. Sorting is stable,
// so each group keeps its own order.
function orderTabs(state) {
  const rank=t=>t.fixed?0:t.pinned?1:2;
  state.tabs=state.tabs.map((t,i)=>[t,i]).sort((a,b)=>rank(a[0])-rank(b[0])||a[1]-b[1]).map(([t])=>t);
}
// Pinning moves a tab to the end of the pinned group; unpinning moves it to
// the top of the other tabs. Only web pages can be pinned.
function togglePin(state,id) {
  const item=state.tabs.find(t=>t.id===id&&t.kind==='web'&&!t.fixed);
  if(!item)return false;
  item.pinned=!item.pinned;
  state.tabs=state.tabs.filter(t=>t!==item);
  const firstUnpinned=state.tabs.findIndex(t=>!t.fixed&&!t.pinned);
  state.tabs.splice(firstUnpinned<0?state.tabs.length:firstUnpinned,0,item);
  return true;
}
// Moves tab id before the tab "before", or to the end when before is null.
// The fixed map tab never moves, and tabs stay within their pinned group.
function moveTab(state,id,before) {
  const item=state.tabs.find(t=>t.id===id&&!t.fixed);
  if(!item||id===before)return false;
  if(before!=null&&!state.tabs.some(t=>t.id===before&&!t.fixed))return false;
  state.tabs=state.tabs.filter(t=>t!==item);
  const index=before==null?state.tabs.length:state.tabs.findIndex(t=>t.id===before);
  state.tabs.splice(index,0,item);orderTabs(state);return true;
}
// Bookmark categories: the built-in keys are translated for display; anything
// else is a category the user typed.
const bookmarkGroups=['maps','tasks','items','progress','other'];
function bookmarkGroup(value) {
  const group=typeof value==='string'?value.trim().replace(/\s+/g,' ').slice(0,40):'';
  return group||'other';
}
// Bookmarks pinned to the sidebar keep their order in the bookmark list.
// Pinning with "before" (a pinned bookmark id, or null for the end) also
// places it there, which is how the sidebar is reordered by dragging.
function pinBookmark(state,id,pin,before) {
  const item=state.bookmarks.find(b=>b.id===id);
  if(!item)return false;
  if(!pin){delete item.sidebar;return true;}
  item.sidebar=true;
  if(before===undefined||before===id)return true;
  state.bookmarks=state.bookmarks.filter(b=>b!==item);
  const index=before==null?-1:state.bookmarks.findIndex(b=>b.id===before&&b.sidebar);
  if(index<0){const last=state.bookmarks.findLastIndex(b=>b.sidebar);state.bookmarks.splice(last+1,0,item);}
  else state.bookmarks.splice(index,0,item);
  return true;
}
// Bookmarks a web tab and pins it to the sidebar. A bookmark with the same URL
// is pinned instead of adding a duplicate.
function bookmarkTab(state,tabId,before) {
  const tab=state.tabs.find(t=>t.id===tabId&&t.kind==='web'&&webURL(t.url));
  if(!tab)return null;
  let item=state.bookmarks.find(b=>b.url===tab.url);
  if(!item){
    if(state.bookmarks.length>=100)return null;
    item={id:randomUUID(),name:String(tab.title||tab.url).slice(0,150),url:tab.url,group:'other'};
    state.bookmarks.push(item);
  }
  pinBookmark(state,item.id,true,before??null);
  return item;
}
// Returns a fixed view to its home page.
function goHome(state,id) {
  const tab=state.tabs.find(t=>t.id===id&&t.fixed);
  if(!tab||!webURL(tab.home))return false;
  tab.url=tab.home;return true;
}
function openLocal(state,kind) {let tab=state.tabs.find(t=>t.kind===kind);if(!tab){tab={id:randomUUID(),kind};state.tabs.push(tab);}state.active=tab.id;return tab;}
// Settings sections: the browser's own, then the Host's (its settings page in a
// frame, which shows the section named in its URL hash).
const browserSections=['appearance','tasks','adblock','connection','about'];
const hostSections=['status','logs','folders','recognition','remote','tracker','sounds','startup','debug'];
// The address bar takes a URL or a search. Anything that is not an address
// (a scheme, a host name with a dot or a port, an IPv4 address, localhost)
// is searched on Google, as in Chrome; "? words" always searches.
const searchURL=query=>'https://www.google.com/search?q='+encodeURIComponent(query);
const hostLike=/^(localhost|(\d{1,3}\.){3}\d{1,3}|([a-z0-9¡-￿-]+\.)+[a-z¡-￿]{2,}|[a-z0-9-]+:\d{1,5})(:\d{1,5})?([/?#].*)?$/i;
function resolveAddress(value) {
  const text=String(value||'').trim();
  if(!text)return null;
  if(text.startsWith('?')){const query=text.slice(1).trim();return query?searchURL(query):null;}
  if(/^https?:\/\//i.test(text))return webURL(text)||searchURL(text);
  if(/\s/.test(text))return searchURL(text);
  if(hostLike.test(text))return webURL('https://'+text)||searchURL(text);
  return searchURL(text);
}
// Chrome's tab shortcuts, from a key event (key as event.key, lower-cased)
// in the shell or forwarded from a page view. Returns what to do, or null.
function shortcut({key,ctrl,shift,alt}) {
  key=String(key||'').toLowerCase();
  if(!ctrl&&!shift&&(key==='f6'||(alt&&key==='d')))return {type:'address'};
  if(!ctrl||alt)return null;
  if(key==='t')return {type:shift?'reopenTab':'newTab'};
  if(key==='tab')return {type:shift?'prevTab':'nextTab'};
  if(shift)return null;
  if(key==='w'||key==='f4')return {type:'closeTab'};
  if(key==='pageup')return {type:'prevTab'};
  if(key==='pagedown')return {type:'nextTab'};
  if(key==='l')return {type:'address'};
  if(key==='d')return {type:'bookmark'};
  if(/^[1-9]$/.test(key))return {type:'tabAt',index:Number(key)};
  return null;
}
// Ctrl+1..8 pick the nth tab in the sidebar's order (the fixed views first),
// Ctrl+9 the last one; Ctrl+Tab and Ctrl+PageDown cycle through them.
function tabAt(state,index) {
  const tabs=state.tabs;if(!tabs.length)return null;
  const tab=index>=9?tabs[tabs.length-1]:tabs[index-1];
  if(!tab)return null;
  state.active=tab.id;return tab;
}
function cycleTab(state,delta) {
  const tabs=state.tabs;if(!tabs.length)return null;
  const current=Math.max(0,tabs.findIndex(t=>t.id===state.active));
  const tab=tabs[(current+delta+tabs.length)%tabs.length];
  state.active=tab.id;return tab;
}
// A detected position brings the map view forward. Its page is kept when it
// already shows that map (a navigation would lose the state tarkov.dev holds);
// another map, or another page, is opened like a detected map.
function receivePosition(state,name) {
  if(typeof name!=='string'||!name.match(/^[a-z0-9-]{1,60}$/))return null;
  if(name==='ground-zero-21')name='ground-zero';
  const tab=state.tabs.find(t=>t.id===mapTabID);
  if(tab&&tab.kind==='web'){
    let url=null;try{url=new URL(tab.url);}catch{}
    if(url&&url.hostname==='tarkov.dev'&&url.pathname.replace(/\/$/,'')===`/map/${name}`){state.active=tab.id;return tab;}
  }
  return receiveMap(state,name);
}
export {browserSections,hostSections,randomUUID,mapTabID,trackerTabID,themes,bookmarkGroups,bookmarkGroup,rememberFavicon,hostname,sidebarWidths,clampSidebar,defaults,restore,webURL,pageURL,receiveTask,receiveMap,moveTab,togglePin,pinBookmark,bookmarkTab,goHome,openLocal,sites,translatedURL,originalURL,isTranslated,resolveAddress,searchURL,shortcut,tabAt,cycleTab,receivePosition};
