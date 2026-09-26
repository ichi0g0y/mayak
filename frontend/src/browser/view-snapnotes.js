import {state,api,t,esc,icon,action,render,clickHandlers,afterRenderHooks} from './shell-core.js';
import {hostname,originalURL} from './state.js';
import {age} from './item.js';

// Snap notes: a capture of the page in view (or a pasted picture, an image
// file, a blank sheet) with drawing over it. The notes live on this computer
// (internal/snapnote); the shell has their list in state.snapNotes and the
// note open for drawing in state.snapNotes.open. A note taken from a page is
// linked to it: the page's toolbar button counts them, and the note opens
// the page again. Unlinked notes keep their page on record.

// The page a note belongs to: the original of a translated page, without
// its #fragment or a trailing slash.
export function snapPageKey(url){
 try{const u=new URL(originalURL(url)||url);return u.origin+u.pathname.replace(/\/+$/,'')+u.search;}catch{return '';}
}
const pageNotes=url=>{const key=snapPageKey(url);return key?(state.snapNotes?.list||[]).filter(n=>n.linked&&snapPageKey(n.url)===key):[];};
const noteTime=note=>age(note.updatedAt||note.createdAt,state.language);
const thumbHTML=note=>{const src=state.snapNotes?.thumbs?.[note.id];return src?`<img src="${src}" alt="">`:`<span class="shot-placeholder">${icon('image')}</span>`;};
const siteLine=note=>note.url?`${esc(hostname(note.url))}${note.linked?'':` · ${esc(t('snapUnlinked'))}`}`:esc(t('snapSingle'));

// The toolbar button of a web page: it opens the snap menu, and counts the
// page's notes.
export function snapButton(tab){
 if(!state.snapNotes||tab?.kind!=='web')return '';
 const count=pageNotes(tab.url).length;
 return `<button class="snap-page${count?' has-notes':''}${menu?' on':''}" data-action="snapMenu" title="${esc(t('snapTake'))}" aria-label="${esc(t('snapTake'))}" aria-haspopup="menu" ${state.snapNotes.busy?'disabled':''}>${icon('brush')}${count?`<span class="snap-count">${count}</span>`:''}</button>`;
}

// The menu under the button: the two captures, the page's notes and the list.
let menu=null;
const menuWidth=300;
function openMenu(button){
 const r=button.getBoundingClientRect();
 menu={left:Math.round(Math.max(4,Math.min(r.right-menuWidth,innerWidth-menuWidth-4))),top:Math.round(r.bottom+6)};
 render();
 // The page's native view is above the shell: it steps aside while the menu shows.
 void api.action('overlay',true);
}
function closeMenu(){if(!menu)return;menu=null;render();void api.action('overlay',false);}
export function snapMenuHTML(){
 if(!menu)return '';
 const tab=state.tabs.find(t=>t.id===state.active);
 const notes=tab?pageNotes(tab.url).slice(0,6):[];
 const item=(id,iconName,title,hint)=>`<button class="snap-menu-item" role="menuitem" data-action="snapCapture" data-id="${id}">${icon(iconName)}<span><strong>${esc(title)}</strong><small>${esc(hint)}</small></span></button>`;
 const list=notes.length?`<div class="snap-menu-label">${esc(t('snapThisPage'))}</div>${notes.map(n=>`<button class="snap-menu-note" role="menuitem" data-action="snapOpen" data-id="${esc(n.id)}"><span class="snap-menu-thumb">${thumbHTML(n)}</span><span><strong>${esc(n.title)}</strong><small>${esc(noteTime(n))}</small></span></button>`).join('')}`:'';
 return `<div class="context-backdrop" data-action="closeSnapMenu"></div><div class="snap-menu" role="menu" aria-label="${esc(t('snapNotes'))}" style="left:${menu.left}px;top:${menu.top}px;width:${menuWidth}px">${item('visible','scan',t('snapVisible'),t('snapVisibleHint'))}${item('full','file',t('snapFull'),t('snapFullHint'))}${list}<button class="snap-menu-all" role="menuitem" data-action="snapnotes">${icon('image')}${esc(t('snapShowAll'))}</button></div>`;
}

// The sidebar section: the latest note, and the button to the list.
export function snapSection(){
 const notes=state.snapNotes;
 if(!notes)return '';
 const open=state.tabs.find(tab=>tab.id===state.active)?.kind==='snapnotes';
 const folded=state.snapNotesCollapsed;
 const latest=notes.list[0];
 const preview=latest?`<button class="shot-latest snap-latest" data-action="snapOpen" data-id="${esc(latest.id)}" title="${esc(latest.title)}">${thumbHTML(latest)}<span class="shot-age">${esc(noteTime(latest))}</span></button>`:`<p class="shot-empty">${esc(t('snapNewBlank'))}…</p>`;
 return `<div class="section-label screenshot-section-label snap-section-label ${open?'active':''}"><button class="section-link" data-action="toggleSnapSection" aria-expanded="${!folded}" title="${esc(t(folded?'expandSection':'collapseSection'))}">${esc(t('snapNotes'))}${icon('chevron','section-chevron')}</button><button class="new-tab shots-open" data-action="snapnotes" title="${esc(t('snapNotesAll'))}" aria-label="${esc(t('snapNotesAll'))}" aria-pressed="${open}">${icon('brush')}</button></div>${folded?'':`<div class="shot-section">${preview}</div>`}`;
}

// The page: the list of notes, or the note open for drawing.
export function snapNotesPage(){
 const notes=state.snapNotes;
 if(!notes)return `<div class="page"><p class="empty-tabs">${esc(t('snapEmpty'))}</p></div>`;
 return notes.open?editorHTML(notes.open):listHTML(notes);
}
function listHTML(notes){
 const filter=notes.filter||'all';
 const shown=notes.list.filter(n=>filter==='all'||(filter==='linked'?n.linked:!n.linked));
 const chip=(value,label)=>`<button class="segment${filter===value?' selected':''}" data-action="snapFilter" data-id="${value}" aria-pressed="${filter===value}">${esc(t(label))}</button>`;
 const cards=shown.map(n=>`<button class="snap-card" data-action="snapOpen" data-id="${esc(n.id)}" title="${esc(n.title)}"><span class="snap-card-thumb">${thumbHTML(n)}</span><span class="snap-card-text"><strong>${esc(n.title)}</strong><small>${siteLine(n)} · ${esc(noteTime(n))}</small></span></button>`).join('');
 return `<div class="page snap-page-list"><div class="bookmarks-head"><h1>${esc(t('snapNotes'))}</h1><div class="bookmarks-tools"><button data-action="snapNewBlank">${icon('plus')}<span>${esc(t('snapNewBlank'))}</span></button><button data-action="snapNewFile">${icon('image')}<span>${esc(t('snapNewImage'))}</span></button><input type="file" id="snap-file" accept="image/*" hidden></div></div><div class="snap-filters"><div class="segmented" role="group">${chip('all','snapFilterAll')}${chip('linked','snapFilterLinked')}${chip('single','snapFilterSingle')}</div><p class="hint">${esc(t('snapPasteHint'))}</p></div>${notes.list.length?(shown.length?`<div class="snap-grid">${cards}</div>`:`<p class="shot-empty">${esc(t('snapNoMatch'))}</p>`):`<p class="shot-empty">${esc(t('snapEmpty'))}</p>`}</div>`;
}

// The editor. Strokes are in the image's pixels: {c: colour, w: width, p:
// [[x,y],…]}. The image and the ink canvas keep what they show across
// renders (data-keep); afterRender paints them.
const colors=['#ff3b30','#ffd60a','#34c759','#32ade6','#ffffff','#111111'];
const sizes=[['s',3],['m',6],['l',12]];
let tool='pen',color=colors[0],size='m',zoom='fit',armedDelete='',saving=false;
let ed=null;
const lineWidth=note=>sizes.find(([k])=>k===size)[1]*Math.max(1,note.width/1000);
// The ink canvas is at most about 24 million pixels; a larger image is inked
// at a smaller scale.
const inkScale=note=>Math.min(1,Math.sqrt(24e6/(note.width*note.height)));

function editorHTML(open){
 const note=open.note;
 const scale=inkScale(note);
 const tools=`<div class="snap-tools" role="toolbar"><button class="snap-tool${tool==='pen'?' on':''}" data-action="snapTool" data-id="pen" title="${esc(t('snapPen'))}" aria-pressed="${tool==='pen'}">${icon('brush')}</button><button class="snap-tool${tool==='eraser'?' on':''}" data-action="snapTool" data-id="eraser" title="${esc(t('snapEraser'))}" aria-pressed="${tool==='eraser'}">${icon('eraser')}</button><span class="snap-sep"></span>${colors.map(c=>`<button class="snap-color${color===c&&tool==='pen'?' on':''}" data-action="snapColor" data-id="${c}" title="${esc(t('snapColor'))} ${c}" style="--swatch:${c}"></button>`).join('')}<span class="snap-sep"></span>${sizes.map(([k,w])=>`<button class="snap-size${size===k?' on':''}" data-action="snapSize" data-id="${k}" title="${esc(t('snapSize'+k.toUpperCase()))}"><span style="--dot:${w+2}px"></span></button>`).join('')}<span class="snap-sep"></span><button data-action="snapUndo" title="${esc(t('snapUndo'))}" ${ed?.undo.length?'':'disabled'}>${icon('undo')}</button><button data-action="snapRedo" title="${esc(t('snapRedo'))}" ${ed?.redo.length?'':'disabled'}>${icon('redo')}</button><button data-action="snapClear" title="${esc(t('snapClear'))}" ${ed?.strokes.length?'':'disabled'}>${icon('trash')}</button><span class="snap-sep"></span><button data-action="snapZoom" title="${esc(t(zoom==='fit'?'snapActual':'snapFit'))}">${icon('fit')}</button></div>`;
 const link=note.url?(note.linked?`<button data-action="snapLinkToggle" title="${esc(t('snapUnlink'))}">${icon('unlinked')}<span>${esc(t('snapUnlink'))}</span></button>`:`<button data-action="snapLinkToggle" title="${esc(t('snapRelink'))}">${icon('linked')}<span>${esc(t('snapRelink'))}</span></button>`):'';
 const page=note.url?`<button data-action="snapOpenPage" data-id="${esc(note.id)}" title="${esc(note.url)}">${icon('external')}<span>${esc(t('snapOpenPage'))}</span></button>`:'';
 const status=saving?t('snapSaving'):ed?.dirty?'':t('snapSaved');
 return `<div class="page snap-editor"><div class="snap-head"><button data-action="snapBack" title="${esc(t('snapBack'))}" aria-label="${esc(t('snapBack'))}">${icon('back')}</button><input id="snap-title" class="snap-title" value="${esc(ed?.id===note.id?ed.title:note.title)}" aria-label="${esc(t('snapTitle'))}" maxlength="160"><span class="snap-status">${esc(status)}</span><div class="snap-actions">${page}${link}<button class="snap-delete${armedDelete===note.id?' armed':''}" data-action="snapDeleteNote" data-id="${esc(note.id)}">${icon('trash')}<span>${esc(t(armedDelete===note.id?'snapDeleteConfirm':'snapDelete'))}</span></button></div></div><p class="snap-where">${note.url?`${esc(t(note.linked?'snapLinked':'snapUnlinked'))}: <span title="${esc(note.url)}">${esc(note.pageTitle||note.url)}</span>`:esc(t('snapNoPage'))}</p>${tools}<div class="snap-stage ${zoom}" data-tool="${tool}"><div class="snap-sheet" style="width:${zoom==='fit'?'100%':note.width+'px'};max-width:${zoom==='fit'?note.width+'px':'none'};aspect-ratio:${note.width}/${note.height}"><img class="snap-base" data-keep="${esc(note.id)}" alt="" draggable="false"><canvas class="snap-ink" data-keep="${esc(note.id)}" width="${Math.round(note.width*scale)}" height="${Math.round(note.height*scale)}"></canvas></div></div></div>`;
}

// The editor follows the note open: a new one starts with its strokes; a
// note left with unsaved drawing is saved first.
function syncEditor(){
 const open=state.snapNotes?.open;
 if(ed&&(!open||open.note.id!==ed.id)){if(ed.dirty)void saveNow(ed);ed=null;armedDelete='';}
 if(open&&!ed){
  ed={id:open.note.id,note:open.note,strokes:open.strokes.filter(s=>s&&Array.isArray(s.p)),undo:[],redo:[],dirty:false,title:open.note.title,img:null,version:1,thumbed:!!state.snapNotes.thumbs[open.note.id]};
  const img=new Image();const mine=ed;
  img.onload=()=>{if(ed!==mine)return;mine.img=img;mine.version++;paint();if(!mine.thumbed){mine.thumbed=true;void saveNow(mine);}};
  img.src=open.image;
 }
 if(ed&&open)ed.note=open.note;
}
function paint(){
 if(!ed)return;
 const base=document.querySelector('.snap-base');
 if(base&&ed.img&&base.dataset.src!==ed.id){base.src=ed.img.src;base.dataset.src=ed.id;}
 const canvas=document.querySelector('.snap-ink');
 if(!canvas)return;
 // Only a change of the strokes redraws: a render in the middle of a line
 // must not wipe it.
 const version=ed.id+'/'+ed.version;
 if(canvas.dataset.v===version)return;
 canvas.dataset.v=version;
 const ctx=canvas.getContext('2d'),scale=canvas.width/ed.note.width;
 ctx.clearRect(0,0,canvas.width,canvas.height);
 for(const s of ed.strokes)drawStroke(ctx,s,scale);
}
function drawStroke(ctx,s,scale){
 if(!s.p.length)return;
 ctx.save();ctx.scale(scale,scale);ctx.lineCap='round';ctx.lineJoin='round';ctx.strokeStyle=s.c;ctx.lineWidth=s.w;
 ctx.beginPath();ctx.moveTo(s.p[0][0],s.p[0][1]);
 if(s.p.length===1)ctx.lineTo(s.p[0][0]+0.01,s.p[0][1]);
 for(let i=1;i<s.p.length;i++)ctx.lineTo(s.p[i][0],s.p[i][1]);
 ctx.stroke();ctx.restore();
}
afterRenderHooks.push(()=>{syncEditor();paint();});
const repaint=()=>{if(!ed)return;ed.version++;paint();};

// Saving: a moment after the last change, with a small picture for the lists
// (the top of the image, drawing included).
let saveTimer=0;
function changed(){if(!ed)return;ed.dirty=true;clearTimeout(saveTimer);const mine=ed;saveTimer=setTimeout(()=>void saveNow(mine),900);render();}
async function saveNow(target){
 if(!target)return;
 clearTimeout(saveTimer);
 target.dirty=false;saving=true;if(target===ed)render();
 let thumb='';
 if(target.img){
  const w=360,h=Math.min(Math.round(w*target.note.height/target.note.width),Math.round(w*10/16)),s=w/target.note.width;
  const c=document.createElement('canvas');c.width=w;c.height=h;const ctx=c.getContext('2d');
  ctx.fillStyle='#fff';ctx.fillRect(0,0,w,h);ctx.drawImage(target.img,0,0,w,target.note.height*s);
  for(const st of target.strokes)drawStroke(ctx,st,s);
  thumb=c.toDataURL('image/jpeg',0.82);
 }
 try{await action('snapSave',{id:target.id,title:target.title,strokes:target.strokes,thumb});}
 catch{target.dirty=true;}
 finally{saving=false;if(target===ed)render();}
}

// Drawing with the pointer: the pen adds a stroke, the eraser removes the
// strokes it touches.
let drawing=null;
function imagePoint(event,canvas){const r=canvas.getBoundingClientRect();return [Math.round((event.clientX-r.left)/r.width*ed.note.width*10)/10,Math.round((event.clientY-r.top)/r.height*ed.note.height*10)/10];}
function remember(){ed.undo.push(ed.strokes.slice());if(ed.undo.length>100)ed.undo.shift();ed.redo=[];}
function erase(point){
 const radius=10*Math.max(1,ed.note.width/1000);
 const keep=ed.strokes.filter(s=>!s.p.some(([x,y])=>Math.hypot(x-point[0],y-point[1])<=radius+s.w/2));
 if(keep.length===ed.strokes.length)return false;
 if(!drawing.erased){remember();drawing.erased=true;}
 ed.strokes=keep;repaint();return true;
}
document.addEventListener('pointerdown',event=>{
 const canvas=event.target.closest?.('.snap-ink');
 if(!canvas||!ed||event.button!==0)return;
 event.preventDefault();canvas.setPointerCapture(event.pointerId);
 const point=imagePoint(event,canvas);
 if(tool==='eraser'){drawing={pointer:event.pointerId,canvas,erase:true,erased:false};erase(point);return;}
 drawing={pointer:event.pointerId,canvas,stroke:{c:color,w:lineWidth(ed.note),p:[point]}};
 drawStroke(canvas.getContext('2d'),drawing.stroke,canvas.width/ed.note.width);
});
document.addEventListener('pointermove',event=>{
 if(!drawing||event.pointerId!==drawing.pointer||!ed)return;
 const point=imagePoint(event,drawing.canvas);
 if(drawing.erase){erase(point);return;}
 const p=drawing.stroke.p,last=p[p.length-1];
 if(Math.hypot(point[0]-last[0],point[1]-last[1])<1.5)return;
 p.push(point);
 const ctx=drawing.canvas.getContext('2d'),scale=drawing.canvas.width/ed.note.width;
 drawStroke(ctx,{c:drawing.stroke.c,w:drawing.stroke.w,p:[last,point]},scale);
});
const endDraw=event=>{
 if(!drawing||event.pointerId!==drawing.pointer)return;
 const done=drawing;drawing=null;
 if(!ed)return;
 if(done.erase){if(done.erased)changed();return;}
 remember();ed.strokes.push(done.stroke);ed.version++;done.canvas.dataset.v=ed.id+'/'+ed.version;changed();
};
document.addEventListener('pointerup',endDraw);
document.addEventListener('pointercancel',endDraw);

function undo(){if(!ed?.undo.length)return;ed.redo.push(ed.strokes);ed.strokes=ed.undo.pop();repaint();changed();}
function redo(){if(!ed?.redo.length)return;ed.undo.push(ed.strokes);ed.strokes=ed.redo.pop();repaint();changed();}
document.addEventListener('keydown',event=>{
 if(!ed||state.tabs.find(t=>t.id===state.active)?.kind!=='snapnotes'||event.target.closest?.('input,textarea'))return;
 const key=event.key.toLowerCase();
 if(event.ctrlKey&&!event.altKey&&(key==='z'||key==='y')){event.preventDefault();if(key==='y'||event.shiftKey)redo();else undo();}
});
document.addEventListener('input',event=>{if(event.target.id==='snap-title'&&ed){ed.title=event.target.value;changed();}});

// New notes from a picture: pasted, or an image file. Any image the browser
// reads is turned into PNG.
async function imageToPNG(blob){
 const bitmap=await createImageBitmap(blob);
 const c=document.createElement('canvas');c.width=bitmap.width;c.height=bitmap.height;c.getContext('2d').drawImage(bitmap,0,0);bitmap.close?.();
 return c.toDataURL('image/png');
}
async function newFromImage(blob,title){try{await action('snapNew',{image:await imageToPNG(blob),title});}catch{}}
function blankSheet(){const c=document.createElement('canvas');c.width=1600;c.height=1000;const ctx=c.getContext('2d');ctx.fillStyle='#ffffff';ctx.fillRect(0,0,c.width,c.height);return c.toDataURL('image/png');}
document.addEventListener('paste',event=>{
 if(state?.tabs.find(t=>t.id===state.active)?.kind!=='snapnotes'||event.target.closest?.('input,textarea'))return;
 const file=[...(event.clipboardData?.items||[])].find(i=>i.kind==='file'&&i.type.startsWith('image/'))?.getAsFile();
 if(file){event.preventDefault();void newFromImage(file,'');}
});
document.addEventListener('change',event=>{
 if(event.target.id!=='snap-file')return;
 const file=event.target.files?.[0];event.target.value='';
 if(file&&file.type.startsWith('image/'))void newFromImage(file,file.name.replace(/\.[^.]+$/,''));
});

clickHandlers.push(async(type,id,button)=>{
 if(type==='snapMenu'){if(menu)closeMenu();else openMenu(button);return true;}
 if(type==='closeSnapMenu'){closeMenu();return true;}
 if(type==='snapCapture'){closeMenu();void action('snapCapture',{full:id==='full'});return true;}
 if((type==='snapOpen'||type==='snapnotes')&&menu){closeMenu();void action(type,id);return true;}
 if(type==='toggleSnapSection'){void action('preferences',{snapNotesCollapsed:!state.snapNotesCollapsed});return true;}
 if(type==='snapNewBlank'){void action('snapNew',{image:blankSheet(),title:''});return true;}
 if(type==='snapNewFile'){document.querySelector('#snap-file')?.click();return true;}
 if(type==='snapTool'){tool=id==='eraser'?'eraser':'pen';render();return true;}
 if(type==='snapColor'){color=colors.includes(id)?id:colors[0];tool='pen';render();return true;}
 if(type==='snapSize'){size=sizes.some(([k])=>k===id)?id:'m';tool='pen';render();return true;}
 if(type==='snapUndo'){undo();return true;}
 if(type==='snapRedo'){redo();return true;}
 if(type==='snapClear'){if(ed?.strokes.length){remember();ed.strokes=[];repaint();changed();}return true;}
 if(type==='snapZoom'){zoom=zoom==='fit'?'actual':'fit';render();return true;}
 if(type==='snapBack'){if(ed?.dirty)await saveNow(ed);void action('snapClose');return true;}
 if(type==='snapLinkToggle'){if(ed)void action('snapLink',{id:ed.id,linked:!ed.note.linked});return true;}
 if(type==='snapDeleteNote'){
  if(armedDelete!==id){armedDelete=id;render();setTimeout(()=>{if(armedDelete===id){armedDelete='';render();}},4000);return true;}
  armedDelete='';if(ed?.id===id)ed.dirty=false;void action('snapDelete',id);return true;
 }
 return false;
});
