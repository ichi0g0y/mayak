import {mapTabID} from './state.js';
import {esc,t,appVersion,icon,render,api,state,action} from './shell-core.js';

// The first-run tutorial: an overlay that walks through the setup after
// installing. It shows on the Host until it is finished or skipped
// (state.tutorialDone) and can be reopened from the appearance settings.

export let tutorialOpen=false,tutorialStep=0,tutorialSettings=null;

const tutorialSteps=['welcome','folders','key','map','remote','tracker','done'];
export function tutorialHTML(){
 const key=tutorialSteps[tutorialStep],last=tutorialStep===tutorialSteps.length-1,first=tutorialStep===0;
 const body={
  welcome:`<p>${esc(t('tutWelcome'))}</p>${appVersion?`<p class="hint">MAYAK ${esc(appVersion)}</p>`:''}`,
  folders:`<p>${esc(t('tutFolders'))}</p><div class="tutorial-status"><span>${esc(t('tutFoldersScreens'))}: <code>${esc(tutorialSettings?.screenshotDirectory||t('tutNotFound'))}</code></span><span>${esc(t('tutFoldersLogs'))}: <code>${esc(tutorialSettings?.logsDirectory||t('tutNotFound'))}</code></span></div><button data-action="tutorialFolders">${icon('folder')}${esc(t('tutOpenFolders'))}</button>`,
  key:`<p>${esc(t('tutKey'))}</p><p class="hint">${esc(t('tutKeyNote'))}</p>`,
  map:`<p>${esc(t('tutMap'))}</p><button data-action="tutorialMap">${icon('map')}${esc(t('tutOpenMap'))}</button>`,
  remote:`<p>${esc(t('tutRemote'))}</p><button data-action="tutorialRemote">${icon('linked')}${esc(t('tutOpenRemote'))}</button>`,
  tracker:`<p>${esc(t('tutTracker'))}</p><button data-action="tutorialTracker">${icon('tracker')}${esc(t('tutOpenTracker'))}</button>`,
  done:`<p>${esc(t('tutDone'))}</p>`,
 }[key];
 const title=t({welcome:'tutWelcomeTitle',folders:'tutFoldersTitle',key:'tutKeyTitle',map:'tutMapTitle',remote:'tutRemoteTitle',tracker:'tutTrackerTitle',done:'tutDoneTitle'}[key]);
 const dots=tutorialSteps.map((_,i)=>`<i class="${i===tutorialStep?'on':''}"></i>`).join('');
 const nav=first?`<div><button data-action="tutorialSkip">${esc(t('tutLater'))}</button></div><div><button class="primary" data-action="tutorialNext">${esc(t('tutStart'))}</button></div>`
  :last?`<div></div><div><button data-action="tutorialBack">${esc(t('tutBack'))}</button><button class="primary" data-action="tutorialFinish">${esc(t('tutFinish'))}</button></div>`
  :`<div><button data-action="tutorialSkip">${esc(t('tutSkip'))}</button></div><div><button data-action="tutorialBack">${esc(t('tutBack'))}</button><button class="primary" data-action="tutorialNext">${esc(t('tutNext'))}</button></div>`;
 return `<div class="tutorial-backdrop" data-action="tutorialSkip"></div><section class="tutorial" role="dialog" aria-modal="true" aria-labelledby="tutorial-title"><p class="tutorial-kicker">${esc(t('tutStep'))} ${tutorialStep+1} / ${tutorialSteps.length}</p><h2 id="tutorial-title">${esc(title)}</h2>${body}<div class="tutorial-steps">${dots}</div><div class="tutorial-actions">${nav}</div></section>`;
}
export async function openTutorial(){
 tutorialOpen=true;tutorialStep=0;tutorialSettings=null;
 render();
 // The native web views sit above the shell; hide them while the overlay shows.
 await api.action('overlay',true);
 try{tutorialSettings=await window.mayakDesktop?.backend?.GetSettings?.();}catch{tutorialSettings=null;}
 if(tutorialOpen)render();
}
async function closeTutorial(){
 if(!tutorialOpen)return;
 tutorialOpen=false;
 await api.action('overlay',false);
 if(!state.tutorialDone)await action('preferences',{tutorialDone:true});
 render();
}
export async function handleTutorial(type){
 switch(type){
  case 'tutorial':return openTutorial();
  case 'tutorialNext':tutorialStep=Math.min(tutorialSteps.length-1,tutorialStep+1);render();return;
  case 'tutorialBack':tutorialStep=Math.max(0,tutorialStep-1);render();return;
  case 'tutorialSkip':case 'tutorialFinish':return closeTutorial();
  case 'tutorialMap':await closeTutorial();return action('activate',mapTabID);
  case 'tutorialFolders':case 'tutorialRemote':case 'tutorialTracker':{
   await closeTutorial();await action('settings');
   return action('settingsSection',{tutorialFolders:'folders',tutorialRemote:'remote',tutorialTracker:'tracker'}[type]);
  }
 }
}
document.addEventListener('keydown',event=>{
 if(!tutorialOpen)return;
 if(event.key==='Escape')void closeTutorial();
 else if(event.key==='Enter'||event.key==='ArrowRight')void handleTutorial(tutorialStep===tutorialSteps.length-1?'tutorialFinish':'tutorialNext');
 else if(event.key==='ArrowLeft')void handleTutorial('tutorialBack');
 else return;
 event.preventDefault();
});
