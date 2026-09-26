package app

// Scripts run in a translated page (translate.goog) around a snap note's
// capture, so the capture shows the page without Google Translate's bar.

// hideTranslateBar hides the bar: the elements Google Translate adds (ids
// starting with "gt-", its frames, "skiptranslate"), and the room the page
// keeps for it at the top (a margin, padding or offset of the page as high
// as the bar). It resolves once the page has drawn itself again.
const hideTranslateBar = `(()=>{const hidden=[],fixed=[];let h=0;
const bars=[...document.querySelectorAll('[id^="gt-"],iframe[src*="translate"],.skiptranslate')].filter(el=>el!==document.documentElement&&el!==document.body);
for(const el of bars){const r=el.getBoundingClientRect();if(r.top<=1&&r.height>0&&r.height<200)h=Math.max(h,r.bottom);hidden.push([el,el.style.cssText]);el.style.setProperty('display','none','important');}
if(h>0)for(const el of [document.documentElement,document.body]){const cs=getComputedStyle(el);for(const p of ['margin-top','padding-top','top']){const v=parseFloat(cs.getPropertyValue(p))||0;if(Math.abs(v-h)<=2){fixed.push([el,p,el.style.getPropertyValue(p),el.style.getPropertyPriority(p)]);el.style.setProperty(p,'0px','important');}}}
window.__mayakSnap={hidden,fixed};
return new Promise(done=>requestAnimationFrame(()=>requestAnimationFrame(()=>done(bars.length))));})()`

// showTranslateBar puts back what hideTranslateBar changed.
const showTranslateBar = `(()=>{const s=window.__mayakSnap;if(!s)return 0;
for(const [el,css] of s.hidden)el.style.cssText=css;
for(const [el,p,v,pr] of s.fixed){if(v)el.style.setProperty(p,v,pr);else el.style.removeProperty(p);}
delete window.__mayakSnap;return 1;})()`
