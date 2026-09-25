// Tab reordering. The grabbed tab follows the pointer along the tab axis and
// the others slide aside to show where it will land; releasing commits it.
// drop(id, before) receives the tab to insert before, or null for the end.
// over(id, x, y) reports whether the pointer is over another drop target (the
// pinned bookmarks); releasing there calls dropElsewhere(id) instead.
// While a drag is active the list must not be re-rendered (tabDragActive).
let drag=null,suppressClick=false;

export const tabDragActive=()=>!!drag?.active;

export function installTabDrag({horizontal,drop,cancel,over=()=>false,dropElsewhere=()=>{}}){
 // A release outside the shell (over a page view, which is a separate native
 // window, or outside the app) never reaches it. Without these checks the
 // drag would stay active and hold back every re-render of the sidebar.
 const abandon=()=>{if(drag)end(false);};
 window.addEventListener('blur',abandon);
 document.addEventListener('pointerdown',event=>{
  abandon();
  const el=event.target.closest('.tabs > .tab');
  if(!el||event.button!==0||event.target.closest('.tab-actions'))return;
  drag={el,id:el.dataset.tab,x:event.clientX,y:event.clientY,pointer:event.pointerId,active:false,horizontal:horizontal()};
 });
 document.addEventListener('pointermove',event=>{
  if(!drag||event.pointerId!==drag.pointer)return;
  if(event.pointerType==='mouse'&&event.buttons===0){abandon();return;}
  const dx=event.clientX-drag.x,dy=event.clientY-drag.y;
  if(!drag.active){if(Math.hypot(dx,dy)<5)return;start();}
  move(drag.horizontal?dx:dy);
  // Over another target the list keeps its order; the tab only follows.
  drag.elsewhere=over(drag.id,event.clientX,event.clientY);
  drag.el.classList.toggle('drop-elsewhere',drag.elsewhere);
  if(drag.elsewhere){drag.target=drag.index;drag.items.forEach(el=>{if(el!==drag.el)el.style.transform='';});}
 });
 document.addEventListener('pointerup',event=>{if(drag&&event.pointerId===drag.pointer)end(true);});
 document.addEventListener('pointercancel',()=>end(false));
 document.addEventListener('keydown',event=>{if(event.key==='Escape'&&drag?.active){event.preventDefault();end(false);}});
 // The click after a drag must not also activate or close a tab.
 document.addEventListener('click',event=>{if(suppressClick){event.stopImmediatePropagation();event.preventDefault();}},true);

 function start(){
  // Pinned and other tabs are reordered within their own group.
  const items=[...document.querySelectorAll('.tabs > .tab')].filter(el=>el.classList.contains('pinned')===drag.el.classList.contains('pinned'));
  const rects=items.map(el=>el.getBoundingClientRect());
  const begin=r=>drag.horizontal?r.left:r.top,size=r=>drag.horizontal?r.width:r.height;
  const index=items.indexOf(drag.el);
  const gap=items.length>1?begin(rects[1])-begin(rects[0])-size(rects[0]):0;
  Object.assign(drag,{active:true,items,index,target:index,step:size(rects[index])+gap,
   center:begin(rects[index])+size(rects[index])/2,centers:rects.map(r=>begin(r)+size(r)/2),
   min:begin(rects[0])-begin(rects[index]),max:begin(rects.at(-1))+size(rects.at(-1))-begin(rects[index])-size(rects[index])});
  drag.el.classList.add('dragging');document.body.classList.add('tab-dragging');
  try{drag.el.setPointerCapture(drag.pointer);}catch{}
  drag.el.addEventListener('lostpointercapture',abandon,{once:true});
 }
 function move(delta){
  const offset=Math.max(drag.min,Math.min(drag.max,delta));
  const translate=value=>value?'translate'+(drag.horizontal?'X':'Y')+'('+value+'px)':'';
  drag.el.style.transform=translate(offset);
  // The landing slot is where the dragged tab's center now falls.
  const center=drag.center+offset;
  // At the clamp the centers coincide; the edge slot must still be reachable.
  drag.target=drag.centers.filter((c,i)=>i>drag.index?center>=c:i<drag.index&&center>c).length;
  drag.items.forEach((el,i)=>{
   if(i===drag.index)return;
   el.style.transform=translate(i>drag.index&&i<=drag.target?-drag.step:i<drag.index&&i>=drag.target?drag.step:0);
  });
 }
 function end(commit){
  const current=drag;drag=null;
  if(!current?.active)return;
  suppressClick=true;setTimeout(()=>{suppressClick=false;},0);
  document.body.classList.remove('tab-dragging');
  current.el.classList.remove('dragging','drop-elsewhere');
  for(const el of current.items)el.style.transform='';
  const others=current.items.filter(el=>el!==current.el);
  // Past the end of the pinned group means before the first other tab.
  const after=current.items.at(-1).nextElementSibling;
  if(commit&&current.elsewhere)dropElsewhere(current.id);
  else if(commit&&current.target!==current.index)drop(current.id,others[current.target]?.dataset.tab??after?.dataset.tab??null);
  else cancel();
 }
}
