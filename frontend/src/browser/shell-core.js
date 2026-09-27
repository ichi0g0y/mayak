import {words} from './words.js';
import {esc,icon} from './icons.js';

// What every part of the browser shell shares: the desktop API, the current
// state, the render and action entry points, the words and the HTML helpers.
// The views (view-*.js) import from here and never from shell.js, so that
// shell.js can import them.

// The shell's state, a snapshot from api.js: every view reads it here and
// only setState (shell.js, on each state event) and action replace it.
export let state=null;
export const setState=next=>{state=next;};
// render is shell.js's; the views call it through this binding.
export let render=()=>{};
export const setRender=fn=>{render=fn;};
// The views' click handlers: (type,id,button,event) => handled.
export const clickHandlers=[];
// Functions run after each render, for what the markup cannot carry (a
// canvas's drawing, an image too large to put in the markup).
export const afterRenderHooks=[];

// The shell's API (api.js sets window.mayak), looked up when used, not when
// this module runs: in a build, a module can run before api.js has.
// A callback registered before api.js has run is handed over once it has
// (tried again every 10 ms, for up to 5 s).
const register=(name,fn,tries=0)=>{if(window.mayak)window.mayak[name](fn);else if(tries<500)setTimeout(()=>register(name,fn,tries+1),10);};
export const api={
 action:(type,data)=>window.mayak.action(type,data),
 request:(type,data)=>window.mayak.request(type,data),
 onState:fn=>register('onState',fn),
 onKey:fn=>register('onKey',fn),
 onMenu:fn=>register('onMenu',fn),
};
// The build's version, for the About section; empty in development. The
// desktop bridge (api.js) exists once the first state arrives, so it loads then.
export let appVersion='';
export async function loadVersion(){try{appVersion=(await window.mayakDesktop?.backend?.GetVersion?.())||'';}catch{}if(appVersion)render();}
export const t=key=>words[state?.language||'ja'][key]||key;
export {esc,icon};
export const option=(value,label,current)=>`<option value="${esc(value)}" ${current===value?'selected':''}>${esc(label)}</option>`;
export function select(key,label,choices,value,scope='preferences'){return `<label class="field"><span>${esc(label)}</span><select data-scope="${scope}" data-key="${key}">${choices.map(([v,l])=>option(v,l,value)).join('')}</select></label>`;}
export const siteChoices=()=>[['tarkov-dev','tarkov.dev'],['official-wiki',t('official')],['japanese-wiki',t('japanese')]];

export async function action(type,data){const next=await api.action(type,data);if(next){state=next;render();}return next;}
// request is action that throws when the action fails (action turns a
// failure into the current state, for callers that need not know).
export async function request(type,data){const next=await api.request(type,data);if(next){state=next;render();}return next;}
