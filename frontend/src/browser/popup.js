// The item popup's window: its header, above the page (a native view the Go
// side places below it). The main window's shell decides what it shows and
// handles its buttons; this page only draws the header and passes clicks on.
import './style.css';
import {Events,Browser} from '@wailsio/runtime';
import {BrowserPopupPin} from '../../bindings/github.com/local/mayak/internal/app/app';
import {t as word} from './words.js';

// Lucide icon paths (ISC), as in the shell.
const icons={
 task:'<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 13h4"/><path d="M10 17h4"/>',
 globe:'<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/>',
 plus:'<path d="M5 12h14"/><path d="M12 5v14"/>',
 external:'<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h3"/>',
 x:'<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
 pin:'<path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/>',
};
const icon=(name,cls='icon')=>`<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name]}</svg>`;
const esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

let page={url:'',title:'',task:false,theme:'',language:'ja',pinned:false,loading:false};
function render(){
 const t=key=>word(page.language,key);
 if(page.theme)document.documentElement.dataset.theme=page.theme;
 document.documentElement.lang=page.language;
 document.title=page.title||'MAYAK';
 document.querySelector('#app').innerHTML=`<header class="popup-head popup-window-head">${icon(page.task?'task':'globe','popup-icon')}<span class="popup-title" title="${esc(page.url)}">${esc(page.title)}</span><button class="popup-icon-button popup-pin ${page.pinned?'pinned':''}" data-action="pin" title="${esc(t(page.pinned?'popupUnpin':'popupPin'))}" aria-label="${esc(t(page.pinned?'popupUnpin':'popupPin'))}" aria-pressed="${page.pinned}">${icon('pin')}</button><button data-action="toTab">${icon('plus')}<span>${esc(t('openInTab'))}</span></button><button class="popup-icon-button" data-action="external" title="${esc(t('openExternalBrowser'))}" aria-label="${esc(t('openExternalBrowser'))}">${icon('external')}</button><button class="popup-icon-button" data-action="close" title="${esc(t('close'))}" aria-label="${esc(t('close'))}">${icon('x')}</button>${page.loading?`<span class="load-bar" style="--load-offset:-${Date.now()%1400}ms"></span>`:''}</header>`;
}
Events.On('popup:page',event=>{page={...page,...event.data};render();});
// Following links inside the page keeps the title current.
Events.On('browser:navigation',event=>{const e=event.data;if(e?.id!=='popup')return;page.url=e.url||page.url;page.title=e.title||page.title;page.loading=!!e.loading;render();});
document.addEventListener('click',event=>{
 const type=event.target.closest('[data-action]')?.dataset.action;
 if(type==='external'){if(/^https?:\/\//.test(page.url))void Browser.OpenURL(page.url);return;}
 // Pinned, the popup stays when something else is clicked.
 if(type==='pin'){page.pinned=!page.pinned;render();void BrowserPopupPin(page.pinned);return;}
 if(type)void Events.Emit('popup:action',type);
});
document.addEventListener('keydown',event=>{if(event.key==='Escape')void Events.Emit('popup:action','close');});
render();
