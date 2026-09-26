import test from 'node:test';
import assert from 'node:assert/strict';
import {mapTabID,trackerTabID,rememberFavicon,bookmarkGroup,defaults,restore,receiveTask,receiveMap,moveTab,togglePin,pinBookmark,bookmarkTab,goHome,openLocal,webURL,pageURL,resolveAddress,searchURL,shortcut,tabAt,cycleTab,receivePosition,translatedURL,originalURL,isTranslated} from './state.js';
import {encode,decode,iceServers} from './peer-code.js';
test('curated bookmarks merge once and preserve user choices',()=>{
 const existing={id:'custom-market',name:'My prices',url:'https://tarkov-market.com',group:'other'};
 const s=restore({bookmarks:[existing]});
 assert.equal(s.bookmarks.length,3);
 assert.deepEqual(s.bookmarks[0],{...existing,url:existing.url+'/'});
 assert.equal(s.bookmarks.some(b=>b.id==='maps'),false);
 s.bookmarks=s.bookmarks.filter(b=>b.id!=='eft-ammo');
 assert.deepEqual(restore(JSON.parse(JSON.stringify(s))).bookmarks,s.bookmarks);
 const fresh=defaults();fresh.bookmarks=[];
 assert.deepEqual(restore(fresh).bookmarks,[]);
 const full=Array.from({length:100},(_,i)=>({id:`custom-${i}`,name:`Custom ${i}`,url:`https://example.com/${i}`,group:'other'}));
 assert.deepEqual(restore({bookmarks:full}).bookmarks,full);
});
const task=id=>({id,name:id,site:'official-wiki',urls:{'tarkov-dev':`https://tarkov.dev/task/${id}`,'official-wiki':`https://escapefromtarkov.fandom.com/wiki/${id}`,'japanese-wiki':`https://wikiwiki.jp/eft/Prapor/${id}`}});
test('reuse preserves pinned tasks and deduplicates repeated detections',()=>{
 const s=defaults();s.taskMode='reuse';const first=receiveTask(s,task('Debut'));first.pinned=true;
 const second=receiveTask(s,task('Checking'));assert.notEqual(first.id,second.id);
 assert.equal(receiveTask(s,task('Checking')).id,second.id);
 assert.equal(receiveTask(s,task('Search')).id,second.id);
 assert.equal(first.task.id,'Debut');assert.equal(s.tabs.length,5);
});
test('new-tab mode and selected wiki survive restoration',()=>{
 const s=defaults();s.taskMode='new';s.questSite='japanese-wiki';
 const a=receiveTask(s,task('Debut'));const b=receiveTask(s,task('Checking'));
 assert.notEqual(a.id,b.id);assert.match(b.url,/wikiwiki.jp/);
 assert.deepEqual(restore(JSON.parse(JSON.stringify(s))).tabs.map(t=>t.id),s.tabs.map(t=>t.id));
});
test('settings can close and reopen without duplication',()=>{
 const s=defaults();s.tabs=s.tabs.filter(t=>t.kind!=='settings');openLocal(s,'settings');openLocal(s,'settings');
 assert.equal(s.tabs.filter(t=>t.kind==='settings').length,1);
});
test('invalid incoming sites, internal protocols and traversal IDs are rejected',()=>{
 for(const u of ['javascript:alert(1)','file:///etc/passwd','https://user:pass@example.com','http://wails.localhost/settings.html','wails://wails/'])assert.equal(webURL(u),null);
 const s=defaults();const bad=task('x');bad.urls['japanese-wiki']='file:///tmp/a';assert.equal(receiveTask(s,bad),null);
 assert.equal(receiveMap(s,'../settings'),null);
 assert.deepEqual(restore({tabs:[{id:'../../outside',kind:'web',url:'https://tarkov.dev'}]}).tabs.map(t=>t.id),[mapTabID,trackerTabID]);
});
test('every map lands in the fixed map tab, which always leads and survives restoration',()=>{
 const s=defaults();assert.equal(s.tabs[0].id,mapTabID);
 const a=receiveMap(s,'ground-zero-21');assert.equal(a.id,mapTabID);assert.equal(a.url,'https://tarkov.dev/map/ground-zero');
 assert.equal(receiveMap(s,'customs').id,mapTabID);
 assert.equal(s.tabs.filter(t=>t.role==='map').length,1);
 const restored=restore({tabs:[{id:'x',kind:'blank'},{id:mapTabID,kind:'web',url:'https://tarkov.dev/map/customs?connection=AB12'},{id:'old',kind:'web',role:'map',url:'https://tarkov.dev/map/woods'}]});
 assert.deepEqual(restored.tabs.map(t=>t.id),[mapTabID,trackerTabID,'x','old']);
 assert.equal(restored.tabs[0].url,'https://tarkov.dev/map/customs');
 assert.equal(restored.tabs[0].fixed,true);assert.equal(restored.tabs[3].role,undefined);
});
test('incoming traffic cannot grow tabs without a limit',()=>{
 const s=defaults();s.taskMode='new';for(let i=0;i<100;i++)receiveTask(s,task(String(i)));
 assert.equal(s.tabs.length,80);
});
test('pairing rejects expired, relayed, media and malformed descriptions without networking',()=>{
 const code={version:1,type:'offer',id:'01234567-89ab-4cde-8fab-0123456789ab',createdAt:Date.now(),sdp:'v=0\r\nm=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\na=fingerprint:sha-256 AA:BB\r\n'};
 assert.deepEqual(decode(encode(code)),code);
 assert.throws(()=>decode(encode(code),code.createdAt+600001));
 assert.throws(()=>encode({...code,sdp:code.sdp+'a=candidate:1 1 UDP 1 1.2.3.4 9 typ relay\r\n'}));
 assert.throws(()=>encode({...code,sdp:code.sdp+'m=audio 9 RTP/AVP 0\r\n'}));
 assert.throws(()=>decode('MAYAK1.invalid'));
 assert.throws(()=>iceServers('turn:example.com:3478'));
 assert.deepEqual(iceServers(''),[]);
});

test('legacy home tabs are removed without losing bookmarks or web tabs',()=>{
 const s=defaults();const web=receiveTask(s,task('Debut'));
 const restored=restore({...s,tabs:[{id:'home',kind:'home'},...s.tabs],active:'home'});
 assert.equal(restored.tabs.some(t=>t.kind==='home'),false);
 assert.ok(restored.tabs.some(t=>t.id===web.id));
 assert.deepEqual(restored.bookmarks,s.bookmarks);
 assert.deepEqual(restore({tabs:[],active:''}).tabs.map(t=>t.id),[mapTabID,trackerTabID]);
 assert.equal(restore({tabs:[],active:''}).active,mapTabID);
});

test('pageURL strips the injected tarkov.dev connection only', () => {
  assert.equal(pageURL('https://tarkov.dev/map/customs?connection=AB12'), 'https://tarkov.dev/map/customs');
  assert.equal(pageURL('https://tarkov.dev/map/customs?connection=AB12&x=1'), 'https://tarkov.dev/map/customs?x=1');
  assert.equal(pageURL('https://example.com/?connection=AB12'), 'https://example.com/?connection=AB12');
  assert.equal(pageURL('javascript:alert(1)'), null);
});

test('ad blocking defaults on and keeps an explicit off', () => {
  assert.equal(defaults().adblock, true);
  assert.equal(restore({}).adblock, true);
  assert.equal(restore({adblock:false}).adblock, false);
  assert.equal(restore(JSON.parse(JSON.stringify(restore({adblock:false})))).adblock, false);
});

test('recognized tasks open in a new tab by default, but a saved reuse choice is kept', () => {
  assert.equal(defaults().taskMode, 'new');
  assert.equal(restore({}).taskMode, 'new');
  assert.equal(restore({taskMode:'reuse'}).taskMode, 'reuse');
  const s=defaults();const a=receiveTask(s,task('Debut'));const b=receiveTask(s,task('Checking'));
  assert.notEqual(a.id,b.id);
});

test('theme defaults to MAYAK Dark, maps the earlier names and rejects unknown themes', () => {
  assert.equal(defaults().theme, 'mayak-dark');
  assert.equal(restore({theme:'catppuccin-mocha'}).theme, 'catppuccin-mocha');
  assert.equal(restore({theme:'system'}).theme, 'system');
  assert.equal(restore({theme:'neon'}).theme, 'mayak-dark');
  assert.equal(restore({theme:'claude-light'}).theme, 'mayak-light');
});

test('tabs reorder before a tab or to the end, never past the fixed map', () => {
  const s=defaults();s.tabs.push({id:'a',kind:'blank'},{id:'b',kind:'blank'});
  const ids=()=>s.tabs.map(t=>t.id);
  assert.equal(moveTab(s,'b','settings'),true);assert.deepEqual(ids(),[mapTabID,trackerTabID,'b','settings','a']);
  assert.equal(moveTab(s,'b',null),true);assert.deepEqual(ids(),[mapTabID,trackerTabID,'settings','a','b']);
  assert.equal(moveTab(s,'a',mapTabID),false);assert.equal(moveTab(s,trackerTabID,null),false);
  assert.equal(moveTab(s,'a','missing'),false);assert.deepEqual(ids(),[mapTabID,trackerTabID,'settings','a','b']);
});

test('pinned tabs gather at the top and reorder only among themselves', () => {
  const s=defaults();s.tabs=[s.tabs[0],s.tabs[1],...['a','b','c','d'].map(id=>({id,kind:'web',url:'https://example.com/'+id}))];
  const ids=()=>s.tabs.map(t=>t.id).join(',');
  togglePin(s,'c');togglePin(s,'d');assert.equal(ids(),mapTabID+','+trackerTabID+',c,d,a,b');
  assert.equal(moveTab(s,'d','c'),true);assert.equal(ids(),mapTabID+','+trackerTabID+',d,c,a,b');
  moveTab(s,'a','d');assert.equal(ids(),mapTabID+','+trackerTabID+',d,c,a,b');       // cannot jump into the pinned group
  moveTab(s,'d',null);assert.equal(ids(),mapTabID+','+trackerTabID+',c,d,a,b');       // stays at the end of its group
  togglePin(s,'c');assert.equal(ids(),mapTabID+','+trackerTabID+',d,c,a,b');          // unpinned: top of the others
  assert.equal(togglePin(s,'settings'),false);
  const restored=restore({tabs:[{id:'x',kind:'web',url:'https://example.com/x'},{id:'y',kind:'web',url:'https://example.com/y',pinned:true}]});
  assert.equal(restored.tabs.map(t=>t.id).join(','),mapTabID+','+trackerTabID+',y,x');
});

test('bookmarks pin to the sidebar at a drop position and survive restoration', () => {
  const s=defaults();s.bookmarks=['a','b','c','d'].map(id=>({id,name:id,url:'https://example.com/'+id,group:'other'}));
  const pinned=()=>s.bookmarks.filter(b=>b.sidebar).map(b=>b.id).join(',');
  pinBookmark(s,'c',true,null);pinBookmark(s,'a',true,null);assert.equal(pinned(),'c,a');
  pinBookmark(s,'d',true,'c');assert.equal(pinned(),'d,c,a');
  pinBookmark(s,'a',true,'d');assert.equal(pinned(),'a,d,c');       // reorder within the sidebar
  pinBookmark(s,'d',false);assert.equal(pinned(),'a,c');
  assert.equal(pinBookmark(s,'missing',true,null),false);
  assert.equal(restore(JSON.parse(JSON.stringify(s))).bookmarks.filter(b=>b.sidebar).map(b=>b.id).join(','),'a,c');
});
test('sidebar and bookmark view preferences restore with safe defaults', () => {
  assert.equal(restore({}).sidebarCollapsed,false);assert.equal(restore({sidebarCollapsed:true}).sidebarCollapsed,true);
  assert.equal(restore({}).bookmarkView,'grid');assert.equal(restore({bookmarkView:'list'}).bookmarkView,'list');
  const r=restore({tabs:[{id:'b1',kind:'bookmarks'},{id:'b2',kind:'bookmarks'}]});
  assert.deepEqual(r.tabs.map(t=>t.id),[mapTabID,trackerTabID,'b1']);
});

test('sidebar width restores within its limits', () => {
  assert.equal(restore({}).sidebarWidth,224);
  assert.equal(restore({sidebarWidth:300.4}).sidebarWidth,300);
  assert.equal(restore({sidebarWidth:20}).sidebarWidth,180);
  assert.equal(restore({sidebarWidth:9999}).sidebarWidth,420);
  assert.equal(restore({sidebarWidth:'wide'}).sidebarWidth,224);
});

test('favicons are kept per site with a limit and only as web URLs', () => {
  const s=defaults();
  rememberFavicon(s,'https://www.example.com/a','https://example.com/favicon.ico');
  rememberFavicon(s,'https://evil.example/','javascript:alert(1)');
  assert.deepEqual(s.favicons,{'example.com':'https://example.com/favicon.ico'});
  for(let i=0;i<250;i++)rememberFavicon(s,'https://s'+i+'.test/','https://s'+i+'.test/f.png');
  assert.equal(Object.keys(s.favicons).length,200);assert.equal(s.favicons['s249.test'],'https://s249.test/f.png');
  const r=restore({favicons:{...s.favicons,'bad host':'https://x/'},tabs:[{id:'t',kind:'web',url:'https://a.test/',favicon:'file:///x'}]});
  assert.equal(r.favicons["bad host"],undefined);assert.equal(r.tabs[2].favicon,undefined);
});

test('bookmark categories accept typed names', () => {
  assert.equal(bookmarkGroup('  Boss   routes '),'Boss routes');
  assert.equal(bookmarkGroup(''),'other');assert.equal(bookmarkGroup(undefined),'other');
  assert.equal(bookmarkGroup('x'.repeat(60)).length,40);
  const r=restore({bookmarkRevision:1,bookmarks:[{id:'a',name:'A',url:'https://a.test/',group:'弾薬'},{id:'b',name:'B',url:'https://b.test/',group:'maps'}]});
  assert.deepEqual(r.bookmarks.map(b=>b.group),['弾薬','maps']);
});

test('the bookmark section fold state is remembered', () => {
  assert.equal(restore({}).bookmarksCollapsed,false);
  assert.equal(restore({bookmarksCollapsed:true}).bookmarksCollapsed,true);
 assert.equal(restore({}).screenshotsCollapsed,false);
 assert.equal(restore({screenshotsCollapsed:true}).screenshotsCollapsed,true);
 assert.equal(restore({}).bossesView,'full');
 assert.equal(restore({bossesView:'goons'}).bossesView,'goons');
 assert.equal(restore({bossesView:'half'}).bossesView,'full');
 assert.equal(restore({}).clock,'24');
 assert.equal(restore({}).bossMap,'');
 assert.equal(restore({bossMap:'customs',bossMode:'pve'}).bossMap,'customs');
 assert.equal(restore({bossMap:'../x'}).bossMap,'');
 assert.equal(restore({bossMode:'pve'}).bossMode,'pve');
 assert.equal(restore({bossMode:'arena'}).bossMode,'');
 assert.equal(restore({clock:'12'}).clock,'12');
 assert.equal(restore({clock:'13'}).clock,'24');
 assert.equal(restore({bossesCollapsed:true}).bossesView,'closed');
 assert.equal(restore({tabs:[{id:'b1',kind:'bosses'},{id:'b2',kind:'bosses'}]}).tabs.filter(tab=>tab.kind==='bosses').length,1);
});

test('dropping a tab on the pinned bookmarks bookmarks and pins it once', () => {
  const s=defaults();s.bookmarks=[];s.tabs.push({id:'w',kind:'web',url:'https://a.test/',title:'A page'},{id:'b',kind:'blank'});
  const first=bookmarkTab(s,'w',null);
  assert.deepEqual([first.name,first.url,first.group,first.sidebar],['A page','https://a.test/','other',true]);
  assert.equal(bookmarkTab(s,'w',null).id,first.id);assert.equal(s.bookmarks.length,1);
  assert.equal(bookmarkTab(s,'b',null),null);assert.equal(bookmarkTab(s,'settings',null),null);
});

test('TarkovTracker is a fixed view after the map and keeps its page', () => {
  const s=defaults();assert.deepEqual(s.tabs.slice(0,2).map(t=>[t.id,t.fixed,t.url]),[[mapTabID,true,'https://tarkov.dev/maps/'],[trackerTabID,true,'https://tarkovtracker.org/']]);
  const r=restore({tabs:[{id:trackerTabID,kind:'web',url:'https://tarkovtracker.org/tasks'},{id:'x',kind:'blank'}]});
  assert.deepEqual(r.tabs.map(t=>t.id),[mapTabID,trackerTabID,'x']);assert.equal(r.tabs[1].url,'https://tarkovtracker.org/tasks');
  // The old .io site is replaced by the .org one the tracker API uses.
  assert.equal(restore({tabs:[{id:trackerTabID,kind:'web',url:'https://tarkovtracker.io/tasks'}]}).tabs[1].url,'https://tarkovtracker.org/');
  const old=restore({bookmarkRevision:1,bookmarks:[{id:'tracker',name:'TarkovTracker',url:'https://tarkovtracker.io/',group:'progress'}]});
  assert.deepEqual(old.bookmarks.map(b=>b.url),['https://tarkovtracker.org/']);
  assert.equal(receiveMap(s,'customs').id,mapTabID);
});

test('fixed views return home after links led elsewhere', () => {
  const s=defaults();const map=s.tabs[0],tracker=s.tabs[1];
  receiveMap(s,'woods');map.url='https://tarkov.dev/items';tracker.url='https://tarkov.dev/';
  assert.equal(goHome(s,mapTabID),true);assert.equal(map.url,'https://tarkov.dev/map/woods');
  assert.equal(goHome(s,trackerTabID),true);assert.equal(tracker.url,'https://tarkovtracker.org/');
  assert.equal(goHome(s,'settings'),false);
  assert.equal(restore(JSON.parse(JSON.stringify(s))).tabs[0].home,'https://tarkov.dev/map/woods');
});
test('the item sidebar is restored with its item',()=>{
 assert.deepEqual(restore({itemPanel:{open:true,id:'57347ca924597744596b4e71',mode:'pve'}}).itemPanel,{open:true,id:'57347ca924597744596b4e71',mode:'pve'});
 assert.deepEqual(restore({itemPanel:{open:'yes',id:'../x',mode:'pve'}}).itemPanel,{open:false,id:'',mode:''});
 assert.deepEqual(restore({}).itemPanel,{open:false,id:'',mode:''});
 assert.equal(restore({}).itemDock,'right');
 assert.equal(restore({itemDock:'bottom'}).itemDock,'bottom');
 assert.equal(restore({itemDock:'left'}).itemDock,'left');
 assert.equal(restore({itemDock:'top'}).itemDock,'right');
 assert.equal(restore({itemPanelHeight:9999}).itemPanelHeight,560);
 assert.equal(restore({}).itemPanelHeight,280);
 assert.equal(restore({}).sidebarSide,'left');
 assert.equal(restore({sidebarSide:'right'}).sidebarSide,'right');
 const right=restore({layout:'horizontal',navPosition:'right'});
 assert.equal(right.layout,'vertical');assert.equal(right.sidebarSide,'right');
 const top=restore({sidebarSide:'right',navPosition:'top'});
 assert.equal(top.layout,'horizontal');assert.equal(top.sidebarSide,'right');
});
test('the address bar opens addresses and searches everything else',()=>{
 assert.equal(resolveAddress(''),null);
 assert.equal(resolveAddress('   '),null);
 assert.equal(resolveAddress('https://tarkov.dev/maps/'),'https://tarkov.dev/maps/');
 assert.equal(resolveAddress(' HTTP://tarkov.dev '),'http://tarkov.dev/');
 assert.equal(resolveAddress('tarkov.dev/maps'),'https://tarkov.dev/maps');
 assert.equal(resolveAddress('escapefromtarkov.fandom.com/wiki/Quests?x=1#top'),'https://escapefromtarkov.fandom.com/wiki/Quests?x=1#top');
 assert.equal(resolveAddress('localhost:5173/page'),'https://localhost:5173/page');
 assert.equal(resolveAddress('192.168.1.10'),'https://192.168.1.10/');
 assert.equal(resolveAddress('devbox:8080'),'https://devbox:8080/');
 assert.equal(resolveAddress('flea market prices'),searchURL('flea market prices'));
 assert.equal(resolveAddress('customs'),searchURL('customs'));
 assert.equal(resolveAddress('タルコフ 攻略'),searchURL('タルコフ 攻略'));
 assert.equal(resolveAddress('3.14'),searchURL('3.14'));
 assert.equal(resolveAddress('v0.1.13 patch notes'),searchURL('v0.1.13 patch notes'));
 assert.equal(resolveAddress('? tarkov.dev'),searchURL('tarkov.dev'));
 assert.equal(resolveAddress('?'),null);
 // Privileged and non-web schemes never navigate.
 assert.equal(resolveAddress('https://user:pw@tarkov.dev/'),searchURL('https://user:pw@tarkov.dev/'));
 assert.equal(resolveAddress('http://wails.localhost/'),searchURL('http://wails.localhost/'));
 assert.equal(resolveAddress('javascript:alert(1)'),searchURL('javascript:alert(1)'));
 assert.equal(resolveAddress('file:///C:/x'),searchURL('file:///C:/x'));
});
test('keyboard shortcuts follow Chrome',()=>{
 const k=(key,mods={})=>shortcut({key,ctrl:false,shift:false,alt:false,...mods});
 assert.deepEqual(k('t',{ctrl:true}),{type:'newTab'});
 assert.deepEqual(k('T',{ctrl:true,shift:true}),{type:'reopenTab'});
 assert.deepEqual(k('w',{ctrl:true}),{type:'closeTab'});
 assert.deepEqual(k('F4',{ctrl:true}),{type:'closeTab'});
 assert.deepEqual(k('Tab',{ctrl:true}),{type:'nextTab'});
 assert.deepEqual(k('Tab',{ctrl:true,shift:true}),{type:'prevTab'});
 assert.deepEqual(k('PageDown',{ctrl:true}),{type:'nextTab'});
 assert.deepEqual(k('PageUp',{ctrl:true}),{type:'prevTab'});
 assert.deepEqual(k('l',{ctrl:true}),{type:'address'});
 assert.deepEqual(k('d',{alt:true}),{type:'address'});
 assert.deepEqual(k('F6'),{type:'address'});
 assert.deepEqual(k('d',{ctrl:true}),{type:'bookmark'});
 assert.deepEqual(k('1',{ctrl:true}),{type:'tabAt',index:1});
 assert.deepEqual(k('9',{ctrl:true}),{type:'tabAt',index:9});
 assert.equal(k('t'),null);
 assert.equal(k('t',{ctrl:true,alt:true}),null);
 assert.equal(k('w',{ctrl:true,shift:true}),null);
 assert.equal(k('0',{ctrl:true}),null);
 assert.equal(k('l',{ctrl:true,shift:true}),null);
});
test('tab switching by number and cycling covers every tab',()=>{
 const s=defaults();s.tabs=[...s.tabs.filter(t=>t.fixed),{id:'a',kind:'web',url:'https://a.example/'},{id:'b',kind:'web',url:'https://b.example/'},{id:'c',kind:'blank'}];
 s.active='a';
 assert.equal(tabAt(s,1).id,mapTabID);
 assert.equal(tabAt(s,2).id,trackerTabID);
 assert.equal(tabAt(s,3).id,'a');
 assert.equal(tabAt(s,9).id,'c');
 assert.equal(tabAt(s,6),null);
 assert.equal(s.active,'c');
 assert.equal(cycleTab(s,1).id,mapTabID);
 assert.equal(cycleTab(s,-1).id,'c');
 assert.equal(cycleTab(s,-1).id,'b');
 s.tabs=[];assert.equal(cycleTab(s,1),null);assert.equal(tabAt(s,1),null);
});
test('a detected position brings the map view forward without reloading its map',()=>{
 const s=defaults();
 receiveMap(s,'customs');s.tabs.find(t=>t.id===mapTabID).url='https://tarkov.dev/map/customs?layer=2';
 openLocal(s,'settings');
 assert.equal(receivePosition(s,'customs').id,mapTabID);
 assert.equal(s.active,mapTabID);
 assert.equal(s.tabs.find(t=>t.id===mapTabID).url,'https://tarkov.dev/map/customs?layer=2');
 receivePosition(s,'woods');
 assert.equal(s.tabs.find(t=>t.id===mapTabID).url,'https://tarkov.dev/map/woods');
 assert.equal(receivePosition(s,'ground-zero-21').url,'https://tarkov.dev/map/ground-zero');
 assert.equal(receivePosition(s,'Bad Map'),null);
});
test('pages translate through Google Translate\'s proxy and back',()=>{
 const wiki='https://escapefromtarkov.fandom.com/wiki/Quests?x=1#top';
 const translated=translatedURL(wiki,'ja');
 assert.equal(translated,'https://escapefromtarkov-fandom-com.translate.goog/wiki/Quests?x=1&_x_tr_sl=auto&_x_tr_tl=ja&_x_tr_hl=ja#top');
 assert.equal(isTranslated(translated),true);
 assert.equal(isTranslated(wiki),false);
 assert.equal(originalURL(translated),wiki);
 // Hyphens in the host are doubled and restored.
 assert.equal(translatedURL('https://tarkov-market.com/item/x','en'),'https://tarkov--market-com.translate.goog/item/x?_x_tr_sl=auto&_x_tr_tl=en&_x_tr_hl=en');
 assert.equal(originalURL('https://tarkov--market-com.translate.goog/item/x?_x_tr_sl=auto&_x_tr_tl=en'),'https://tarkov-market.com/item/x');
 // Already translated stays; the original stays; junk is null.
 assert.equal(translatedURL(translated,'ja'),translated);
 assert.equal(originalURL(wiki),wiki);
 assert.equal(translatedURL('not a url','ja'),null);
});
test('a detected task opens the official wiki translated when asked',()=>{
 const s=defaults();s.questSite='official-wiki';s.translateWiki=true;
 const tab=receiveTask(s,task('Debut'));
 assert.equal(tab.url,'https://escapefromtarkov-fandom-com.translate.goog/wiki/Debut?_x_tr_sl=auto&_x_tr_tl=ja&_x_tr_hl=ja');
 s.translateWiki=false;
 assert.equal(receiveTask(s,task('Debut')).url,'https://escapefromtarkov.fandom.com/wiki/Debut');
 s.translateWiki=true;s.questSite='japanese-wiki';
 assert.equal(receiveTask(s,task('Debut')).url,'https://wikiwiki.jp/eft/Prapor/Debut');
 assert.equal(restore({translateWiki:true}).translateWiki,true);
 assert.equal(restore({}).translateWiki,false);
});
