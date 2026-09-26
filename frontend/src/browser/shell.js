import './api.js';
import './style.css';
import {hostname,bookmarkGroups,browserSections,hostSections,themes,sidebarWidths,isTranslated,originalURL,webURL,resolveAddress,shortcut,clampSidebar} from './state.js';
import {Window,Browser} from '@wailsio/runtime';
import morphdom from 'morphdom';
import {tabDragActive,installTabDrag} from './tab-drag.js';
import {words} from './words.js';
import {age,clampItemPanelHeight,clampItemPanel,itemPanelHeights,itemPanelWidths} from './item.js';
import {state,api,t,esc,icon,action,appVersion,select,siteChoices,option,clickHandlers,setState,loadVersion,setRender} from './shell-core.js';
import {goonFresh,bossSection,bossesPage} from './view-bosses.js';
import {shotBadges,screenshotsPage} from './view-screenshots.js';
import {itemToggle,itemPanel} from './view-item.js';
import {tutorialOpen,tutorialHTML,handleTutorial,openTutorial} from './view-tutorial.js';
setRender(render);

// The changelog page, rendered from CHANGELOG.md in the repository.
const CHANGELOG_URL='https://mayak.ich.sh/changelog';
let editingBookmark=null,bookmarkQuery='',tabQuery='',contextMenu=null,placeMenu=null;
const peerDrafts={offer:'',answer:'',pairCode:''};
let copied=false,windowTheme='',renderDeferred=false;
// The tutorial is offered once, when the first state arrives.
let tutorialStarted=false;
const themeNames={'mayak-dark':'MAYAK Dark','mayak-light':'MAYAK Light','catppuccin-latte':'Catppuccin Latte','catppuccin-frappe':'Catppuccin Frappé','catppuccin-macchiato':'Catppuccin Macchiato','catppuccin-mocha':'Catppuccin Mocha',nord:'Nord',dracula:'Dracula','gruvbox-dark':'Gruvbox Dark','tokyo-night':'Tokyo Night','solarized-dark':'Solarized Dark','solarized-light':'Solarized Light'};
const lightScheme=matchMedia('(prefers-color-scheme: light)');
// Applies the theme to the shell and to the same-origin Host settings frame.
function applyTheme(){
 const theme=state.theme==='system'?(lightScheme.matches?'mayak-light':'mayak-dark'):state.theme;
 document.documentElement.dataset.theme=theme;
 try{const frame=document.querySelector('#host-settings')?.contentDocument;if(frame){frame.documentElement.dataset.theme=theme;frame.documentElement.dataset.clock=state.clock;frame.documentElement.lang=state.language;}}catch{}
 // The native title bar takes the sidebar color, so it reads as part of the chrome.
 const css=getComputedStyle(document.documentElement);
 const colors={caption:css.getPropertyValue('--sidebar').trim(),text:css.getPropertyValue('--text').trim(),border:css.getPropertyValue('--border').trim(),dark:css.colorScheme!=='light'};
 const key=JSON.stringify(colors);
 if(key!==windowTheme){windowTheme=key;void api.action('windowTheme',colors);}
}
const tabName=tab=>tab.role==='map'&&tab.fixed?t('mapTab'):tab.role==='tracker'?'TarkovTracker':tab.kind==='blank'?t('newTab'):tab.kind==='settings'?t('settings'):tab.kind==='bosses'?t('bosses'):tab.kind==='screenshots'?t('screenshots'):tab.kind==='bookmarks'?t('bookmarks'):tab.kind==='tabs'?t('allTabs'):tab.title||tab.url;
// Tabs listed in the tab section (the strip sizes itself by their number).
const listedTabs=()=>state.tabs.filter(tab=>!tab.fixed&&tab.kind!=='bookmarks'&&tab.kind!=='settings'&&tab.kind!=='screenshots'&&tab.kind!=='bosses'&&tab.kind!=='tabs');
const tabCount=()=>listedTabs().length;
function tabs(){return listedTabs().map(tab=>`<div class="tab ${state.active===tab.id?'active':''} ${tab.pinned?'pinned':''}" data-tab="${esc(tab.id)}" role="tab" aria-selected="${state.active===tab.id}"><button data-action="activate" data-id="${esc(tab.id)}" class="tab-select" title="${esc(tabName(tab))}">${tabIcon(tab)?favicon(tabIcon(tab)):icon(tab.kind==='settings'?'settings':tab.kind==='bookmarks'?'bookmark':tab.kind==='blank'?'plus':tab.role==='map'?'map':tab.task?'task':'globe','tab-icon')}<span class="tab-name">${esc(tabName(tab))}</span></button><span class="tab-actions">${tab.kind==='web'?`<button class="tab-pin" data-action="pin" data-id="${esc(tab.id)}" title="${esc(t(tab.pinned?'unpin':'pin'))}" aria-label="${esc(t(tab.pinned?'unpin':'pin'))}" aria-pressed="${!!tab.pinned}">${icon('pin','icon pin-on')}${icon('pinOff','icon pin-off')}</button>`:''}${tab.pinned?'':`<button class="tab-close" data-action="close" data-id="${esc(tab.id)}" title="${esc(t('close'))}" aria-label="${esc(t('close'))}">${icon('x')}</button>`}</span></div>`).join('');}
// Fixed views (TARKOV.DEV, TarkovTracker) sit above the sections like nav items.
function mapEntry(){return state.tabs.filter(tab=>tab.fixed).map(tab=>`<div class="tab map-entry ${state.active===tab.id?'active':''}"><button data-action="activate" data-id="${esc(tab.id)}" class="tab-select" title="${esc(tab.role==='map'?t('mapTabHelp'):tabName(tab))}">${tabIcon(tab)?favicon(tabIcon(tab)):icon(tab.role==='map'?'map':'tracker','tab-icon')}<span class="tab-name">${esc(tabName(tab))}</span></button></div>`).join('');}
function connectionLabel(){
 const mode=state.connection.mode;
 if(mode==='off')return t('off');
 const phase=state.peer?.phase||'idle';
 const status={connected:'p2pConnected',connecting:'p2pConnecting',gathering:'p2pGathering','waiting-answer':'p2pWaitingAnswer','waiting-host':'p2pWaitingHost',failed:'p2pFailed',disconnected:'p2pDisconnected'}[phase]||'p2pIdle';
 return mode==='local'?`${t('hostMode')} · ${t('local')}${phase!=='idle'?' · '+t(status):''}`:`${t('clientMode')} · ${t(status)}`;
}
// The top of the sidebar: collapse button and the status indicators; the rest
// of the row is title bar, so the window can be dragged from it. In the
// horizontal layout the indicators sit at the right end of the top strip.
function brandBar(){
 return state.layout==='vertical'?`<div class="app-menu-anchor">${sidebarToggle()}${indicators()}</div>`:'';
}
// Host (or connection) status and the monitoring switch.
function indicators(){
 const label=connectionLabel();
 const mode=state.connection.mode;
 const status=mode==='local'?'host':mode==='off'?'off':state.peer?.phase==='connected'?'linked':'unlinked';
 return `<span class="mode-status ${status}" tabindex="0" role="img" aria-label="${esc(label)}">${icon(status)}<span class="status-tooltip" role="tooltip">${esc(label)}</span></span>${monitorButton()}`;
}
function sidebarToggle(){
 const label=t(state.sidebarCollapsed?'expandSidebar':'collapseSidebar');
 return `<button class="dock-button sidebar-toggle" data-action="toggleSidebar" title="${esc(label)}" aria-label="${esc(label)}" aria-expanded="${!state.sidebarCollapsed}">${icon(state.sidebarSide==='right'?'panelRight':'panelLeft')}</button>`;
}
// The window is frameless: the shell draws the caption buttons. They sit at
// the right of the toolbar, or of the tab strip in the horizontal layout.
let maximised=false;
function windowControls(){
 return `<div class="window-controls"><button data-action="windowMinimise" title="${esc(t('minimise'))}" aria-label="${esc(t('minimise'))}">${icon('minimise')}</button><button data-action="windowMaximise" title="${esc(t(maximised?'restore':'maximise'))}" aria-label="${esc(t(maximised?'restore':'maximise'))}">${icon(maximised?'restore':'maximise')}</button><button class="window-close" data-action="windowClose" title="${esc(t('close'))}" aria-label="${esc(t('close'))}">${icon('x')}</button></div>`;
}
async function syncMaximised(){const next=await Window.IsMaximised().catch(()=>maximised);if(next!==maximised){maximised=next;render();}}
// Monitoring is operated from the top of the sidebar, next to the Host
// status, not from settings: its state is always in view. The tooltip adds the map, raid and TarkovTracker state.
function monitorButton(){
 const h=state.host;if(!h||!state.localHost)return '';
 const on=!!h.monitoring;
 const details=[t(on?'monitorOn':'monitorOff'),h.map,on?t(h.raid?'inRaid':'outRaid'):'',h.tracker?`${t('trackerLabel')}: ${words[state.language]['tracker_'+h.tracker]||h.tracker}`:'',t(on?'monitorStop':'monitorStart')].filter(Boolean).join(' · ');
 return `<button class="monitor-toggle ${on?'on':''}" data-action="monitor" title="${esc(details)}" aria-label="${esc(details)}" aria-pressed="${on}"><span class="monitor-dot"></span></button>`;
}
// The dock's layout button opens a menu choosing, by icon, where the tabs
// and the item details go.
function layoutToggle(){
 const label=t('placement'),nav=navPlace();
 return `<button class="dock-button place-toggle ${placeMenu?'selected':''}" data-action="placeMenu" title="${esc(label)}" aria-label="${esc(label)}" aria-haspopup="menu" aria-expanded="${!!placeMenu}">${icon('place'+nav[0].toUpperCase()+nav.slice(1))}</button>`;
}
const navPlace=()=>state.layout==='horizontal'?'top':state.sidebarSide;
const placeMenuWidth=232;
function placeMenuHTML(){
 if(!placeMenu)return '';
 const row=(label,action,current,sides)=>`<div class="place-row"><span>${esc(label)}</span><div class="place-choices" role="group" aria-label="${esc(label)}">${sides.map(side=>{const name=t('place'+side[0].toUpperCase()+side.slice(1));return `<button class="place-choice ${side===current?'selected':''}" role="menuitemradio" aria-checked="${side===current}" data-action="${action}" data-id="${side}" title="${esc(name)}" aria-label="${esc(name)}">${icon('place'+side[0].toUpperCase()+side.slice(1))}</button>`;}).join('')}</div></div>`;
 const at=placeMenu.top!==undefined?`top:${placeMenu.top}px`:`bottom:${placeMenu.bottom}px`;
 return `<div class="context-backdrop" data-action="closePlaceMenu"></div><div class="place-menu" role="menu" aria-label="${esc(t('placement'))}" style="left:${placeMenu.left}px;${at};width:${placeMenuWidth}px">${row(t('navPosition'),'placeNav',navPlace(),['left','right','top'])}${row(t('itemDock'),'placeItem',state.itemDock,['left','right','bottom'])}</div>`;
}
// It opens above the button in the sidebar's dock, below it in the top strip.
// Page views are native windows above the shell: they hide while the menu
// reaches over the page area.
function openPlaceMenu(button){
 const r=button.getBoundingClientRect(),top=state.layout==='horizontal';
 const left=Math.round(Math.max(4,Math.min(state.sidebarSide==='right'||top?r.right-placeMenuWidth:r.left,innerWidth-placeMenuWidth-4)));
 placeMenu=top?{left,top:Math.round(r.bottom+6)}:{left,bottom:Math.round(innerHeight-r.top+6)};
 render();
 const menu=document.querySelector('.place-menu')?.getBoundingClientRect(),page=document.querySelector('main')?.getBoundingClientRect();
 placeMenu.overlay=!!(menu&&page&&page.width>0&&menu.right>page.left&&menu.left<page.right&&menu.bottom>page.top&&menu.top<page.bottom);
 if(placeMenu.overlay)void api.action('overlay',true);
 document.querySelector('.place-choice.selected')?.focus();
}
function closePlaceMenu(){
 if(!placeMenu)return;
 const overlay=placeMenu.overlay;placeMenu=null;render();
 if(overlay)void api.action('overlay',false);
}
// Sidebar section with the bookmarks pinned to it. Bookmarks dragged here from
// the bookmarks page are pinned at the drop position.
function bookmarkSection(){
 const pinned=state.bookmarks.filter(b=>b.sidebar);
 const open=state.tabs.find(tab=>tab.id===state.active)?.kind==='bookmarks';
 const folded=state.bookmarksCollapsed;
 return `<div class="section-label bookmark-section-label ${open?'active':''}"><button class="section-link" data-action="toggleBookmarkSection" aria-expanded="${!folded}" title="${esc(t(folded?'expandSection':'collapseSection'))}">${esc(t('bookmarks'))}${icon('chevron','section-chevron')}</button><button class="new-tab bookmarks-open" data-action="bookmarks" title="${esc(t('allBookmarks'))}" aria-label="${esc(t('allBookmarks'))}" aria-pressed="${open}">${icon('bookmark')}</button></div><div class="sidebar-bookmarks ${pinned.length?'':'empty'} ${folded?'folded':''}">${pinned.map(b=>`<div class="sidebar-bookmark" draggable="true" data-sidebar-bookmark="${esc(b.id)}"><button class="bookmark-icon-button" data-action="bookmarkOpen" data-id="${esc(b.id)}" title="${esc(b.name+' — '+hostOf(b.url))}" aria-label="${esc(b.name)}">${siteFavicon(b.url)?favicon(siteFavicon(b.url)):`<span class="bookmark-letter">${esc([...b.name.trim()][0]?.toUpperCase()||'?')}</span>`}</button></div>`).join('')||`<p class="drop-hint">${esc(t('bookmarkDropHint'))}</p>`}</div>`;
}
// Sidebar section with the latest screenshot: its thumbnail opens it large,
// its icon all of them (the screenshot page); the heading folds the preview
// away, like the bookmarks. The collapsed rail and the
// horizontal tab strip show only its icon, which opens the list.
function screenshotSection(){
 const shots=state.screenshots;
 if(!shots)return '';
 const open=state.tabs.find(tab=>tab.id===state.active)?.kind==='screenshots';
 const folded=state.screenshotsCollapsed;
 const latest=shots.list[0],thumb=latest&&shots.thumbs[latest.name];
 const preview=latest?`<button class="shot-latest" data-action="screenshotOpen" data-id="${esc(latest.name)}" title="${esc(latest.name)}">${thumb?`<img src="${thumb}" alt="">`:`<span class="shot-placeholder">${icon('image')}</span>`}<span class="shot-age">${esc(age(latest.time,state.language))}</span>${shotBadges(latest.meta)}</button>`:`<p class="shot-empty">${esc(t('noScreenshots'))}</p>`;
 return `<div class="section-label screenshot-section-label ${open?'active':''}"><button class="section-link" data-action="toggleScreenshotSection" aria-expanded="${!folded}" title="${esc(t(folded?'expandSection':'collapseSection'))}">${esc(t('screenshots'))}${icon('chevron','section-chevron')}</button><button class="new-tab shots-open" data-action="screenshots" title="${esc(t('allScreenshots'))}" aria-label="${esc(t('allScreenshots'))}" aria-pressed="${open}">${icon('image')}</button></div>${folded?'':`<div class="shot-section">${preview}</div>`}`;
}
// Favicons load without a referrer; a broken one falls back to the globe icon.
// Icons come from the icon cache when it has them.
const iconSrc=url=>state.faviconData?.[url]||url;
const tabIcon=tab=>tab.favicon||(tab.kind==='web'?siteFavicon(tab.url):undefined);
const favicon=url=>`<img class="tab-icon favicon" src="${esc(iconSrc(url))}" alt="" referrerpolicy="no-referrer" loading="lazy">`;
const siteFavicon=url=>state.favicons?.[hostname(url)];
const hostOf=url=>{try{return new URL(url).hostname.replace(/^www\./,'');}catch{return url;}};
function bookmarkItem(b){
 return `<div class="bookmark-item" draggable="true" data-bookmark="${esc(b.id)}"><button class="bookmark-open" data-action="bookmarkOpen" data-id="${esc(b.id)}" title="${esc(b.url)}"><span class="bookmark-avatar" aria-hidden="true">${siteFavicon(b.url)?`<img src="${esc(iconSrc(siteFavicon(b.url)))}" alt="" referrerpolicy="no-referrer" loading="lazy">`:esc([...b.name.trim()][0]?.toUpperCase()||'?')}</span><span class="bookmark-text"><span class="bookmark-name">${esc(b.name)}</span><span class="bookmark-host">${esc(hostOf(b.url))}</span></span></button><span class="bookmark-actions"><button data-action="bookmarkPin" data-id="${esc(b.id)}" class="${b.sidebar?'selected':''}" title="${esc(t(b.sidebar?'unpinFromSidebar':'pinToSidebar'))}" aria-label="${esc(t(b.sidebar?'unpinFromSidebar':'pinToSidebar'))}" aria-pressed="${!!b.sidebar}">${icon(b.sidebar?'pinOff':'pin')}</button><button data-action="editBookmark" data-id="${esc(b.id)}" title="${esc(t('edit'))}" aria-label="${esc(t('edit'))}">${icon('edit')}</button><button data-action="deleteBookmark" data-id="${esc(b.id)}" title="${esc(t('delete'))}" aria-label="${esc(t('delete'))}">${icon('trash')}</button></span></div>`;
}
// Built-in categories show translated names; typed names are kept as typed.
const groupName=group=>bookmarkGroups.includes(group)?t(group):group;
const groupKey=name=>{const value=String(name||'').trim();return bookmarkGroups.find(g=>value===words.ja[g]||value===words.en[g]||value===g)||value;};
const allGroups=()=>[...bookmarkGroups,...[...new Set(state.bookmarks.map(b=>b.group))].filter(g=>!bookmarkGroups.includes(g)).sort((a,b)=>a.localeCompare(b))];
// Right-click menu for pinned bookmarks.
const contextMenuWidth=200,contextMenuHeight=84;
function contextMenuHTML(){
 const b=contextMenu&&state.bookmarks.find(b=>b.id===contextMenu.id);
 if(!b)return '';
 return `<div class="context-backdrop" data-action="closeContext"></div><div class="context-menu" role="menu" aria-label="${esc(b.name)}" style="left:${contextMenu.x}px;top:${contextMenu.y}px;width:${contextMenuWidth}px"><button role="menuitem" data-action="bookmarkOpen" data-id="${esc(b.id)}">${icon('globe')}<span>${esc(t('open'))}</span></button><button role="menuitem" data-action="bookmarkPin" data-id="${esc(b.id)}">${icon('pinOff')}<span>${esc(t('unpinFromSidebar'))}</span></button></div>`;
}
function openContextMenu(id,x,y){
 const left=Math.max(4,Math.min(x,innerWidth-contextMenuWidth-4)),top=Math.max(4,Math.min(y,innerHeight-contextMenuHeight-4));
 // Page views are native windows above the shell: hide them while the menu
 // extends past the sidebar.
 const sidebar=state.layout==='vertical'?(state.sidebarCollapsed?52:state.sidebarWidth):0;
 const overlay=state.layout==='horizontal'||(state.sidebarSide==='right'?left<innerWidth-sidebar:left+contextMenuWidth>sidebar);
 contextMenu={id,x:left,y:top,overlay};
 if(overlay)void api.action('overlay',true);
 render();document.querySelector('.context-menu button')?.focus();
}
function closeContextMenu(){
 if(!contextMenu)return;
 const overlay=contextMenu.overlay;contextMenu=null;render();
 if(overlay)void api.action('overlay',false);
}
// The tabs page lists every open tab, searchable by name and address; the
// "Tabs" heading in the sidebar opens it.
function tabsPage(){
 const query=tabQuery.trim().toLowerCase();
 // The same tabs as the sidebar's list: not the fixed map and tracker, nor the app's own pages.
 const all=listedTabs();
 const found=all.filter(tab=>{const name=String(tabName(tab)||'');return !query||name.toLowerCase().includes(query)||String(tab.url||'').toLowerCase().includes(query);});
 const row=tab=>`<div class="bookmark-item tab-row ${state.active===tab.id?'current':''}"><button class="bookmark-open" data-action="activate" data-id="${esc(tab.id)}">${tabIcon(tab)?`<img class="tab-favicon" src="${esc(tabIcon(tab))}" alt="">`:icon(tab.kind==='web'?'globe':'plus','tab-icon')}<span class="bookmark-name">${esc(tabName(tab))}</span>${tab.kind==='web'?`<span class="bookmark-url">${esc(tab.url)}</span>`:''}</button>${!tab.pinned?`<button class="tab-row-close" data-action="close" data-id="${esc(tab.id)}" title="${esc(t('tabsClose'))}" aria-label="${esc(t('tabsClose'))}">${icon('x')}</button>`:''}</div>`;
 return `<div class="page bookmarks-page tabs-page"><div class="bookmarks-head"><h1>${esc(t('allTabs'))}</h1><div class="bookmarks-tools"><label class="bookmark-search">${icon('search')}<input id="tab-search" type="search" autocomplete="off" placeholder="${esc(t('searchTabs'))}" aria-label="${esc(t('searchTabs'))}" value="${esc(tabQuery)}"></label></div></div><p class="hint">${esc(t('tabsHint'))}</p>${found.length?`<div class="bookmark-list tab-list">${found.map(row).join('')}</div>`:`<p class="hint">${esc(t('noTabResults'))}</p>`}</div>`;
}
function bookmarksPage(){
 const query=bookmarkQuery.trim().toLowerCase();
 const found=state.bookmarks.filter(b=>!query||b.name.toLowerCase().includes(query)||b.url.toLowerCase().includes(query));
 const groups=allGroups().map(g=>[g,found.filter(b=>b.group===g)]).filter(([,items])=>items.length);
 const view=state.bookmarkView;
 return `<div class="page bookmarks-page"><div class="bookmarks-head"><h1>${t('bookmarks')}</h1><div class="bookmarks-tools"><label class="bookmark-search">${icon('search')}<input id="bookmark-search" type="search" autocomplete="off" placeholder="${esc(t('searchBookmarks'))}" aria-label="${esc(t('searchBookmarks'))}" value="${esc(bookmarkQuery)}"></label><div class="segmented" role="group"><button data-action="bookmarkView" data-id="grid" class="${view==='grid'?'selected':''}" title="${esc(t('gridView'))}" aria-label="${esc(t('gridView'))}">${icon('grid')}</button><button data-action="bookmarkView" data-id="list" class="${view==='list'?'selected':''}" title="${esc(t('listView'))}" aria-label="${esc(t('listView'))}">${icon('list')}</button></div><button class="primary" data-action="addBookmark">${icon('plus')}<span>${t('addBookmark')}</span></button></div></div><p class="hint">${t('bookmarkDragHint')}</p>${bookmarkEditor()}${groups.map(([g,items])=>`<section class="bookmark-group"><h3>${esc(groupName(g))}<span class="count">${items.length}</span></h3><div class="bookmark-${view}">${items.map(bookmarkItem).join('')}</div></section>`).join('')||`<p class="empty-tabs">${t(query?'noBookmarkResults':'empty')}</p>`}</div>`;
}
function bookmarkEditor(){if(!editingBookmark)return '';const b=editingBookmark;return `<form id="bookmark-form" class="panel editor"><label class="field"><span>${t('name')}</span><input name="name" required maxlength="150" value="${esc(b.name)}"></label><label class="field"><span>${t('url')}</span><input name="url" type="url" required value="${esc(b.url)}" placeholder="https://"></label><label class="field"><span>${t('group')}</span><input name="group" list="bookmark-groups" maxlength="40" autocomplete="off" placeholder="${esc(t('groupHint'))}" value="${esc(groupName(b.group))}"><datalist id="bookmark-groups">${allGroups().map(g=>`<option value="${esc(groupName(g))}"></option>`).join('')}</datalist></label><div class="actions"><button type="submit" class="primary">${t('done')}</button><button type="button" data-action="cancelBookmark">${t('cancel')}</button></div></form>`;}
const sectionIcons={about:'info',appearance:'palette',tasks:'task',adblock:'shield',connection:'linked',status:'activity',logs:'list',folders:'folder',recognition:'scan',remote:'map',tracker:'tracker',sounds:'volume',startup:'power',debug:'bug'};
// Sections grouped by what they are about. Some are the browser's own and
// some the Host's (its page in a frame); that split is not shown.
const settingsGroups=[['grpGeneral',['appearance','startup','sounds']],['grpGame',['folders','recognition','tasks']],['grpLinks',['remote','tracker','connection']],['grpBrowser',['adblock']],['grpDiagnostics',['status','logs','debug']],['grpAbout',['about']]];
const sectionAvailable=key=>browserSections.includes(key)||(state.localHost&&hostSections.includes(key));
// While settings are open, the sidebar lists their sections instead of tabs.
function settingsSidebar(){
 const link=key=>`<div class="tab ${state.settingsSection===key?'active':''}"><button class="tab-select" data-action="settingsSection" data-id="${key}" title="${esc(t('sec_'+key))}" ${state.settingsSection===key?'aria-current="page"':''}>${icon(sectionIcons[key],'tab-icon')}<span class="tab-name">${esc(t('sec_'+key))}</span></button></div>`;
 return `<div class="tab settings-back"><button class="tab-select" data-action="closeSettings" title="${esc(t('settingsBack'))}">${icon('back','tab-icon')}<span class="tab-name">${esc(t('settingsBack'))}</span></button></div>${settingsGroups.map(([group,keys])=>{const shown=keys.filter(sectionAvailable);return shown.length?`<div class="section-label"><span>${esc(t(group))}</span></div><div class="settings-list" style="--n:${shown.length}">${shown.map(link).join('')}</div>`:'';}).join('')}`;
}
function settings(){
 const key=browserSections.includes(state.settingsSection)?state.settingsSection:'appearance';
 const tutorial=key==='appearance'?`<section class="panel"><h2>${esc(t('tutorialShow'))}</h2><p class="hint">${esc(t('tutorialShowHelp'))}</p><button data-action="tutorial">${esc(t('tutorialShow'))}</button></section>`:'';
 return settingsHost()?'':`<div class="page settings"><h1>${esc(t('sec_'+key))}</h1>${browserSettings(key)}${tutorial}</div>`;
}
// The update controls, in About: check now, download or restart, and the
// state of the last check. The status strip along the bottom says the same
// while an update is pending.
function aboutUpdate(){
 const u=state.update||{};
 const v=key=>t(key).replace('{v}',u.latest||'');
 const text=u.state==='checking'?t('updateChecking'):u.state==='current'?t('updateUpToDate'):u.state==='available'?v('updateAvailable'):u.state==='downloading'?`${v('updateDownloading')} ${Math.max(0,Math.min(100,u.progress|0))}%`:u.state==='ready'?v('updateReady'):u.state==='unsupported'?t('updateUnsupported'):u.state==='error'?`${t('updateError')}${u.lastError?': '+u.lastError:''}`:t('updateNever');
 const checked=u.checkedAt?new Date(u.checkedAt).toLocaleString(state.language==='ja'?'ja-JP':'en-US'):'—';
 const busy=u.state==='checking'||u.state==='downloading';
 return `<section class="panel about-update"><h2>${esc(t('updateSection'))}</h2><dl class="about-facts"><div><dt>${esc(t('updateCurrent'))}</dt><dd>${esc(u.current||appVersion||t('aboutDev'))}</dd></div><div><dt>${esc(t('updateLatest'))}</dt><dd>${esc(u.latest||'—')}</dd></div><div><dt>${esc(t('updateCheckedAt'))}</dt><dd>${esc(checked)}</dd></div></dl><p class="update-state ${u.state==='error'?'peer-error':''}">${esc(text)}</p><div class="about-links"><button data-action="updateCheck" ${busy?'disabled':''}>${icon('reload')}${esc(t('updateCheck'))}</button>${u.state==='available'?`<button class="primary" data-action="updateDownload">${esc(t('updateDownload'))}</button>`:''}${u.state==='ready'?`<button class="primary" data-action="updateInstall">${esc(t('updateRestart'))}</button>`:''}</div><p class="hint">${esc(t('updateAutoNote'))}</p></section>`;
}
function browserSettings(key){
 switch(key){
 case 'about':return `<section class="panel about"><div class="about-head"><img src="/favicon-256.png?v=${esc(appVersion||'dev')}" alt="" width="56" height="56"><div><h2>MAYAK</h2><p class="about-version">${esc(t('aboutVersion'))} ${esc(appVersion||t('aboutDev'))}</p></div></div><p>${esc(t('aboutTagline'))}</p><div class="about-links"><button data-action="openOrFocus" data-id="https://mayak.ich.sh">${icon('globe')}${esc(t('aboutSite'))}</button><button data-action="openOrFocus" data-id="https://github.com/ichi0g0y/mayak">${icon('external')}${esc(t('aboutSource'))}</button><button data-action="openOrFocus" data-id="${CHANGELOG_URL}">${icon('list')}${esc(t('aboutReleases'))}</button></div><p class="hint">${esc(t('aboutLicense'))} ${esc(t('aboutCredits'))}</p></section>${aboutUpdate()}`;
 case 'appearance':return `<section class="panel"><div class="fields">${select('language',t('language'),[['ja','日本語'],['en','English']],state.language)}${select('theme',t('theme'),themes.map(key=>[key,key==='system'?t('themeSystem'):themeNames[key]]),state.theme)}${select('clock',t('clockFormat'),[['24',t('clock24')],['12',t('clock12')]],state.clock)}${select('navPosition',t('navPosition'),[['left',t('placeLeft')],['right',t('placeRight')],['top',t('placeTop')]],state.layout==='horizontal'?'top':state.sidebarSide)}${select('itemDock',t('itemDock'),[['right',t('placeRight')],['left',t('placeLeft')],['bottom',t('placeBottom')]],state.itemDock)}</div></section>`;
 case 'tasks':{
  // On the Host the site is its setting; a receiving computer may follow it.
  const local=state.localHost&&state.connection.mode==='local';
  const site=local?`<label class="field"><span>${esc(t('questSite'))}</span><select id="host-quest-site">${siteChoices().map(([v,l])=>option(v,l,state.hostQuestSite)).join('')}</select></label>`:select('questSite',t('questSite'),[['host',t('hostChoice')],...siteChoices()],state.questSite);
  return `<section class="panel"><div class="fields">${site}${select('taskMode',t('taskMode'),[['new',t('new')],['reuse',t('reuse')]],state.taskMode)}</div><p class="hint">${esc(t(local?'taskSiteHelp':'taskSiteClientHelp'))} ${t('taskHelp')}</p><label class="check"><input type="checkbox" data-action="translateWiki" ${state.translateWiki?'checked':''}>${esc(t('translateWiki'))}</label><p class="hint">${esc(t('translateWikiHelp'))}</p></section>`;
 }
 case 'adblock':return `<section class="panel"><label class="check"><input type="checkbox" data-action="adblock" ${state.adblock?'checked':''}>${t('adblockEnable')}</label><p class="hint">${t('adblockHelp')}</p></section>`;
 default:return `<section class="panel"><h2>${t('connection')}</h2>${select('mode',t('mode'),[...(state.localHost?[['local',t('local')]]:[]),['webrtc',t('webrtc')],['off',t('disabled')]],state.connection.mode,'connection')}${state.connection.mode==='local'?`<p class="hint">${t('localHelp')}</p>`:''}</section>${peerPanel()}`;
 }
}
function peerPanel(){
  if(state.connection.mode!=='webrtc'&&!(state.localHost&&state.connection.mode==='local'))return '';
  const p=state.peer||{phase:'idle',role:''};const receive=state.connection.mode==='webrtc';
  const statusKey={idle:'p2pIdle',gathering:'p2pGathering','waiting-answer':'p2pWaitingAnswer','waiting-host':p.pairCode?'p2pWaitingHostAuto':'p2pWaitingHost',connecting:'p2pConnecting',connected:'p2pConnected',failed:'p2pFailed',disconnected:'p2pDisconnected',closed:'p2pDisconnected'}[p.phase]||'p2pIdle';
  const disabled=p.busy?'disabled':'';
  const idle=!p.code||['failed','disconnected','closed'].includes(p.phase);
  const waiting=p.role==='sender'&&p.phase==='waiting-answer';
  const pairCode=p.pairCode?`${p.pairCode.slice(0,4)} ${p.pairCode.slice(4)}`:'';
  return `<section class="panel peer-panel"><h2>${t(receive?'p2pReceive':'p2pTitle')}</h2><p class="hint">${t('p2pHelp')}</p><p class="peer-status" role="status">${t(p.reason==='invite-expired'?'p2pExpired':statusKey)}</p>
  ${!receive&&p.phase!=='connected'&&!waiting?`<button class="primary" data-action="peerInvite" ${disabled}>${t('createInvite')}</button>`:''}
  ${waiting&&p.pairCode?`<div class="pair-code"><output>${esc(pairCode)}</output><button data-action="peerCopyPair">${t(copied?'copied':'copyCode')}</button></div><p class="hint">${t('pairCodeHelp')}</p>`:''}
  ${waiting&&p.relayError?`<p class="hint peer-error">${t('relayFailed')}</p>`:''}
  ${receive&&idle&&p.phase!=='connected'?`<form id="peer-join-form"><label class="field"><span>${t('enterPairCode')}</span><input name="pairCode" inputmode="numeric" autocomplete="off" spellcheck="false" placeholder="1234 5678" value="${esc(peerDrafts.pairCode||'')}" required></label><button class="primary" type="submit" ${disabled}>${t('joinPair')}</button></form>`:''}
  ${p.phase!=='connected'?`<details class="peer-manual" ${p.code&&!p.pairCode?'open':''}><summary>${t('manualExchange')}</summary><p class="hint">${t('manualExchangeHelp')}</p>
  ${receive&&idle?`<form id="peer-offer-form"><label class="field"><span>${t('enterOffer')}</span><textarea name="peerOffer" required spellcheck="false" maxlength="100000" rows="3">${esc(peerDrafts.offer)}</textarea></label><button type="submit" ${disabled}>${t('createAnswer')}</button></form>`:''}
  ${p.code?`<div class="code-output"><p class="hint">${t(p.role==='sender'?'offerStep':'answerStep')}</p><label class="field"><span>${t(p.role==='sender'?'inviteCode':'answerCode')}</span><textarea readonly rows="2" spellcheck="false">${esc(p.code)}</textarea></label><button data-action="peerCopy">${t(copied?'copied':'copyCode')}</button></div>`:''}
  ${waiting?`<form id="peer-answer-form"><label class="field"><span>${t('enterAnswer')}</span><textarea name="peerAnswer" required spellcheck="false" maxlength="100000" rows="3">${esc(peerDrafts.answer)}</textarea></label><button type="submit" ${disabled}>${t('finishPairing')}</button></form>`:''}
  </details>`:''}
  ${p.phase!=='idle'?`<button data-action="peerClose" ${disabled}>${t('disconnectPeer')}</button>`:''}
  <p class="hint">${t('p2pLimit')}</p><details><summary>${t('stunLabel')}</summary><label class="field"><span>${t('stunLabel')}</span><input data-scope="connection" data-key="stun" value="${esc(state.connection.stun||'')}" placeholder="stun:stun.cloudflare.com:3478"></label><p class="hint">${t('stunHelp')}</p></details></section>`;
}
function settingsHost(){return state.tabs.find(tab=>tab.id===state.active)?.kind==='settings'&&hostSections.includes(state.settingsSection)&&state.localHost;}
// Ages tick every half minute without re-rendering; prices refresh every
// few minutes while the panel is visible.
setInterval(()=>{
 document.querySelectorAll('.item-age').forEach(el=>{el.textContent=age(el.dataset.time,state?.language);});
 document.querySelectorAll('[data-goon-time]').forEach(el=>{el.textContent=age(el.dataset.goonTime,state?.language);el.closest('[data-fresh]')?.setAttribute('data-fresh',goonFresh(el.dataset.goonTime));});
},30000);

// A thin bar along the bottom of the address field while the page loads.
// Its animation is offset by the clock, so a re-render does not restart it.
const loadBar=tab=>tab&&state.loadingTabs?.includes(tab.id)?`<span class="load-bar" style="--load-offset:-${Date.now()%1400}ms"></span>`:'';
// Areas that scroll on their own, by selector.
let scrolledTab='';
const scrollAreas=['.item-body','.tab-strip','main','.item-results'];
// The update bar: a status strip along the bottom of the window (api.js lifts
// the page views by it), shown while a newer version is found, downloading
// or ready, until it is applied or put off.
function updateBarHTML(){
 const u=state.update;
 if(!state.updateBar||!u)return '';
 const text=t(u.state==='ready'?'updateReady':u.state==='downloading'?'updateDownloading':'updateAvailable').replace('{v}',u.latest);
 const progress=u.state==='downloading'?`<span class="update-progress"><i style="width:${Math.max(0,Math.min(100,u.progress|0))}%"></i></span>`:'';
 const action=u.state==='ready'?`<button class="primary" data-action="updateInstall">${esc(t('updateRestart'))}</button>`:u.state==='available'?`<button class="primary" data-action="updateDownload">${esc(t('updateDownload'))}</button>`:'';
 const notes=u.releaseUrl?`<button data-action="updateNotes">${esc(t('updateNotes'))}</button>`:'';
 return `<div class="update-bar" role="status" title="${esc(u.state==='ready'?t('updateReadyHint'):'')}"><span class="update-text">${esc(text)}</span>${progress}<span class="update-actions">${notes}${action}<button data-action="updateDismiss">${esc(t('updateLater'))}</button></span></div>`;
}
function render(){
  if(!state)return;
  // Rebuilding the DOM would drop the tab being dragged; render when it lands.
  if(tabDragActive()){renderDeferred=true;return;}
  const focused=document.activeElement;const focusId=focused?.id;const focusKey=focused?.dataset?.key;const focusName=focused?.name;const draft=['INPUT','TEXTAREA'].includes(focused?.tagName)&&!focused.readOnly?focused.value:undefined;const start=focused?.selectionStart;
  const hostFrame=document.querySelector('#host-settings');
  applyTheme();
  // The Host page shows the section named in its hash; changing only the hash
  // switches sections without reloading it.
  if(settingsHost()){const want='#'+state.settingsSection;if(!hostFrame.getAttribute('src'))hostFrame.src='/settings.html'+want;else try{if(hostFrame.contentWindow.location.hash!==want)hostFrame.contentWindow.location.hash=want;}catch{}}
  hostFrame.hidden=!settingsHost();
  document.documentElement.lang=state.language;document.body.dataset.layout=state.layout;document.body.dataset.item=state.itemOpen?state.itemDock:'closed';document.body.dataset.nav=state.layout==='horizontal'?'top':state.sidebarSide;document.body.dataset.settings=settingsHost()?'host':'';document.body.dataset.updateBar=state.updateBar?'on':'';document.body.dataset.statusRows=String(state.statusRows||0);document.body.dataset.sidebar=state.layout==='vertical'&&state.sidebarCollapsed?'collapsed':'open';if(!sidebarDrag)document.documentElement.style.setProperty('--sidebar-width',state.sidebarWidth+'px');if(!itemDrag){document.documentElement.style.setProperty('--item-width',state.itemPanelWidth+'px');document.documentElement.style.setProperty('--item-height',state.itemPanelHeight+'px');}
  const tab=state.tabs.find(tab=>tab.id===state.active);
  const isWeb=tab?.kind==='web';
  // A rebuilt element under the pointer would fade into its hover colour
  // again on every render (a page loading in the popup renders often).
  document.documentElement.classList.add('rendering');requestAnimationFrame(()=>requestAnimationFrame(()=>document.documentElement.classList.remove('rendering')));
  // Rebuilding would scroll these back to the top: keep where they were.
  // The page area starts at the top on another tab.
  const scrolled=scrollAreas.map(sel=>{const el=document.querySelector(sel);return el&&[sel,el.scrollTop,el.scrollLeft];}).filter(Boolean);
  const html=`<div class="sidebar-resizer" role="separator" aria-orientation="vertical" aria-valuemin="${sidebarWidths.min}" aria-valuemax="${sidebarWidths.max}" aria-valuenow="${state.sidebarWidth}" title="${esc(t('resizeSidebar'))}"></div>${brandBar()}<nav class="tab-strip ${tab?.kind==='settings'?'settings-strip':''}" aria-label="${esc(t(tab?.kind==='settings'?'settings':'tabHelp'))}">${tab?.kind==='settings'?settingsSidebar():`${mapEntry()}${bookmarkSection()}${screenshotSection()}${bossSection()}<div class="section-label ${tab?.kind==='tabs'?'active':''}"><button class="section-link" data-action="tabsPage" title="${esc(t('allTabs'))}">${esc(t('tabs'))}</button><button class="new-tab" data-action="newTab" title="${esc(t('newTab'))}" aria-label="${esc(t('newTab'))}">${icon('plus')}</button></div><div class="tabs" role="tablist" style="--n:${tabCount()}">${tabs()}</div>`}</nav><div class="layout-dock">${layoutToggle()}${itemToggle()}<button class="dock-button dock-settings ${tab?.kind==='settings'?'selected':''}" aria-pressed="${tab?.kind==='settings'}" data-action="settings" title="${esc(t('settings'))}" aria-label="${esc(t('settings'))}">${icon('settings')}</button>${state.layout==='horizontal'?`<span class="dock-indicators">${indicators()}</span>`:''}</div>${windowControls()}<div class="toolbar">${tab?.fixed?`<button data-action="back" aria-label="${t('back')}" title="${t('back')}" ${!tab.canBack?'disabled':''}>${icon('back')}</button><button data-action="forward" aria-label="${t('forward')}" title="${t('forward')}" ${!tab.canForward?'disabled':''}>${icon('forward')}</button><button data-action="reload" aria-label="${t('reload')}" title="${t('reload')}">${icon('reload')}</button><button data-action="home" aria-label="${esc(t('home'))}" title="${esc(t('homeHelp'))}" ${tab.url===tab.home?'disabled':''}>${icon('home')}</button><form id="address-form" class="readonly">${icon('globe','address-icon')}<input id="address" readonly aria-readonly="true" aria-label="${esc(tabName(tab))}" title="${esc(t('fixedAddress'))}" value="${esc(tab.url)}">${loadBar(tab)}</form>`:tab?.kind==='bookmarks'||tab?.kind==='screenshots'||tab?.kind==='bosses'?'':tab?.kind==='settings'?'':`<button data-action="back" aria-label="${t('back')}" title="${t('back')}" ${!tab?.canBack?'disabled':''}>${icon('back')}</button><button data-action="forward" aria-label="${t('forward')}" title="${t('forward')}" ${!tab?.canForward?'disabled':''}>${icon('forward')}</button><button data-action="reload" aria-label="${t('reload')}" title="${t('reload')}" ${!isWeb?'disabled':''}>${icon('reload')}</button><form id="address-form">${icon('search','address-icon')}<input id="address" aria-label="${t('address')}" placeholder="${t('address')}" value="${esc(isWeb?tab.url:'')}">${loadBar(tab)}</form>${tab?.task?`<select id="task-site" aria-label="${t('questSite')}">${siteChoices().map(([key,label])=>option(key,label,sitesForURL(tab))).join('')}</select>${translateButton(tab)}<button data-action="wikiSearch" title="${t('searchWiki')}" aria-label="${t('searchWiki')}">${icon('search')}</button>`:translateButton(tab)}`}${isWeb?`<button class="open-external" data-action="openExternal" title="${esc(t('openExternal'))}" aria-label="${esc(t('openExternal'))}">${icon('external')}</button>`:''}${state.layout==='vertical'?'<div class="titlebar-grip"></div>':''}</div><main>${tab?.kind==='settings'?settings():tab?.kind==='bookmarks'?bookmarksPage():tab?.kind==='tabs'?tabsPage():tab?.kind==='screenshots'?screenshotsPage():tab?.kind==='bosses'?bossesPage():!tab?`<p class="empty-tabs">${t('noTabs')}</p>`:''}</main>${tab?.kind==='settings'?`<button class="page-close" data-action="closeSettings" title="${esc(t('closeSettings'))}" aria-label="${esc(t('closeSettings'))}">${icon('x')}</button>`:''}${itemPanel()}${contextMenuHTML()}${placeMenuHTML()}${state.error?`<aside class="error-bar" role="alert"><span title="${esc(state.error)}">${esc(state.error)}</span><button data-action="dismiss" title="${esc(t('dismiss'))}" aria-label="${esc(t('dismiss'))}">${icon('x')}</button></aside>`:''}${updateBarHTML()}${tutorialOpen?tutorialHTML():''}`;
  // The DOM is morphed to the new markup rather than rebuilt: elements that
  // stay keep their node, so hover, focus, a press in flight and scroll
  // positions survive a render, and a render costs only its differences.
  morphdom(document.querySelector('#app'),'<div id="app">'+html+'</div>',{childrenOnly:true,onBeforeElUpdated:(from,to)=>!from.isEqualNode(to)});
  for(const [sel,top,left] of scrolled){const el=document.querySelector(sel);if(el&&!(sel==='main'&&scrolledTab!==state.active)){el.scrollTop=top;el.scrollLeft=left;}}
  scrolledTab=state.active;
  if(focusId||focusKey||focusName){const target=[...document.querySelectorAll('input,select,textarea')].find(el=>focusId?el.id===focusId:focusKey?el.dataset.key===focusKey:el.name===focusName);if(target){if(draft!==undefined)target.value=draft;target.focus();try{target.setSelectionRange(start,start);}catch{}}}
}
// The translate button (Google Translate's proxy, see api.js): before the
// wiki search button on a task page, else after the address bar.
function translateButton(tab){if(tab?.kind!=='web'||tab.fixed)return '';const on=isTranslated(tab.url);return `<button class="translate-page ${on?'on':''}" data-action="translate" aria-pressed="${on}" title="${esc(t(on?'translateOff':'translatePage'))}" aria-label="${esc(t(on?'translateOff':'translatePage'))}">${icon('translate')}</button>`;}
function sitesForURL(tab){const url=originalURL(tab.url)||tab.url;return Object.keys(tab.task?.urls||{}).find(key=>tab.task.urls[key]===url)||'tarkov-dev';}
document.addEventListener('click',async event=>{
  const button=event.target.closest('[data-action]');if(!button||button.disabled)return;const type=button.dataset.action,id=button.dataset.id;
  if(type.startsWith('tutorial')){void handleTutorial(type);return;}
  if(type==='updateNotes'){void action('openOrFocus',CHANGELOG_URL+(state.update?.latest?'#v'+state.update.latest:''));return;}
  if(contextMenu){closeContextMenu();if(type==='closeContext')return;}
  if(placeMenu){closePlaceMenu();if(type==='closePlaceMenu'||type==='placeMenu')return;}
  if(type==='placeMenu'){openPlaceMenu(button);return;}
  if(type==='placeNav'){void action('preferences',{navPosition:id});return;}
  if(type==='placeItem'){void action('preferences',{itemDock:id});return;}
  if(type==='openExternal'){const url=webURL(state.tabs.find(t=>t.id===state.active)?.url);if(url)void Browser.OpenURL(url);return;}
  for(const handle of clickHandlers){if(await handle(type,id,button,event))return;}
  if(type==='newTab'){await action(type);await focusAddress();return;}
  if(type==='toggleSidebar'){void action('preferences',{sidebarCollapsed:!state.sidebarCollapsed});return;}
  if(type==='toggleScreenshotSection'){void action('preferences',{screenshotsCollapsed:!state.screenshotsCollapsed});return;}
  if(type==='toggleBookmarkSection'){void action('preferences',{bookmarksCollapsed:!state.bookmarksCollapsed});return;}
  if(type==='bookmarkView'){void action('preferences',{bookmarkView:id});return;}
  if(type==='windowMinimise'){void Window.Minimise();return;}
  if(type==='windowMaximise'){void Window.ToggleMaximise().then(syncMaximised);return;}
  if(type==='windowClose'){void Window.Close();return;}
  if(type==='adblock'||type==='translateWiki')return;
  if(type==='peerCopy'||type==='peerCopyPair')copied=true;
  if(['peerInvite','peerAccept','peerJoin','peerClose'].includes(type))copied=false;
  if(type==='addBookmark'){editingBookmark={name:'',url:'',group:'other'};render();document.querySelector('[name="name"]')?.focus();return;}
  if(type==='editBookmark'){editingBookmark={...state.bookmarks.find(b=>b.id===id)};render();return;}
  if(type==='cancelBookmark'){editingBookmark=null;render();return;}
  if(type==='bookmarkOpen'){void action('open',state.bookmarks.find(b=>b.id===id).url);return;}
  if(type==='wikiSearch'){const tab=state.tabs.find(t=>t.id===state.active);const japanese=sitesForURL(tab)==='japanese-wiki';void action('navigate',japanese?`https://wikiwiki.jp/eft/?cmd=search&word=${encodeURIComponent(tab.task.name)}`:`https://escapefromtarkov.fandom.com/wiki/Special:Search?query=${encodeURIComponent(tab.task.name)}`);return;}
  void action(type,id);
});
document.addEventListener('change',event=>{const input=event.target;if(input.closest('#bookmark-form')&&editingBookmark)editingBookmark[input.name]=input.value;if(input.dataset.scope)void action(input.dataset.scope,{[input.dataset.key]:input.value});else if(input.id==='task-site')void action('site',input.value);else if(input.id==='host-quest-site')void action('hostQuestSite',input.value);else if(input.dataset.action==='adblock')void action('preferences',{adblock:input.checked});if(input.dataset.action==='translateWiki')void action('preferences',{translateWiki:input.checked});});
document.addEventListener('input',event=>{
 if(event.target.id==='bookmark-search'){bookmarkQuery=event.target.value;render();return;}if(event.target.id==='tab-search'){tabQuery=event.target.value;render();return;}if(event.target.closest('#bookmark-form')&&editingBookmark)editingBookmark[event.target.name]=event.target.value;if(event.target.name==='peerOffer')peerDrafts.offer=event.target.value;if(event.target.name==='peerAnswer')peerDrafts.answer=event.target.value;if(event.target.name==='pairCode')peerDrafts.pairCode=event.target.value;});
document.addEventListener('submit',event=>{
  event.preventDefault();if(event.target.id==='peer-join-form'){copied=false;void action('peerJoin',peerDrafts.pairCode);return;}if(event.target.id==='peer-offer-form'){copied=false;void action('peerAccept',peerDrafts.offer);return;}if(event.target.id==='peer-answer-form'){void action('peerAnswer',peerDrafts.answer);return;}// Fixed views show their address read-only; Enter must not navigate them.
  if(event.target.id==='address-form'&&event.target.classList.contains('readonly'))return;
  // A URL opens; anything else is searched (resolveAddress). Enter then
  // hands the keyboard to the page, as in Chrome.
  if(event.target.id==='address-form'){const url=resolveAddress(document.querySelector('#address').value);if(!url)return;void action('navigate',url).then(()=>action('focus','page'));}
  if(event.target.id==='bookmark-form'){const data=Object.fromEntries(new FormData(event.target));data.id=editingBookmark?.id;data.group=groupKey(data.group);editingBookmark=null;void action('bookmark',data);}
});
installTabDrag({
 horizontal:()=>state.layout==='horizontal',
 drop:(id,before)=>{renderDeferred=false;void action('move',{id,before});},
 cancel:()=>{if(renderDeferred){renderDeferred=false;render();}},
 over:tabOverBookmarks,
 dropElsewhere:id=>{clearBookmarkDrop();renderDeferred=false;void action('bookmarkTab',{id,before:tabBookmarkBefore});},
});
api.onState(next=>{setState(next);render();if(!tutorialStarted){tutorialStarted=true;void loadVersion();if(!state.tutorialDone&&state.localHost)setTimeout(()=>{if(!state.tutorialDone&&!tutorialOpen)void openTutorial();},1500);}});
// Title bar behavior for the frameless window: double-click toggles maximise;
// maximising by any means (snap, keyboard) updates the caption button.
document.addEventListener('dblclick',event=>{if(getComputedStyle(event.target).getPropertyValue('--wails-draggable').trim()==='drag')void Window.ToggleMaximise().then(syncMaximised);});
window.addEventListener('resize',()=>void syncMaximised());
// Wails resets the caption buttons to the OS mode on a system theme change;
// send the colors again once it has.
lightScheme.addEventListener('change',()=>{if(state)applyTheme();setTimeout(()=>{if(state){windowTheme='';applyTheme();}},300);});
document.querySelector('#host-settings')?.addEventListener('load',()=>{if(state)applyTheme();});
// Chrome's tab shortcuts (state.js shortcut): pressed in the shell, or in a
// page view and forwarded by the Go side (browser:key). Switching to a page
// hands it the keyboard; switching to a shell page keeps it in the shell.
async function focusAddress(){
 await action('focus','shell');
 const input=document.querySelector('#address');input?.focus();input?.select();
}
const focusActive=next=>action('focus',next?.tabs?.find(t=>t.id===next.active)?.kind==='web'?'page':'shell');
async function runShortcut(key){
 const what=shortcut(key);if(!what||!state)return false;
 const tab=state.tabs.find(t=>t.id===state.active);
 switch(what.type){
 case 'newTab':await action('newTab');await focusAddress();break;
 case 'closeTab':if(tab&&!tab.fixed&&!tab.pinned)await focusActive(await action('close',tab.id));break;
 case 'reopenTab':await focusActive(await action('reopenTab'));break;
 case 'nextTab':case 'prevTab':await focusActive(await action('cycleTab',what.type==='nextTab'?1:-1));break;
 case 'tabAt':await focusActive(await action('tabAt',what.index));break;
 case 'address':await focusAddress();break;
 case 'bookmark':if(tab?.kind==='web')await action('bookmarkTab',{id:tab.id,before:null});break;
 }
 return true;
}
api.onKey(key=>void runShortcut(key));
document.addEventListener('keydown',event=>{
 if(event.defaultPrevented||event.isComposing)return;
 // Escape in the address bar puts the page's address back and returns to the page.
 if(event.key==='Escape'&&event.target.id==='address'){const tab=state?.tabs.find(t=>t.id===state.active);event.preventDefault();event.target.value=tab?.kind==='web'?tab.url:'';event.target.blur();void action('focus','page');return;}
 const key={key:event.key,ctrl:event.ctrlKey||event.metaKey,shift:event.shiftKey,alt:event.altKey};
 if(!shortcut(key))return;
 event.preventDefault();void runShortcut(key);
});
void action('state');

// Sidebar resizing. The CSS width follows the pointer at once; the page view
// follows once per frame, and the width is saved when the drag ends.
let sidebarDrag=null;
document.addEventListener('pointerdown',event=>{
 if(event.button!==0||!event.target.closest('.sidebar-resizer'))return;
 event.preventDefault();
 sidebarDrag={x:event.clientX,start:state.sidebarWidth,width:state.sidebarWidth,frame:0,pointer:event.pointerId};
 try{event.target.setPointerCapture(event.pointerId);}catch{}
 document.body.classList.add('sidebar-resizing');
});
document.addEventListener('pointermove',event=>{
 if(!sidebarDrag||event.pointerId!==sidebarDrag.pointer)return;
 sidebarDrag.width=clampSidebar(sidebarDrag.start+(event.clientX-sidebarDrag.x)*(state.sidebarSide==='right'?-1:1));
 document.documentElement.style.setProperty('--sidebar-width',sidebarDrag.width+'px');
 if(!sidebarDrag.frame)sidebarDrag.frame=requestAnimationFrame(()=>{if(!sidebarDrag)return;sidebarDrag.frame=0;void api.action('sidebarWidth',sidebarDrag.width);});
});
function endSidebarDrag(){
 if(!sidebarDrag)return;
 const width=sidebarDrag.width;cancelAnimationFrame(sidebarDrag.frame);sidebarDrag=null;
 document.body.classList.remove('sidebar-resizing');
 void action('preferences',{sidebarWidth:width});
}
document.addEventListener('pointerup',endSidebarDrag);
document.addEventListener('pointercancel',endSidebarDrag);
document.addEventListener('dblclick',event=>{if(event.target.closest('.sidebar-resizer'))void action('preferences',{sidebarWidth:sidebarWidths.default});});

// Item panel resizing, from its edge facing the page area: dragging away from
// the panel widens it (at the bottom, heightens it).
let itemDrag=null;
document.addEventListener('pointerdown',event=>{
 if(event.button!==0||!event.target.closest('.item-resizer'))return;
 event.preventDefault();
 const bottom=state.itemDock==='bottom',sign=state.itemDock==='left'?1:-1;
 itemDrag={bottom,sign,x:bottom?event.clientY:event.clientX,start:bottom?state.itemPanelHeight:state.itemPanelWidth,width:bottom?state.itemPanelHeight:state.itemPanelWidth,frame:0,pointer:event.pointerId};
 try{event.target.setPointerCapture(event.pointerId);}catch{}
 document.body.classList.add('item-resizing');
});
document.addEventListener('pointermove',event=>{
 if(!itemDrag||event.pointerId!==itemDrag.pointer)return;
 const clamp=itemDrag.bottom?clampItemPanelHeight:clampItemPanel;
 itemDrag.width=clamp(itemDrag.start+itemDrag.sign*((itemDrag.bottom?event.clientY:event.clientX)-itemDrag.x));
 document.documentElement.style.setProperty(itemDrag.bottom?'--item-height':'--item-width',itemDrag.width+'px');
 if(!itemDrag.frame)itemDrag.frame=requestAnimationFrame(()=>{if(!itemDrag)return;itemDrag.frame=0;void api.action(itemDrag.bottom?'itemPanelHeight':'itemPanelWidth',itemDrag.width);});
});
function endItemDrag(){
 if(!itemDrag)return;
 const {width,bottom}=itemDrag;cancelAnimationFrame(itemDrag.frame);itemDrag=null;
 document.body.classList.remove('item-resizing');
 void action('preferences',bottom?{itemPanelHeight:width}:{itemPanelWidth:width});
}
document.addEventListener('pointerup',endItemDrag);
document.addEventListener('pointercancel',endItemDrag);
document.addEventListener('dblclick',event=>{if(event.target.closest('.item-resizer'))void action('preferences',state.itemDock==='bottom'?{itemPanelHeight:itemPanelHeights.default}:{itemPanelWidth:itemPanelWidths.default});});

document.addEventListener('error',event=>{
 const img=event.target;if(img.tagName!=='IMG')return;
 if(img.classList.contains('favicon'))img.outerHTML=icon('globe','tab-icon');
 else if(img.parentElement?.classList.contains('bookmark-avatar'))img.remove();
},true);

document.addEventListener('contextmenu',event=>{
 const item=event.target.closest('.sidebar-bookmark');if(!item)return;
 event.preventDefault();openContextMenu(item.dataset.sidebarBookmark,event.clientX,event.clientY);
});
document.addEventListener('keydown',event=>{
 if(placeMenu&&event.key==='Escape'){event.preventDefault();closePlaceMenu();document.querySelector('.place-toggle')?.focus();return;}
 if(!contextMenu)return;
 if(event.key==='Escape'){event.preventDefault();closeContextMenu();return;}
 if(event.key==='ArrowDown'||event.key==='ArrowUp'){
  event.preventDefault();const items=[...document.querySelectorAll('.context-menu button')];
  const i=items.indexOf(document.activeElement);items[(i+(event.key==='ArrowDown'?1:items.length-1))%items.length]?.focus();
 }
});
window.addEventListener('blur',closeContextMenu);
window.addEventListener('blur',closePlaceMenu);
window.addEventListener('resize',closePlaceMenu);

// Dragging bookmarks onto the sidebar's bookmark section pins them there; a
// line marks where the bookmark will land.
const bookmarkType='text/mayak-bookmark';
function clearBookmarkDrop(){document.querySelectorAll('.drop-before,.drop-end').forEach(el=>el.classList.remove('drop-before','drop-end'));}
document.addEventListener('dragstart',event=>{
 const item=event.target.closest?.('[data-bookmark],[data-sidebar-bookmark]');if(!item)return;
 event.dataTransfer.setData(bookmarkType,item.dataset.bookmark||item.dataset.sidebarBookmark);event.dataTransfer.effectAllowed='copyMove';
 document.body.classList.add('bookmark-dragging');
});
document.addEventListener('dragend',()=>{document.body.classList.remove('bookmark-dragging');clearBookmarkDrop();});
const pinnedBefore=(zone,x,y)=>[...zone.querySelectorAll('.sidebar-bookmark')].find(item=>{const r=item.getBoundingClientRect();return y<r.top||(y<=r.bottom&&x<r.left+r.width/2);});
function bookmarkDropTarget(event){
 const zone=event.target.closest?.('.sidebar-bookmarks');if(!zone)return null;
 return {zone,before:pinnedBefore(zone,event.clientX,event.clientY)};
}
// Tabs dragged onto the pinned bookmarks (or the bookmarks icon) are
// bookmarked and pinned there. Only web pages can be bookmarked.
let tabBookmarkBefore=null;
function tabOverBookmarks(id,x,y){
 clearBookmarkDrop();
 if(state.tabs.find(t=>t.id===id)?.kind!=='web')return false;
 const hit=el=>{const r=el?.getBoundingClientRect();return !!r&&r.width>0&&x>=r.left-4&&x<=r.right+4&&y>=r.top-4&&y<=r.bottom+4;};
 const zone=document.querySelector('.sidebar-bookmarks'),open=document.querySelector('.bookmarks-open');
 if(hit(zone)){const before=pinnedBefore(zone,x,y);if(before)before.classList.add('drop-before');else zone.classList.add('drop-end');tabBookmarkBefore=before?.dataset.sidebarBookmark??null;return true;}
 if(hit(open)){open.classList.add('drop-end');tabBookmarkBefore=null;return true;}
 return false;
}
document.addEventListener('dragover',event=>{
 if(!event.dataTransfer.types.includes(bookmarkType))return;
 const drop=bookmarkDropTarget(event);clearBookmarkDrop();if(!drop)return;
 event.preventDefault();event.dataTransfer.dropEffect='move';
 if(drop.before)drop.before.classList.add('drop-before');else drop.zone.classList.add('drop-end');
});
document.addEventListener('drop',event=>{
 const drop=event.dataTransfer.types.includes(bookmarkType)&&bookmarkDropTarget(event);clearBookmarkDrop();document.body.classList.remove('bookmark-dragging');if(!drop)return;
 event.preventDefault();
 void action('bookmarkPin',{id:event.dataTransfer.getData(bookmarkType),pin:true,before:drop.before?.dataset.sidebarBookmark??null});
});
