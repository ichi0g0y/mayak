import {age} from './item.js';
import {state,esc,t,icon,action,clickHandlers,render} from './shell-core.js';

// The screenshots page: the viewer with zoom and drag, the recognition
// badges and details.

// Zoom of the screenshot shown large: 1 fits it to the page area; x and y
// move it (in screen pixels) while it is larger. Kept in the shell, so
// zooming does not rebuild the page.
let shotZoom={name:'',scale:1,x:0,y:0};
const shotZoomMax=8;
const shotTransform=()=>`translate(${shotZoom.x}px,${shotZoom.y}px) scale(${shotZoom.scale})`;
function applyShotZoom(){
 const img=document.querySelector('.shot-viewer-body img'),level=document.querySelector('.shot-zoom-level');
 if(img)img.style.transform=shotTransform();
 if(level)level.textContent=Math.round(shotZoom.scale*100)+'%';
}
// zoomShot sets the scale, keeping the point at (px, py) from the image's
// centre where it is; at the fitted size it is centred again.
function zoomShot(scale,px=0,py=0){
 const next=Math.max(1,Math.min(shotZoomMax,scale));
 if(next===1){shotZoom={...shotZoom,scale:1,x:0,y:0};applyShotZoom();return;}
 const ratio=next/shotZoom.scale;
 shotZoom={...shotZoom,scale:next,x:px-(px-shotZoom.x)*ratio,y:py-(py-shotZoom.y)*ratio};
 applyShotZoom();
}
// The image's actual pixels on screen: the zoom that shows it at 100%.
function shotActualScale(){
 const img=document.querySelector('.shot-viewer-body img');
 return img&&img.naturalWidth?Math.max(1,img.naturalWidth/img.getBoundingClientRect().width*shotZoom.scale):2;
}
// The screenshot page: a grid of all of them, newest first; one opened is
// shown large over it, with the previous and next at hand.
export function screenshotsPage(){
 const shots=state.screenshots;
 if(!shots)return `<div class="page"><p class="empty-tabs">${esc(t('screenshotsHostOnly'))}</p></div>`;
 const grid=shots.list.map(s=>`<button class="shot-cell" data-action="screenshotView" data-id="${esc(s.name)}" title="${esc([s.name,shotSummary(s.meta)].filter(Boolean).join('\n'))}">${shots.thumbs[s.name]?`<img src="${shots.thumbs[s.name]}" alt="" loading="lazy">`:`<span class="shot-placeholder">${icon('image')}</span>`}<span class="shot-age">${esc(age(s.time,state.language))}</span>${shotBadges(s.meta)}</button>`).join('');
 return `<div class="page shots-page"><div class="bookmarks-head"><h1>${esc(t('screenshots'))}</h1><div class="bookmarks-tools"><button data-action="screenshotFolder">${icon('folder')}<span>${esc(t('openScreenshotFolder'))}</span></button></div></div>${shots.list.length?`<div class="shot-grid">${grid}</div>`:`<p class="shot-empty">${esc(t('noScreenshots'))}</p>`}</div>${shotViewer()}`;
}
// A screenshot's kind: a flea offer is an item screenshot of its own layout.
const shotKind=meta=>meta.type==='item'&&meta.layout==='flea-offer'?'offer':meta.type;
export function shotBadges(meta){
 if(!meta)return '';
 const kind=shotKind(meta);
 return `<span class="shot-kind" data-kind="${kind}">${esc(t('shotKind_'+kind))}</span>${meta.match?`<span class="shot-match">${esc(meta.match)}</span>`:''}`;
}
const shotSummary=meta=>meta?[t('shotKind_'+shotKind(meta)),meta.match,meta.confidence?Math.round(meta.confidence*100)+'%':''].filter(Boolean).join(' · '):'';
// The details of how a screenshot was recognized, beside it when shown large.
let shotInfoOpen=false;
function shotInfo(meta){
 const row=(label,value)=>value===''||value===undefined||value===null?'':`<div class="shot-info-row"><dt>${esc(label)}</dt><dd>${value}</dd></div>`;
 const pctText=v=>v?Math.round(v*100)+'%':'';
 return `<aside class="shot-info" aria-label="${esc(t('shotInfo'))}"><h3>${esc(t('shotInfo'))}</h3><dl>${[
  row(t('shotInfoKind'),esc(t('shotKind_'+shotKind(meta)))),
  row(t('shotInfoMatch'),esc([meta.match,meta.detail].filter(Boolean).join(' · '))),
  row(t('shotInfoConfidence'),pctText(meta.confidence)),
  row(t('shotInfoCandidates'),meta.candidates.map(c=>esc(c)).join('<br>')),
  row(t('shotInfoLayout'),esc(meta.layout)),
  row(t('shotInfoScore'),meta.score?meta.score.toFixed(2):''),
  row(t('shotInfoMap'),esc(meta.map)),
  row(t('shotInfoRaid'),esc(t(meta.raid?'yes':'no'))),
  row(t('shotInfoPosition'),esc(meta.position)),
  row(t('shotInfoStage'),esc(meta.stage)),
  row(t('shotInfoError'),esc(meta.error)),
 ].join('')}</dl>${meta.ocr?`<h4>OCR</h4><pre class="shot-ocr">${esc(meta.ocr)}</pre>`:''}</aside>`;
}
function shotViewer(){
 const shots=state.screenshots,name=shots?.viewing;
 if(!name)return '';
 // Another screenshot starts fitted to the page again.
 if(shotZoom.name!==name)shotZoom={name,scale:1,x:0,y:0};
 const index=shots.list.findIndex(s=>s.name===name),shot=shots.list[index];
 const image=shots.full||shots.thumbs[name];
 const newer=shots.list[index-1],older=shots.list[index+1];
 return `<div class="shot-viewer" role="dialog" aria-label="${esc(name)}"><div class="shot-viewer-head"><span class="shot-viewer-name" title="${esc(name)}">${esc(name)}</span><span class="shot-age">${esc(shot?age(shot.time,state.language):'')}</span>${shot?.meta?`<span class="shot-kind" data-kind="${shotKind(shot.meta)}">${esc(t('shotKind_'+shotKind(shot.meta)))}</span>${shot.meta.match?`<span class="shot-viewer-match" title="${esc(shot.meta.match)}">${esc(shot.meta.match)}${shot.meta.confidence?` · ${Math.round(shot.meta.confidence*100)}%`:''}</span>`:''}`:''}<div class="shot-zoom"><button class="shot-tool" data-action="shotZoom" data-id="out" title="${esc(t('zoomOut'))}" aria-label="${esc(t('zoomOut'))}">${icon('minus')}</button><span class="shot-zoom-level">${Math.round(shotZoom.scale*100)}%</span><button class="shot-tool" data-action="shotZoom" data-id="in" title="${esc(t('zoomIn'))}" aria-label="${esc(t('zoomIn'))}">${icon('plus')}</button><button class="shot-tool" data-action="shotZoom" data-id="fit" title="${esc(t('zoomFit'))}" aria-label="${esc(t('zoomFit'))}">${icon('fit')}</button>${shot?.meta?`<button class="shot-tool shot-info-toggle ${shotInfoOpen?'selected':''}" data-action="shotInfo" title="${esc(t('shotInfo'))}" aria-label="${esc(t('shotInfo'))}" aria-pressed="${shotInfoOpen}">${icon('info')}</button>`:''}</div><button class="shot-viewer-close" data-action="screenshotView" data-id="" title="${esc(t('close'))}" aria-label="${esc(t('close'))}">${icon('x')}</button></div><div class="shot-viewer-body">${shot?.meta&&shotInfoOpen?shotInfo(shot.meta):''}${image?`<img src="${image}" alt="" draggable="false" style="transform:${shotTransform()}">`:''}${newer?`<button class="shot-nav shot-newer" data-action="screenshotView" data-id="${esc(newer.name)}" title="${esc(t('newerScreenshot'))}" aria-label="${esc(t('newerScreenshot'))}">${icon('back')}</button>`:''}${older?`<button class="shot-nav shot-older" data-action="screenshotView" data-id="${esc(older.name)}" title="${esc(t('olderScreenshot'))}" aria-label="${esc(t('olderScreenshot'))}">${icon('forward')}</button>`:''}</div></div>`;
}

// The screenshot shown large zooms with the wheel around the pointer, moves
// by dragging while zoomed, and switches between fitted and actual size on a
// double-click.
const shotPoint=event=>{const body=document.querySelector('.shot-viewer-body').getBoundingClientRect();return [event.clientX-(body.left+body.width/2),event.clientY-(body.top+body.height/2)];};
document.addEventListener('wheel',event=>{
 if(!event.target.closest?.('.shot-viewer-body'))return;
 event.preventDefault();
 const [px,py]=shotPoint(event);
 zoomShot(shotZoom.scale*(event.deltaY<0?1.15:1/1.15),px,py);
},{passive:false});
let shotDrag=null;
document.addEventListener('pointerdown',event=>{
 if(event.button!==0||shotZoom.scale<=1||!event.target.closest?.('.shot-viewer-body img'))return;
 event.preventDefault();
 shotDrag={pointer:event.pointerId,x:event.clientX-shotZoom.x,y:event.clientY-shotZoom.y};
 try{document.documentElement.setPointerCapture(event.pointerId);}catch{}
 document.documentElement.classList.add('shot-dragging');
});
document.addEventListener('pointermove',event=>{
 if(!shotDrag||event.pointerId!==shotDrag.pointer)return;
 shotZoom={...shotZoom,x:event.clientX-shotDrag.x,y:event.clientY-shotDrag.y};applyShotZoom();
});
const endShotDrag=event=>{if(shotDrag&&event.pointerId===shotDrag.pointer){shotDrag=null;document.documentElement.classList.remove('shot-dragging');}};
document.addEventListener('pointerup',endShotDrag);
document.addEventListener('pointercancel',endShotDrag);
document.addEventListener('dblclick',event=>{
 if(!event.target.closest?.('.shot-viewer-body img'))return;
 const [px,py]=shotPoint(event);
 zoomShot(shotZoom.scale>1?1:shotActualScale(),px,py);
});
// On the screenshot page, the arrows page through the one shown large and
// Escape closes it; + and - zoom, 0 fits it again.
document.addEventListener('keydown',event=>{
 const shots=state?.screenshots;
 if(!shots?.viewing||state.tabs.find(t=>t.id===state.active)?.kind!=='screenshots'||event.target.closest?.('input,textarea,select'))return;
 const index=shots.list.findIndex(s=>s.name===shots.viewing);
 if(['+','=',';'].includes(event.key)||event.key==='-'||event.key==='0'){event.preventDefault();zoomShot(event.key==='0'?1:shotZoom.scale*(event.key==='-'?0.8:1.25));return;}
 const next=event.key==='Escape'?'':event.key==='ArrowLeft'?shots.list[index-1]?.name:event.key==='ArrowRight'?shots.list[index+1]?.name:undefined;
 if(next===undefined)return;
 event.preventDefault();void action('screenshotView',next);
});

clickHandlers.push(async(type,id,button,event)=>{
  if(type==='shotInfo'){shotInfoOpen=!shotInfoOpen;render();return true;}
  if(type==='shotZoom'){zoomShot(id==='fit'?1:shotZoom.scale*(id==='in'?1.25:0.8));return true;}
  return false;
});
