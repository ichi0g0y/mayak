import './api.js';
import './style.css';
import {themes,sidebarWidths,clampSidebar,hostname,bookmarkGroups,webURL,browserSections,hostSections} from './state.js';
import {Window,Browser} from '@wailsio/runtime';
import {installTabDrag,tabDragActive} from './tab-drag.js';
import {bestSale,price,age,chartSeries,chartPath,itemPanelWidths,clampItemPanel,itemPanelHeights,clampItemPanelHeight,itemPageURL} from './item.js';
const api=window.mayak;
let state,editingBookmark=null,bookmarkQuery='',contextMenu=null,placeMenu=null;
const peerDrafts={offer:'',answer:'',pairCode:''};
let copied=false,windowTheme='',renderDeferred=false;
// The first-run tutorial: an overlay that walks through the setup after
// installing. It shows on the Host until it is finished or skipped
// (state.tutorialDone) and can be reopened from the appearance settings.
let tutorialOpen=false,tutorialStep=0,tutorialSettings=null,tutorialStarted=false;
// The version whose update toast was put off with "later"; it returns for the next one.
let updateDismissed='';
// The build's version, for the About section; empty in development. The
// desktop bridge (api.js) exists once the first state arrives, so it loads then.
let appVersion='';
async function loadVersion(){try{appVersion=(await window.mayakDesktop?.backend?.GetVersion?.())||'';}catch{}if(appVersion)render();}
const words={
  ja:{home:'ホーム',settings:'設定',menu:'メニュー',off:'受信OFF',back:'戻る',forward:'進む',reload:'再読み込み',address:'URLを入力',open:'開く',close:'閉じる',pin:'固定',unpin:'固定解除',newTab:'新しいタブ',welcome:'レイドの準備を、ここから。',intro:'マップ、タスク、調べものを一つのウインドウに。',maps:'マップ',tasks:'タスク攻略',items:'アイテム',progress:'進捗管理',other:'その他',addBookmark:'ブックマークを追加',edit:'編集',delete:'削除',name:'名前',url:'URL',group:'分類',done:'確定',cancel:'キャンセル',empty:'ブックマークを追加して、よく使うサイトをまとめましょう。',client:'ブラウザ設定',host:'Host設定・ログ',appearance:'表示',language:'表示言語',layout:'タブの配置',vertical:'左サイドバー',horizontal:'上部の横並び',recognition:'タスクの自動表示',taskMode:'検出したタスクを開く方法',reuse:'同じタブを更新',new:'新しいタブを追加',questSite:'表示するサイト',hostChoice:'Hostの設定に従う',official:'公式Wiki',japanese:'日本語Wiki',taskHelp:'表示中のタスクは、上のサイト選択で英語Wiki・日本語Wikiを切り替えられます。固定したタブは自動更新の対象外です。',connection:'Host接続',mode:'接続方法',local:'このPCで検出する',disabled:'接続しない',localHelp:'このPCの検出結果をタブに表示します。',bookmarkHelp:'カードをクリックして開く。名前と分類は編集できます。',tabHelp:'ドラッグで順序変更',dismiss:'閉じる',searchWiki:'Wiki内で検索',pinHelp:'固定中'},
  en:{home:'Home',settings:'Settings',menu:'Menu',off:'Receiving off',back:'Back',forward:'Forward',reload:'Reload',address:'Enter a URL',open:'Open',close:'Close',pin:'Pin',unpin:'Unpin',newTab:'New tab',welcome:'Get ready for your next raid.',intro:'Maps, tasks and research, together in one window.',maps:'Maps',tasks:'Task guides',items:'Items',progress:'Progress',other:'Other',addBookmark:'Add bookmark',edit:'Edit',delete:'Delete',name:'Name',url:'URL',group:'Category',done:'Done',cancel:'Cancel',empty:'Add bookmarks to keep your useful sites together.',client:'Browser settings',host:'Host settings & logs',appearance:'Appearance',language:'Language',layout:'Tab placement',vertical:'Left sidebar',horizontal:'Across the top',recognition:'Automatic task navigation',taskMode:'Open recognized tasks',reuse:'Reuse the task tab',new:'Add a new tab',questSite:'Website',hostChoice:'Follow Host setting',official:'Official Wiki',japanese:'Japanese Wiki',taskHelp:'Switch the current task between English and Japanese wikis using the website selector above. Pinned tabs are never replaced automatically.',connection:'Host connection',mode:'Connection mode',local:'Detect on this computer',disabled:'No connection',localHelp:'Show detections from this computer in tabs.',bookmarkHelp:'Click a card to open it. Names and categories are editable.',tabHelp:'Drag to reorder',dismiss:'Dismiss',searchWiki:'Search this Wiki',pinHelp:'Pinned'},
};
Object.assign(words.ja,{bossMapBosses:'マップのボス',goonReport:'Goons を報告',goonReportOnly:'実際に Goons に遭遇したレイドだけ報告してください。報告は tarkov.dev に表示される目撃情報に加わります。',goonAccount:'報告するアカウント（EFT のログから）',goonNoAccount:'EFT のログにアカウントが見つかりません。一度ゲームを起動してください。',goonWhen:'いつ見たか',goonRaidNow:'今のレイド',goonRaidLast:'直前のレイド',goonStarted:'開始',goonAlready:'報告済み',goonWhenNow:'今の時刻で報告',goonConsent:'tarkov.dev は、報告と一緒にあなたの IP アドレスと選んだ EFT アカウント ID を保存します（不正な報告を追跡するため）。',goonSend:'報告する',goonSending:'送信中…',goonReported:'報告しました。tarkov.dev の目撃情報に反映されるまで最大10分ほどかかります。',goonCurrent:'プレイ中',goonLastSeen:'最終',bossAuto:'自動',bossModeLabel:'ゲームモード',bossPickHint:'マップを選ぶか、レイドに入るとボスを表示します',clockFormat:'時刻の表示',clock24:'24時間表示（13:05）',clock12:'12時間表示（1:05 PM）',shotKind_tasks:'タスク',shotKind_item:'アイテム',shotKind_offer:'出品',shotKind_position:'位置',shotKind_unknown:'判定なし',shotInfo:'判定の詳細',shotInfoKind:'種類',shotInfoMatch:'一致',shotInfoConfidence:'一致度',shotInfoCandidates:'候補',shotInfoLayout:'画面の形',shotInfoScore:'検出スコア',shotInfoMap:'マップ',shotInfoRaid:'レイド中',shotInfoPosition:'座標',shotInfoStage:'状態',shotInfoError:'エラー',yes:'はい',no:'いいえ',bosses:'ボス',allBosses:'ボスの詳細',bossesLoading:'ボス情報を読み込み中…',noGoons:'最近の報告なし',noBosses:'ボスは出ません',escorts:'取り巻き',goonHint:'Goons の位置はプレイヤーの報告（tarkov.dev）です。報告が新しいほど、今もそのマップにいる可能性が高くなります。',goonMaps:'Goons が出るマップ',bossMaps:'マップ',zoomIn:'拡大',zoomOut:'縮小',zoomFit:'全体を表示（リセット）',screenshots:'スクリーンショット',allScreenshots:'すべてのスクリーンショット',noScreenshots:'スクリーンショットはまだありません',screenshotsHostOnly:'スクリーンショットはHostのPCで見られます',openScreenshotFolder:'フォルダを開く',newerScreenshot:'新しい方へ',olderScreenshot:'古い方へ',
  webrtc:'インターネット経由（WebRTC・直接接続）',p2pTitle:'別のPCへ直接送信',p2pReceive:'接続コードで接続',p2pHelp:'両方の PC で MAYAK を開き、Host に出る 8 桁の接続コードを相手側に入力します。コードの受け渡しだけ mayak.ich.sh を経由し（10 分で失効）、接続後のタスク・マップ表示指示は暗号化して直接送ります。',pairCodeHelp:'相手の PC の MAYAK で 設定 → 他のPCとの接続 を開き、この番号を入力してください。応答は自動で受け取ります。10 分で失効します。',enterPairCode:'Host に表示された 8 桁の接続コード',joinPair:'接続する',manualExchange:'コードを手動で交換する（サーバーを使わない）',manualExchangeHelp:'mayak.ich.sh を経由したくないときは、長い招待コードと応答コードを自分でコピーして渡します。',relayFailed:'接続サーバー（mayak.ich.sh）に届きませんでした。下の手動交換を使ってください。',p2pWaitingHostAuto:'Host の応答を待っています（自動）',p2pLimit:'STUNは接続先の探索だけに使用します。中継サーバーは使用しないため、回線によっては接続できません。アプリの再起動や接続切れの際はコードを再交換してください。',createInvite:'接続コードを発行',inviteCode:'招待コード',answerCode:'応答コード',copyCode:'コードをコピー',copied:'コピーしました',enterOffer:'Host側の招待コードを貼り付け',createAnswer:'応答コードを作成',enterAnswer:'受信側の応答コードを貼り付け',finishPairing:'接続する',disconnectPeer:'切断 / キャンセル',offerStep:'① このコードを受信側へ渡してください（有効期限10分）。',answerStep:'② このコードをHost側へ返してください。',stunLabel:'STUNサーバー',stunHelp:'空欄にすると探索サーバーも使いません。直接到達できるネットワーク向けです。',p2pIdle:'未接続',p2pGathering:'接続情報を準備しています…',p2pWaitingAnswer:'受信側の応答待ち',p2pWaitingHost:'Host側で応答コードを入力してください',p2pConnecting:'直接接続を試みています…',p2pConnected:'直接接続中（中継なし）',p2pFailed:'直接接続できませんでした。コードを作り直すか、別の回線をお試しください。',p2pDisconnected:'接続が切れました。コードを再交換してください。',p2pExpired:'招待コードの有効期限が切れました。',
});
Object.assign(words.en,{bossMapBosses:"Map's bosses",goonReport:'Report the Goons',goonReportOnly:'Only report a raid where you actually ran into the Goons. The report is added to the sightings shown on tarkov.dev.',goonAccount:'Account to report with (from the EFT logs)',goonNoAccount:'No account in the EFT logs yet. Start the game once.',goonWhen:'When you saw them',goonRaidNow:'This raid',goonRaidLast:'The last raid',goonStarted:'start',goonAlready:'reported',goonWhenNow:'Report at the current time',goonConsent:'tarkov.dev stores the report together with your IP address and the EFT account ID you chose, so that misuse can be traced.',goonSend:'Report',goonSending:'Sending…',goonReported:'Reported. It shows in the tarkov.dev sightings within about ten minutes.',goonCurrent:'playing',goonLastSeen:'last seen',bossAuto:'Auto',bossModeLabel:'Game mode',bossPickHint:'Choose a map, or enter a raid, to see its bosses',clockFormat:'Time format',clock24:'24-hour (13:05)',clock12:'12-hour (1:05 PM)',shotKind_tasks:'Task',shotKind_item:'Item',shotKind_offer:'Offer',shotKind_position:'Position',shotKind_unknown:'Not recognized',shotInfo:'Recognition details',shotInfoKind:'Kind',shotInfoMatch:'Match',shotInfoConfidence:'Match confidence',shotInfoCandidates:'Candidates',shotInfoLayout:'Layout',shotInfoScore:'Detection score',shotInfoMap:'Map',shotInfoRaid:'In raid',shotInfoPosition:'Position',shotInfoStage:'Stage',shotInfoError:'Error',yes:'Yes',no:'No',bosses:'Bosses',allBosses:'Boss details',bossesLoading:'Loading bosses…',noGoons:'no recent reports',noBosses:'No bosses',escorts:'Escorts',goonHint:'Goons locations are player reports (tarkov.dev). The newer a report, the likelier they are still on that map.',goonMaps:'Maps the Goons spawn on',bossMaps:'Maps',zoomIn:'Zoom in',zoomOut:'Zoom out',zoomFit:'Fit to page (reset)',screenshots:'Screenshots',allScreenshots:'All screenshots',noScreenshots:'No screenshots yet',screenshotsHostOnly:'Screenshots are shown on the Host computer',openScreenshotFolder:'Open folder',newerScreenshot:'Newer',olderScreenshot:'Older',
  webrtc:'Over the internet (WebRTC direct)',p2pTitle:'Send directly to another computer',p2pReceive:'Connect with a code',p2pHelp:'Open MAYAK on both PCs and enter the 8-digit code shown on the Host into the other one. Only the code goes through mayak.ich.sh (it expires in 10 minutes); once connected, task and map commands are sent directly, encrypted.',pairCodeHelp:'On the other PC, open MAYAK → Settings → Other computers and enter this number. The response is picked up automatically. It expires in 10 minutes.',enterPairCode:'The 8-digit code shown on the Host',joinPair:'Connect',manualExchange:'Exchange the codes by hand (no server)',manualExchangeHelp:'To keep mayak.ich.sh out of it, copy the long invitation and response codes across yourself.',relayFailed:'The pairing server (mayak.ich.sh) could not be reached; use the manual exchange below.',p2pWaitingHostAuto:'Waiting for the Host (automatic)',p2pLimit:'STUN only discovers connection addresses. No relay server is used, so some networks cannot connect. Exchange fresh codes after restarting the app or losing the connection.',createInvite:'Get a code',inviteCode:'Invitation code',answerCode:'Response code',copyCode:'Copy code',copied:'Copied',enterOffer:'Paste the invitation from the Host',createAnswer:'Create response',enterAnswer:'Paste the response from the receiver',finishPairing:'Connect',disconnectPeer:'Disconnect / cancel',offerStep:'1. Give this code to the receiver (expires in 10 minutes).',answerStep:'2. Return this code to the Host.',stunLabel:'STUN server',stunHelp:'Leave empty to use no discovery server. Requires directly reachable network addresses.',p2pIdle:'Not connected',p2pGathering:'Preparing connection details…',p2pWaitingAnswer:'Waiting for the receiver’s response',p2pWaitingHost:'Enter the response on the Host',p2pConnecting:'Trying a direct connection…',p2pConnected:'Direct connection (no relay)',p2pFailed:'A direct connection could not be established. Try fresh codes or another network.',p2pDisconnected:'Connection lost. Exchange fresh codes to reconnect.',p2pExpired:'The invitation has expired.',
});
Object.assign(words.ja,{tutorialShow:'チュートリアルを表示',tutorialShowHelp:'初回起動時の案内（フォルダ、スクリーンショットキー、マップ、TarkovTracker）をもう一度見ます。',tutStep:'ステップ',tutStart:'はじめる',tutLater:'あとで',tutNext:'次へ',tutBack:'戻る',tutSkip:'スキップ',tutFinish:'完了',
 tutWelcomeTitle:'MAYAK へようこそ',tutWelcome:'ゲームが保存するスクリーンショットとログを読むだけで、マップ・タスク・アイテム情報をこのウインドウに映します。ゲーム本体には触れません。準備は 5 つの手順で終わります。',
 tutFoldersTitle:'EFT のフォルダ',tutFolders:'スクリーンショットとログのフォルダを自動で探しました。違っていれば設定で選び直せます。',tutFoldersScreens:'スクリーンショット',tutFoldersLogs:'ログ',tutNotFound:'未検出。設定で選んでください',tutOpenFolders:'フォルダの設定を開く',
 tutKeyTitle:'スクリーンショットキーを押しやすい場所に',tutKey:'EFT の Settings → Controls → Screenshot。既定は PrintScreen ですが戦闘中に届きにくいので、マウスのサイドボタンや左手で届く空きキーへの割り当てをおすすめします。',tutKeyNote:'座標が入るのは EFT 自身のスクリーンショット機能で保存したファイルだけです。Steam や Windows のスクリーンショットは対象外です。',
 tutMapTitle:'レイド中はそのキーを押すだけ',tutMap:'このウインドウのマップタブは、設定なしで現在地に追従します。タスク画面やアイテム詳細を撮れば、それぞれの情報も表示されます。1 台の PC でマルチモニタなら、これで完結です。',tutOpenMap:'マップを開く',
 tutRemoteTitle:'別の PC や普段のブラウザで見るなら（任意）',tutRemote:'そのブラウザで tarkov.dev のマップを開き、左下の接続ボタンを押します。同じ PC の Chrome / Edge / Brave なら Remote ID は自動検出、別の PC やタブレットなら ID を入力します。',tutOpenRemote:'tarkov.dev 連携の設定を開く',
 tutTrackerTitle:'TarkovTracker で進捗を自動記録（任意）',tutTracker:'TarkovTracker はタスクの進捗を管理できる無料のサイトです。Settings → API Tokens で GP と WP を付けたトークンを作って取り込むと、受注・完了・失敗が自動で反映されます。',tutOpenTracker:'TarkovTracker の設定を開く',
 tutDoneTitle:'準備完了',tutDone:'この案内は 設定 → 表示 からいつでも見直せます。良いレイドを。'});
Object.assign(words.en,{tutorialShow:'Show the tutorial',tutorialShowHelp:'See the first-run guide again: folders, the screenshot key, the map and TarkovTracker.',tutStep:'Step',tutStart:'Get started',tutLater:'Later',tutNext:'Next',tutBack:'Back',tutSkip:'Skip',tutFinish:'Done',
 tutWelcomeTitle:'Welcome to MAYAK',tutWelcome:'MAYAK reads only the screenshots and logs the game saves, and shows maps, tasks and item info in this window. It never touches the game itself. Setup takes five steps.',
 tutFoldersTitle:'EFT folders',tutFolders:'The Screenshots and Logs folders were looked up automatically. If they are wrong, pick them in the settings.',tutFoldersScreens:'Screenshots',tutFoldersLogs:'Logs',tutNotFound:'Not found; pick it in the settings',tutOpenFolders:'Open the folder settings',
 tutKeyTitle:'Put the screenshot key within reach',tutKey:'In EFT: Settings → Controls → Screenshot. The default is PrintScreen, which is hard to reach mid-fight, so bind it to a mouse side button or a free key near your left hand.',tutKeyNote:'Only screenshots taken with EFT’s own function carry coordinates; Steam or Windows screenshots do not.',
 tutMapTitle:'In a raid, just press that key',tutMap:'The map tab in this window follows your position with no setup. Screenshot the Tasks screen or an item window and that info appears too. On one PC with several monitors, that is all you need.',tutOpenMap:'Open the map',
 tutRemoteTitle:'Another PC or your usual browser (optional)',tutRemote:'Open the tarkov.dev map there and click the connect button at the bottom left. Chrome, Edge and Brave on this PC are detected automatically; from another PC or a tablet, type the Remote ID in.',tutOpenRemote:'Open the tarkov.dev settings',
 tutTrackerTitle:'Let TarkovTracker record your progress (optional)',tutTracker:'TarkovTracker is a free site that tracks task progress. Create a token with GP and WP under Settings → API Tokens, add it here, and accepted, completed and failed tasks are recorded automatically.',tutOpenTracker:'Open the TarkovTracker settings',
 tutDoneTitle:'All set',tutDone:'You can see this guide again under Settings → Appearance. Good raids.'});
Object.assign(words.ja,{updateAvailable:'MAYAK {v} が見つかりました',updateDownloading:'MAYAK {v} をダウンロード中',updateReady:'MAYAK {v} の準備ができました',updateRestart:'再起動して適用',updateDownload:'ダウンロード',updateLater:'あとで',updateNotes:'変更点',updateReadyHint:'いま適用しなくても、次に MAYAK を終了したときに入れ替わります。'});
Object.assign(words.en,{updateAvailable:'MAYAK {v} is available',updateDownloading:'Downloading MAYAK {v}',updateReady:'MAYAK {v} is ready',updateRestart:'Restart to apply',updateDownload:'Download',updateLater:'Later',updateNotes:'What changed',updateReadyHint:'If you do not apply it now, it is installed when MAYAK next quits.'});
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
const t=key=>words[state?.language||'ja'][key]||key;
const esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
// Lucide icon paths (ISC), inlined so the shell stays dependency-free.
const icons={
 flag:'<path d="M4 22V4a1 1 0 0 1 .4-.8A6 6 0 0 1 8 2c3 0 5 2 7.333 2q2 0 3.067-.8A1 1 0 0 1 20 4v10a1 1 0 0 1-.4.8A6 6 0 0 1 16 16c-3 0-5-2-8-2a6 6 0 0 0-4 1.528"/>',
 info:'<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>',
 skull:'<path d="m12.5 17-.5-1-.5 1h1z"/><path d="M15 22a1 1 0 0 0 1-1v-1a2 2 0 0 0 1.56-3.25 8 8 0 1 0-11.12 0A2 2 0 0 0 8 20v1a1 1 0 0 0 1 1z"/><circle cx="15" cy="12" r="1"/><circle cx="9" cy="12" r="1"/>',
 placeLeft:'<rect width="18" height="18" x="3" y="3" rx="2"/><rect x="3" y="3" width="7" height="18" rx="2" fill="currentColor" stroke="none" opacity=".55"/>',placeRight:'<rect width="18" height="18" x="3" y="3" rx="2"/><rect x="14" y="3" width="7" height="18" rx="2" fill="currentColor" stroke="none" opacity=".55"/>',placeTop:'<rect width="18" height="18" x="3" y="3" rx="2"/><rect x="3" y="3" width="18" height="7" rx="2" fill="currentColor" stroke="none" opacity=".55"/>',placeBottom:'<rect width="18" height="18" x="3" y="3" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2" fill="currentColor" stroke="none" opacity=".55"/>',
 minus:'<path d="M5 12h14"/>',fit:'<path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/>',
 image:'<rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/>',
 back:'<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>',forward:'<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>',
 reload:'<path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/>',
 plus:'<path d="M5 12h14"/><path d="M12 5v14"/>',x:'<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
 settings:'<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>',
 pin:'<path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/>',
 map:'<path d="M14.1 4.1 9 2 3 4v18l6-2 6 2 6-2V2z"/><path d="M9 2v18"/><path d="M15 4v18"/>',
 task:'<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 13h4"/><path d="M10 17h4"/>',
 globe:'<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/>',
 search:'<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',chevron:'<path d="m6 9 6 6 6-6"/>',chevronRight:'<path d="m9 18 6-6-6-6"/>',
 host:'<rect width="20" height="14" x="2" y="3" rx="2"/><path d="M8 21h8"/><path d="M12 17v4"/>',off:'<circle cx="12" cy="12" r="10"/><path d="m4.9 4.9 14.2 14.2"/>',
 panelLeft:'<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 3v18"/>',panelRight:'<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M15 3v18"/>',panelTop:'<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 9h18"/>',panelBottom:'<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 15h18"/>',
 minimise:'<path d="M5 12h14"/>',maximise:'<rect x="5" y="5" width="14" height="14" rx="1.5"/>',restore:'<rect x="5" y="8" width="11" height="11" rx="1.5"/><path d="M8 8V6.5A1.5 1.5 0 0 1 9.5 5h8A1.5 1.5 0 0 1 19 6.5v8a1.5 1.5 0 0 1-1.5 1.5H16"/>',
 pinOff:'<path d="M12 17v5"/><path d="M15 9.34V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H7.89"/><path d="m2 2 20 20"/><path d="M9 9v1.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h11"/>',
 bookmark:'<path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>',
 list:'<path d="M3 12h.01"/><path d="M3 18h.01"/><path d="M3 6h.01"/><path d="M8 12h13"/><path d="M8 18h13"/><path d="M8 6h13"/>',
 grid:'<rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/>',
 edit:'<path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/>',
 trash:'<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/>',
 tracker:'<path d="m9 11 3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/>',
 external:'<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h3"/>',
 home:'<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .709-1.528l7-5.999a2 2 0 0 1 2.582 0l7 5.999A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
 linked:'<path d="M9 17H7A5 5 0 0 1 7 7h2"/><path d="M15 7h2a5 5 0 1 1 0 10h-2"/><path d="M8 12h8"/>',unlinked:'<path d="M9 17H7A5 5 0 0 1 7 7"/><path d="M15 7h2a5 5 0 0 1 4 8"/><path d="m2 2 20 20"/>',
 package:'<path d="M11 21.73a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73z"/><path d="M12 22V12"/><path d="m3.3 7 7.703 4.734a2 2 0 0 0 1.994 0L20.7 7"/><path d="m7.5 4.27 9 5.15"/>',check:'<path d="M20 6 9 17l-5-5"/>',
 palette:'<circle cx="13.5" cy="6.5" r=".5" fill="currentColor"/><circle cx="17.5" cy="10.5" r=".5" fill="currentColor"/><circle cx="8.5" cy="7.5" r=".5" fill="currentColor"/><circle cx="6.5" cy="12.5" r=".5" fill="currentColor"/><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.926 0 1.648-.746 1.648-1.688 0-.437-.18-.835-.437-1.125-.29-.289-.438-.652-.438-1.125a1.64 1.64 0 0 1 1.668-1.668h1.996c3.051 0 5.555-2.503 5.555-5.554C21.965 6.012 17.461 2 12 2z"/>',
 shield:'<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/>',
 activity:'<path d="M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2"/>',
 folder:'<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>',
 scan:'<path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><path d="M7 12h10"/>',
 volume:'<path d="M11 4.702a.705.705 0 0 0-1.203-.498L6.413 7.587A1.4 1.4 0 0 1 5.416 8H3a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2.416a1.4 1.4 0 0 1 .997.413l3.383 3.384A.705.705 0 0 0 11 19.298z"/><path d="M16 9a5 5 0 0 1 0 6"/><path d="M19.364 18.364a9 9 0 0 0 0-12.728"/>',
 power:'<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.77.04"/>',
bug:'<path d="m8 2 1.88 1.88"/><path d="M14.12 3.88 16 2"/><path d="M9 7.13v-1a3.003 3.003 0 1 1 6 0v1"/><path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6"/><path d="M12 20v-9"/><path d="M6.53 9C4.6 8.8 3 7.1 3 5"/><path d="M6 13H2"/><path d="M3 21c0-2.1 1.7-3.9 3.8-4"/><path d="M20.97 5c0 2.1-1.6 3.8-3.5 4"/><path d="M22 13h-4"/><path d="M17.2 17c2.1.1 3.8 1.9 3.8 4"/>',
};
const icon=(name,cls='icon')=>`<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name]}</svg>`;
const btn=(action,label,extra='')=>`<button data-action="${action}" title="${esc(label)}" aria-label="${esc(label)}" ${extra}>${esc(label)}</button>`;
const option=(value,label,current)=>`<option value="${esc(value)}" ${current===value?'selected':''}>${esc(label)}</option>`;
function select(key,label,choices,value,scope='preferences'){return `<label class="field"><span>${esc(label)}</span><select data-scope="${scope}" data-key="${key}">${choices.map(([v,l])=>option(v,l,value)).join('')}</select></label>`;}
const siteChoices=()=>[['tarkov-dev','tarkov.dev'],['official-wiki',t('official')],['japanese-wiki',t('japanese')]];
const tabName=tab=>tab.role==='map'&&tab.fixed?t('mapTab'):tab.role==='tracker'?'TarkovTracker':tab.kind==='blank'?t('newTab'):tab.kind==='settings'?t('settings'):tab.kind==='bosses'?t('bosses'):tab.kind==='screenshots'?t('screenshots'):tab.kind==='bookmarks'?t('bookmarks'):tab.title||tab.url;
// Tabs listed in the tab section (the strip sizes itself by their number).
const listedTabs=()=>state.tabs.filter(tab=>!tab.fixed&&tab.kind!=='bookmarks'&&tab.kind!=='settings'&&tab.kind!=='screenshots'&&tab.kind!=='bosses');
const tabCount=()=>listedTabs().length;
function tabs(){return listedTabs().map(tab=>`<div class="tab ${state.active===tab.id?'active':''} ${tab.pinned?'pinned':''}" data-tab="${esc(tab.id)}" role="tab" aria-selected="${state.active===tab.id}"><button data-action="activate" data-id="${esc(tab.id)}" class="tab-select" title="${esc(tabName(tab))}">${tabIcon(tab)?favicon(tabIcon(tab)):icon(tab.kind==='settings'?'settings':tab.kind==='bookmarks'?'bookmark':tab.kind==='blank'?'plus':tab.role==='map'?'map':tab.task?'task':'globe','tab-icon')}<span class="tab-name">${esc(tabName(tab))}</span></button><span class="tab-actions">${tab.kind==='web'?`<button class="tab-pin" data-action="pin" data-id="${esc(tab.id)}" title="${esc(t(tab.pinned?'unpin':'pin'))}" aria-label="${esc(t(tab.pinned?'unpin':'pin'))}" aria-pressed="${!!tab.pinned}">${icon('pin','icon pin-on')}${icon('pinOff','icon pin-off')}</button>`:''}${tab.pinned?'':`<button class="tab-close" data-action="close" data-id="${esc(tab.id)}" title="${esc(t('close'))}" aria-label="${esc(t('close'))}">${icon('x')}</button>`}</span></div>`).join('');}
// Fixed views (TARKOV.DEV, TarkovTracker) sit above the sections like nav items.
function mapEntry(){return state.tabs.filter(tab=>tab.fixed).map(tab=>`<div class="tab map-entry ${state.active===tab.id?'active':''}"><button data-action="activate" data-id="${esc(tab.id)}" class="tab-select" title="${esc(tab.role==='map'?t('mapTabHelp'):tabName(tab))}">${tabIcon(tab)?favicon(tabIcon(tab)):icon(tab.role==='map'?'map':'tracker','tab-icon')}<span class="tab-name">${esc(tabName(tab))}</span></button></div>`).join('');}
Object.assign(words.ja,{homeHelp:'最初のページに戻る',fixedAddress:'固定表示のためアドレスは変更できません',openExternal:'既定のブラウザで開く',closeSettings:'設定を閉じる',collapseSection:'折りたたむ',expandSection:'展開する',groupHint:'選ぶか入力',resizeSidebar:'ドラッグで幅を変更（ダブルクリックで元に戻す）',allBookmarks:'すべてのブックマーク',searchBookmarks:'ブックマークを検索',gridView:'タイル表示',listView:'リスト表示',pinToSidebar:'サイドバーにピン留め',unpinFromSidebar:'サイドバーから外す',bookmarkDropHint:'ブックマークをここへドラッグしてピン留め',bookmarkDragHint:'サイドバーの「ブックマーク」へドラッグすると、サイドバーにピン留めできます。',noBookmarkResults:'一致するブックマークはありません。',collapseSidebar:'サイドバーを閉じる',expandSidebar:'サイドバーを開く',minimise:'最小化',maximise:'最大化',restore:'元に戻す',theme:'テーマ',themeSystem:'システムに合わせる',adblock:'広告ブロック',adblockEnable:'広告とトラッカーをブロックする',adblockHelp:'EasyList・EasyPrivacy・AdGuard日本語フィルタで広告を止め、広告があった場所の空欄も詰めます。フィルタは EasyList（easylist.to）と AdGuard が公開しているリストで、自動で更新されます。tarkov.dev は広告で運営されているため対象外です。切り替えると表示中のページを再読み込みします。',mapTab:'TARKOV.DEV',mapTabHelp:'tarkov.dev のマップ。Remote Control で接続済みのまま、検出したマップと位置を表示します。',tabs:'タブ',bookmarks:'ブックマーク',hostMode:'Hostモード',clientMode:'Clientモード',menuHint:'MAYAK メニュー',noTabs:'ロゴのメニューからサイトや設定を開けます。'});
Object.assign(words.en,{homeHelp:'Back to the start page',fixedAddress:'This view is fixed; its address cannot be edited',openExternal:'Open in default browser',closeSettings:'Close settings',collapseSection:'Collapse',expandSection:'Expand',groupHint:'Choose or type',resizeSidebar:'Drag to resize (double-click to reset)',allBookmarks:'All bookmarks',searchBookmarks:'Search bookmarks',gridView:'Tiles',listView:'List',pinToSidebar:'Pin to sidebar',unpinFromSidebar:'Remove from sidebar',bookmarkDropHint:'Drag bookmarks here to pin them',bookmarkDragHint:'Drag a bookmark onto “Bookmarks” in the sidebar to pin it there.',noBookmarkResults:'No bookmarks match.',collapseSidebar:'Close sidebar',expandSidebar:'Open sidebar',minimise:'Minimize',maximise:'Maximize',restore:'Restore',theme:'Theme',themeSystem:'Match system',adblock:'Ad blocking',adblockEnable:'Block ads and trackers',adblockHelp:'Blocks ads with EasyList, EasyPrivacy and the AdGuard Japanese filter, and closes the gaps they leave. The filter lists are published by EasyList (easylist.to) and AdGuard and update automatically. tarkov.dev is excluded because ads fund it. Changing this reloads the current page.',mapTab:'TARKOV.DEV',mapTabHelp:'tarkov.dev map, kept connected via Remote Control. Detected maps and positions appear here.',tabs:'Tabs',bookmarks:'Bookmarks',hostMode:'Host mode',clientMode:'Client mode',menuHint:'MAYAK menu',noTabs:'Open a website or settings from the logo menu.'});
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
// Sidebar section with the bosses: where the Goons were last reported and the
// bosses of the map being played; its icon opens the boss page.
// The heading opens and closes the section, back to how it was open; a row
// under the Goons shows or hides the map's bosses.
let bossOpenView='full';
// The map and the game mode, by hand or (the first choice) following the game.
function bossPick(info){
 const current=info.maps.find(m=>m.key===info.current),mode=info.mode==='pve'?'PvE':'PvP';
 const maps=[['',current?`${t('bossAuto')} (${current.name})`:t('bossAuto')],...info.maps.filter(m=>m.bosses.length).map(m=>[m.key,m.name])];
 const modes=[['',state.bossMode?t('bossAuto'):`${t('bossAuto')} (${mode})`],['regular','PvP'],['pve','PvE']];
 return `<div class="boss-pick"><select data-scope="preferences" data-key="bossMap" aria-label="${esc(t('bossMaps'))}">${maps.map(([v,l])=>option(v,l,state.bossMap)).join('')}</select><select data-scope="preferences" data-key="bossMode" aria-label="${esc(t('bossModeLabel'))}">${modes.map(([v,l])=>option(v,l,state.bossMode)).join('')}</select></div>`;
}
// The Goons report: the map, the account and mode (from the EFT logs) and the
// raid; nothing is sent before the user confirms. goonDraft keeps what was
// chosen across re-renders.
let goonDraft={};
function goonReportDialog(info){
 const r=state.goonReport,raid=r.raid;
 const close=`<button data-action="goonReportClose">${esc(t(r.done?'close':'cancel'))}</button>`;
 if(r.done)return `<div class="goon-dialog-backdrop"><div class="goon-dialog" role="dialog" aria-label="${esc(t('goonReport'))}"><h2>${esc(t('goonReport'))}</h2><p>${esc(t('goonReported'))}</p><div class="actions">${close}</div></div></div>`;
 const maps=info.maps.filter(m=>m.bosses.some(b=>b.id==='bossKnight'));
 const mapKey=goonDraft.map||(raid&&maps.some(m=>m.key===raid.map)?raid.map:maps.some(m=>m.key===(state.bossMap||info.current))?state.bossMap||info.current:maps[0]?.key||'');
 const modeName=mode=>mode==='pve'?'PvE':'PvP';
 const ids=r.identities.map(id=>({value:id.accountId+'|'+id.mode,label:[id.accountId,modeName(id.mode),id.current?t('goonCurrent'):'',id.lastSeen?`${t('goonLastSeen')} ${new Date(id.lastSeen).toLocaleDateString(state.language==='ja'?'ja-JP':'en-US')}`:''].filter(Boolean).join(' · ')}));
 const raidID=raid?raid.accountId+'|'+raid.mode:'';
 const account=goonDraft.account||(ids.some(i=>i.value===raidID)?raidID:ids[0]?.value||'');
 const canRaid=raid&&!raid.reported;
 const when=goonDraft.when??(canRaid?raid.startedAt:'');
 const raidLine=raid?`${raid.active?t('goonRaidNow'):t('goonRaidLast')}: ${maps.find(m=>m.key===raid.map)?.name||raid.map} · ${clock(raid.startedAt)} ${t('goonStarted')}${raid.reported?' · '+t('goonAlready'):''}`:'';
 return `<div class="goon-dialog-backdrop"><form class="goon-dialog" id="goon-form" role="dialog" aria-label="${esc(t('goonReport'))}"><h2>${esc(t('goonReport'))}</h2>
 <p class="hint">${esc(t('goonReportOnly'))}</p>
 <label class="field"><span>${esc(t('shotInfoMap'))}</span><select id="goon-map">${maps.map(m=>option(m.key,m.name,mapKey)).join('')}</select></label>
 <label class="field"><span>${esc(t('goonAccount'))}</span>${ids.length?`<select id="goon-account">${ids.map(i=>option(i.value,i.label,account)).join('')}</select>`:`<p class="notice">${esc(t('goonNoAccount'))}</p>`}</label>
 <fieldset class="goon-when"><legend>${esc(t('goonWhen'))}</legend>${raid?`<label class="check"><input type="radio" name="goon-when" value="${esc(raid.startedAt)}" ${when===raid.startedAt?'checked':''} ${canRaid?'':'disabled'}>${esc(raidLine)}</label>`:''}<label class="check"><input type="radio" name="goon-when" value="" ${when===''?'checked':''}>${esc(t('goonWhenNow'))}</label></fieldset>
 <p class="goon-consent">${esc(t('goonConsent'))}</p>
 ${r.error?`<p class="notice" role="alert">${esc(r.error)}</p>`:''}
 <div class="actions">${close}<button class="primary" data-action="goonReportSend" ${!ids.length||!maps.length||r.busy?'disabled':''}>${esc(t(r.busy?'goonSending':'goonSend'))}</button></div></form></div>`;
}
const bossName=b=>b.id==='bossKnight'?'Goons':b.name;
const pct=value=>Math.round((Number(value)||0)*100)+'%';
// How fresh a Goons report is: within an hour (a raid or two), three hours,
// or older.
function goonFresh(time){const minutes=(Date.now()-Date.parse(time))/60000;return !(minutes>=0)?'old':minutes<=60?'new':minutes<=180?'recent':'old';}
const goonLabel=goon=>goon?`Goons · ${goon.map} · ${age(goon.time,state.language)}`:`Goons · ${t('noGoons')}`;
function bossSection(){
 const info=state.bosses;
 const open=state.tabs.find(tab=>tab.id===state.active)?.kind==='bosses';
 const view=state.bossesView,goon=info?.goons[0];
 let body='';
 if(view!=='closed'){
  const latest=!info?`<p class="shot-empty">${esc(t('bossesLoading'))}</p>`:goon?`<button class="goon-latest" data-action="bosses" data-fresh="${goonFresh(goon.time)}" title="${esc(goonLabel(goon))}"><span class="goon-dot"></span><span class="goon-name">Goons</span><span class="goon-age" data-goon-time="${esc(goon.time)}">${esc(age(goon.time,state.language))}</span><b>${esc(goon.map)}</b></button>`:`<p class="shot-empty">Goons · ${esc(t('noGoons'))}</p>`;
  const map=info?.maps.find(m=>m.key===(state.bossMap||info.current));
  const here=!info?'':`<div class="boss-here">${bossPick(info)}${!map?`<p class="shot-empty">${esc(t('bossPickHint'))}</p>`:map.bosses.length?map.bosses.slice(0,5).map(b=>`<div class="boss-line"><span>${esc(bossName(b))}</span><b>${pct(b.chance)}</b></div>`).join(''):`<p class="shot-empty">${esc(t('noBosses'))}</p>`}</div>`;
  body=`<div class="boss-section">${latest}<button class="boss-more" data-action="bossDetail" aria-expanded="${view==='full'}">${esc(t('bossMapBosses'))}${icon('chevron','section-chevron')}</button>${view==='full'?here:''}</div>`;
 }
 return `<div class="section-label boss-section-label ${open?'active':''}"><button class="section-link" data-action="toggleBossSection" aria-expanded="${view!=='closed'}" title="${esc(t(view==='closed'?'expandSection':'collapseSection'))}">${esc(t('bosses'))}${icon('chevron','section-chevron')}</button><span class="section-actions"><button class="new-tab goon-report-open" data-action="goonReportFromSidebar" title="${esc(t('goonReport'))}" aria-label="${esc(t('goonReport'))}">${icon('flag')}</button><button class="new-tab bosses-open" data-action="bosses" title="${esc(info?goonLabel(goon):t('allBosses'))}" aria-label="${esc(t('allBosses'))}" aria-pressed="${open}">${icon('skull')}</button></span></div>${body}`;
}
// The boss page: the Goons reports and the maps they spawn on, then the
// bosses of one map (the one being played, or the one chosen).
function bossesPage(){
 const info=state.bosses;
 if(!info)return `<div class="page bosses-page"><p class="empty-tabs">${esc(t('bossesLoading'))}</p></div>`;
 const goons=info.goons.slice(0,10).map((g,i)=>`<li class="${i===0?'latest':''}" data-fresh="${goonFresh(g.time)}"><span class="goon-dot"></span><b>${esc(g.map)}</b><span class="goon-age" data-goon-time="${esc(g.time)}">${esc(age(g.time,state.language))}</span><time>${esc(clock(g.time))}</time></li>`).join('');
 const goonMaps=info.maps.map(m=>({m,b:m.bosses.find(b=>b.id==='bossKnight')})).filter(x=>x.b).sort((a,b)=>b.b.chance-a.b.chance).map(({m,b})=>`<button class="boss-chip" data-action="bossMap" data-id="${esc(m.key)}">${esc(m.name)} <b>${pct(b.chance)}</b></button>`).join('');
 const withBosses=info.maps.filter(m=>m.bosses.length);
 const chosen=withBosses.find(m=>m.key===(state.bossMap||info.current))||withBosses[0];
 const maps=withBosses.map(m=>`<button class="${m===chosen?'selected':''}" data-action="bossMap" data-id="${esc(m.key)}" aria-pressed="${m===chosen}">${esc(m.name)}${m.key===info.current?' ●':''}</button>`).join('');
 const cards=chosen?chosen.bosses.map(b=>`<article class="boss-card"><header>${b.portrait?`<img src="${esc(b.portrait)}" alt="" referrerpolicy="no-referrer" loading="lazy">`:`<span class="boss-portrait">${icon('skull')}</span>`}<h3>${esc(bossName(b))}${b.id==='bossKnight'?`<small>${esc([b.name,...b.escorts.map(e=>e.name)].join(' · '))}</small>`:''}</h3><strong>${pct(b.chance)}</strong></header>${b.locations.length?`<ul class="boss-places">${b.locations.map(l=>`<li><span>${esc(l.name)}</span><b>${pct(l.chance)}</b></li>`).join('')}</ul>`:''}${b.escorts.length?`<p class="boss-escorts">${esc(t('escorts'))}: ${b.escorts.map(e=>esc(e.name)+' ×'+(e.min===e.max?e.max:e.min+'–'+e.max)).join(', ')}</p>`:''}</article>`).join(''):'';
 return `<div class="page bosses-page"><div class="bookmarks-head"><h1>${esc(t('bosses'))}</h1>${bossPick(info)}</div>
 <section class="panel goon-panel"><div class="goon-head"><h2>Goons</h2><button data-action="goonReportOpen">${icon('flag')}<span>${esc(t('goonReport'))}</span></button></div>${goons?`<ul class="goon-list">${goons}</ul>`:`<p class="shot-empty">${esc(t('noGoons'))}</p>`}<p class="hint">${esc(t('goonHint'))}</p>${goonMaps?`<h3 class="goon-maps-head">${esc(t('goonMaps'))}</h3><div class="boss-chips">${goonMaps}</div>`:''}</section>
 <section class="boss-maps"><div class="boss-map-tabs" role="group" aria-label="${esc(t('bossMaps'))}">${maps}</div><div class="boss-grid">${cards}</div></section></div>${state.goonReport?goonReportDialog(info):''}`;
}
const clock=time=>{const d=new Date(time);return Number.isNaN(d.getTime())?'':d.toLocaleTimeString(state.language==='ja'?'ja-JP':'en-US',{hour:'2-digit',minute:'2-digit',hour12:state.clock==='12'});};
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
function screenshotsPage(){
 const shots=state.screenshots;
 if(!shots)return `<div class="page"><p class="empty-tabs">${esc(t('screenshotsHostOnly'))}</p></div>`;
 const grid=shots.list.map(s=>`<button class="shot-cell" data-action="screenshotView" data-id="${esc(s.name)}" title="${esc([s.name,shotSummary(s.meta)].filter(Boolean).join('\n'))}">${shots.thumbs[s.name]?`<img src="${shots.thumbs[s.name]}" alt="" loading="lazy">`:`<span class="shot-placeholder">${icon('image')}</span>`}<span class="shot-age">${esc(age(s.time,state.language))}</span>${shotBadges(s.meta)}</button>`).join('');
 return `<div class="page shots-page"><div class="bookmarks-head"><h1>${esc(t('screenshots'))}</h1><div class="bookmarks-tools"><button data-action="screenshotFolder">${icon('folder')}<span>${esc(t('openScreenshotFolder'))}</span></button></div></div>${shots.list.length?`<div class="shot-grid">${grid}</div>`:`<p class="shot-empty">${esc(t('noScreenshots'))}</p>`}</div>${shotViewer()}`;
}
// A screenshot's kind: a flea offer is an item screenshot of its own layout.
const shotKind=meta=>meta.type==='item'&&meta.layout==='flea-offer'?'offer':meta.type;
function shotBadges(meta){
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
function bookmarksPage(){
 const query=bookmarkQuery.trim().toLowerCase();
 const found=state.bookmarks.filter(b=>!query||b.name.toLowerCase().includes(query)||b.url.toLowerCase().includes(query));
 const groups=allGroups().map(g=>[g,found.filter(b=>b.group===g)]).filter(([,items])=>items.length);
 const view=state.bookmarkView;
 return `<div class="page bookmarks-page"><div class="bookmarks-head"><h1>${t('bookmarks')}</h1><div class="bookmarks-tools"><label class="bookmark-search">${icon('search')}<input id="bookmark-search" type="search" autocomplete="off" placeholder="${esc(t('searchBookmarks'))}" aria-label="${esc(t('searchBookmarks'))}" value="${esc(bookmarkQuery)}"></label><div class="segmented" role="group"><button data-action="bookmarkView" data-id="grid" class="${view==='grid'?'selected':''}" title="${esc(t('gridView'))}" aria-label="${esc(t('gridView'))}">${icon('grid')}</button><button data-action="bookmarkView" data-id="list" class="${view==='list'?'selected':''}" title="${esc(t('listView'))}" aria-label="${esc(t('listView'))}">${icon('list')}</button></div><button class="primary" data-action="addBookmark">${icon('plus')}<span>${t('addBookmark')}</span></button></div></div><p class="hint">${t('bookmarkDragHint')}</p>${bookmarkEditor()}${groups.map(([g,items])=>`<section class="bookmark-group"><h3>${esc(groupName(g))}<span class="count">${items.length}</span></h3><div class="bookmark-${view}">${items.map(bookmarkItem).join('')}</div></section>`).join('')||`<p class="empty-tabs">${t(query?'noBookmarkResults':'empty')}</p>`}</div>`;
}
function bookmarkEditor(){if(!editingBookmark)return '';const b=editingBookmark;return `<form id="bookmark-form" class="panel editor"><label class="field"><span>${t('name')}</span><input name="name" required maxlength="150" value="${esc(b.name)}"></label><label class="field"><span>${t('url')}</span><input name="url" type="url" required value="${esc(b.url)}" placeholder="https://"></label><label class="field"><span>${t('group')}</span><input name="group" list="bookmark-groups" maxlength="40" autocomplete="off" placeholder="${esc(t('groupHint'))}" value="${esc(groupName(b.group))}"><datalist id="bookmark-groups">${allGroups().map(g=>`<option value="${esc(groupName(g))}"></option>`).join('')}</datalist></label><div class="actions"><button type="submit" class="primary">${t('done')}</button><button type="button" data-action="cancelBookmark">${t('cancel')}</button></div></form>`;}
Object.assign(words.ja,{tracker_connected:'同期中',tracker_connecting:'接続中','tracker_waiting-profile':'プロフィール待ち','tracker_missing-token':'APIキー未設定',tracker_error:'エラー',tracker_disabled:'オフ',monitorOn:'監視中',monitorOff:'監視停止中',monitorStart:'クリックで監視を開始',monitorStop:'クリックで監視を停止',inRaid:'レイド中',outRaid:'レイド外',trackerLabel:'TarkovTracker',taskSiteHelp:'タスクを認識したときと、アイテム情報からタスクを開くときのサイトです。tarkov.dev の Remote Control へ送るかどうかは「tarkov.dev 連携」の接続先ごとに選べます。',taskSiteClientHelp:'「Hostの設定に従う」にすると、Host側で選んだサイトで開きます。',settingsBack:'設定を閉じる',grpGeneral:'一般',grpGame:'ゲーム',grpLinks:'連携',grpBrowser:'ブラウザ',grpDiagnostics:'状態と診断',sec_appearance:'表示',sec_tasks:'タスクの開き方',sec_adblock:'広告ブロック',sec_connection:'他のPCとの接続',sec_status:'ステータス',sec_logs:'ログ',sec_folders:'フォルダと保存',sec_recognition:'ゲームと認識',sec_remote:'tarkov.dev 連携',sec_tracker:'TarkovTracker',sec_sounds:'通知',sec_startup:'起動とウィンドウ',sec_debug:'デバッグ',grpAbout:'情報',sec_about:'MAYAK について',aboutVersion:'バージョン',aboutDev:'開発ビルド',aboutTagline:'Escape from Tarkov のスクリーンショットとログから、マップ・タスク・アイテム情報を映すコンパニオン。',aboutSite:'公式サイト',aboutSource:'ソースコード',aboutReleases:'リリースノート',aboutLicense:'GPL-3.0 で公開しています。',aboutCredits:'マップは tarkov.dev、タスクの進捗は TarkovTracker のデータと API を使っています。Escape from Tarkov は Battlestate Games の商標で、MAYAK は非公式のツールです。'});
Object.assign(words.en,{tracker_connected:'synced',tracker_connecting:'connecting','tracker_waiting-profile':'waiting for profile','tracker_missing-token':'no API key',tracker_error:'error',tracker_disabled:'off',monitorOn:'Monitoring',monitorOff:'Not monitoring',monitorStart:'Click to start monitoring',monitorStop:'Click to stop monitoring',inRaid:'In raid',outRaid:'Out of raid',trackerLabel:'TarkovTracker',taskSiteHelp:'Where recognized tasks and tasks opened from the item sidebar open. Whether tasks go to tarkov.dev Remote Control is set per target under tarkov.dev link.',taskSiteClientHelp:'"Follow Host setting" opens the site chosen on the Host.',settingsBack:'Close settings',grpGeneral:'General',grpGame:'Game',grpLinks:'Connections',grpBrowser:'Browser',grpDiagnostics:'Status & diagnostics',sec_appearance:'Appearance',sec_tasks:'Opening tasks',sec_adblock:'Ad blocking',sec_connection:'Other computers',sec_status:'Status',sec_logs:'Logs',sec_folders:'Folders & storage',sec_recognition:'Game & recognition',sec_remote:'tarkov.dev link',sec_tracker:'TarkovTracker',sec_sounds:'Notifications',sec_startup:'Startup & window',sec_debug:'Debug',grpAbout:'About',sec_about:'About MAYAK',aboutVersion:'Version',aboutDev:'development build',aboutTagline:'A companion that turns Escape from Tarkov screenshots and logs into map, task and item information.',aboutSite:'Website',aboutSource:'Source code',aboutReleases:'Release notes',aboutLicense:'Released under the GPL-3.0.',aboutCredits:'Maps come from tarkov.dev and task progress from TarkovTracker, through their data and APIs. Escape from Tarkov is a trademark of Battlestate Games; MAYAK is an unofficial tool.'});
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
function browserSettings(key){
 switch(key){
 case 'about':return `<section class="panel about"><div class="about-head"><img src="/favicon.svg" alt="" width="56" height="56"><div><h2>MAYAK</h2><p class="about-version">${esc(t('aboutVersion'))} ${esc(appVersion||t('aboutDev'))}</p></div></div><p>${esc(t('aboutTagline'))}</p><div class="about-links"><button data-action="open" data-id="https://mayak.ich.sh">${icon('globe')}${esc(t('aboutSite'))}</button><button data-action="open" data-id="https://github.com/ichi0g0y/mayak">${icon('external')}${esc(t('aboutSource'))}</button><button data-action="open" data-id="https://github.com/ichi0g0y/mayak/releases">${icon('list')}${esc(t('aboutReleases'))}</button></div><p class="hint">${esc(t('aboutLicense'))} ${esc(t('aboutCredits'))}</p></section>`;
 case 'appearance':return `<section class="panel"><div class="fields">${select('language',t('language'),[['ja','日本語'],['en','English']],state.language)}${select('theme',t('theme'),themes.map(key=>[key,key==='system'?t('themeSystem'):themeNames[key]]),state.theme)}${select('clock',t('clockFormat'),[['24',t('clock24')],['12',t('clock12')]],state.clock)}${select('navPosition',t('navPosition'),[['left',t('placeLeft')],['right',t('placeRight')],['top',t('placeTop')]],state.layout==='horizontal'?'top':state.sidebarSide)}${select('itemDock',t('itemDock'),[['right',t('placeRight')],['left',t('placeLeft')],['bottom',t('placeBottom')]],state.itemDock)}</div></section>`;
 case 'tasks':{
  // On the Host the site is its setting; a receiving computer may follow it.
  const local=state.localHost&&state.connection.mode==='local';
  const site=local?`<label class="field"><span>${esc(t('questSite'))}</span><select id="host-quest-site">${siteChoices().map(([v,l])=>option(v,l,state.hostQuestSite)).join('')}</select></label>`:select('questSite',t('questSite'),[['host',t('hostChoice')],...siteChoices()],state.questSite);
  return `<section class="panel"><div class="fields">${site}${select('taskMode',t('taskMode'),[['new',t('new')],['reuse',t('reuse')]],state.taskMode)}</div><p class="hint">${esc(t(local?'taskSiteHelp':'taskSiteClientHelp'))} ${t('taskHelp')}</p></section>`;
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
Object.assign(words.ja,{placement:'配置を変更',navPosition:'タブの位置',itemDock:'アイテム情報の位置',placeLeft:'左',placeRight:'右',placeTop:'上',placeBottom:'下',openItemPage:'アイテムのページを開く',itemSearch:'アイテムを検索',itemSearchHint:'上のルーペからアイテム名で検索できます。ゲームでアイテムを調べてスクショを撮っても、ここに情報が出ます。',itemNoResults:'見つかりませんでした',openTask:'タスクを開く',itemPanel:'アイテム情報',itemShow:'アイテム情報を表示',itemHide:'アイテム情報を閉じる',itemRefresh:'最新の価格を取得',itemLive:'LIVE',itemCatalog:'カタログ',itemPriced:'価格更新',bestSale:'一番高く売れる先',perSlot:'1マス',flea:'フリーマーケット',fleaLow:'最安値',fleaAvg:'24時間平均',fleaRange:'24時間の幅',fleaChange:'48時間の変動',fleaOffers:'出品数',fleaLevel:'利用可能レベル',noFlea:'フリーマーケットでは売れません',traders:'トレーダー買取',itemTasks:'必要なタスク',itemHideout:'必要なハイドアウト',noNeeds:'なし',fir:'FIR',needDone:'完了',progressUnknown:'TarkovTrackerと同期すると完了済みがわかります'});
Object.assign(words.en,{placement:'Change the layout',navPosition:'Tabs position',itemDock:'Item details position',placeLeft:'Left',placeRight:'Right',placeTop:'Top',placeBottom:'Bottom',openItemPage:'Open the item page',itemSearch:'Search items',itemSearchHint:'Search by item name with the magnifier above, or inspect an item in the game and take a screenshot to see it here.',itemNoResults:'No items found',openTask:'Open task',itemPanel:'Item',itemShow:'Show item details',itemHide:'Close item details',itemRefresh:'Get the latest prices',itemLive:'LIVE',itemCatalog:'Catalog',itemPriced:'Prices updated',bestSale:'Sells best to',perSlot:'per slot',flea:'Flea market',fleaLow:'Lowest offer',fleaAvg:'24h average',fleaRange:'24h range',fleaChange:'48h change',fleaOffers:'Offers',fleaLevel:'Unlocks at level',noFlea:'Cannot be sold on the flea market',traders:'Trader buy prices',itemTasks:'Needed for tasks',itemHideout:'Needed for hideout',noNeeds:'None',fir:'FIR',needDone:'Done',progressUnknown:'Sync TarkovTracker to see what is done'});
Object.assign(words.ja,{priceHistory:'価格の推移',range7d:'7日',range30d:'30日',rangeAll:'全期間',avgLine:'平均',minLine:'最安値',chartHigh:'最高',chartLow:'最安',chartLoading:'価格の履歴を読み込み中…',chartFailed:'価格の履歴を取得できませんでした',chartEmpty:'この期間の履歴はありません',chartNow:'現在'});
Object.assign(words.en,{priceHistory:'Price history',range7d:'7D',range30d:'30D',rangeAll:'All',avgLine:'Average',minLine:'Lowest',chartHigh:'High',chartLow:'Low',chartLoading:'Loading price history…',chartFailed:'Could not load the price history',chartEmpty:'No history for this period',chartNow:'Now'});
// The chart's range is a per-viewer convenience, remembered in this browser.
let chartRange='7d',chart=null;
try{chartRange=localStorage.getItem('mayak.chartRange')||'7d';}catch{}
if(!['7d','30d','all'].includes(chartRange))chartRange='7d';
// The plot keeps chartPad free above and below its lines; four grid lines
// with prices mark the scale.
const chartWidth=288,chartHeight=132,chartPad=14,chartTicks=4;
const shortPrice=v=>v>=1e6?`${(v/1e6).toFixed(v>=1e7?0:1)}M`:v>=1e3?`${Math.round(v/1e3)}k`:String(Math.round(v));
const chartDate=(t,withTime=false)=>new Date(t).toLocaleString(state.language==='ja'?'ja-JP':'en-US',chartRange==='all'&&!withTime?{year:'numeric',month:'numeric'}:withTime?{month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit',hour12:state.clock==='12'}:{month:'numeric',day:'numeric'});
// The flea market price history: average and lowest offer. The lowest line
// ends at the live price, since the history lags it by a few hours.
function itemChart(item){
 const h=state.itemHistory;chart=null;
 if(!item.flea||!h||h.id!==item.id)return '';
 const money=v=>esc(price(v,'RUB',state.language));
 const ranges=[['7d','range7d'],['30d','range30d'],['all','rangeAll']];
 const head=`<h3>${esc(t('priceHistory'))}<span class="chart-ranges" role="group">${ranges.map(([k,l])=>`<button data-action="chartRange" data-id="${k}" class="${chartRange===k?'selected':''}" aria-pressed="${chartRange===k}">${esc(t(l))}</button>`).join('')}</span></h3>`;
 const at=Date.parse(item.pricedAt);
 const s=h.points.length?chartSeries(h.points,chartRange,Date.now(),item.flea.lastLow?{t:Number.isNaN(at)?Date.now():at,min:item.flea.lastLow}:null):null;
 if(!s)return `<section class="item-section item-chart">${head}<p class="item-empty">${esc(t(h.loading?'chartLoading':h.failed?'chartFailed':'chartEmpty'))}</p></section>`;
 chart=s;
 const change=s.change;
 return `<section class="item-section item-chart">${head}
 <div class="chart-stats"><span>${esc(t('chartHigh'))} <b>${money(s.high)}</b></span><span>${esc(t('chartLow'))} <b>${money(s.low)}</b></span><span class="${change>0?'up':change<0?'down':''}">${change>0?'+':''}${change.toFixed(1)}%</span></div>
 <div class="chart-box">${chartGrid(s)}<svg class="price-chart" viewBox="0 0 ${chartWidth} ${chartHeight}" preserveAspectRatio="none" aria-hidden="true">${Array.from({length:chartTicks},(_,i)=>{const y=chartPad+(chartHeight-2*chartPad)*i/(chartTicks-1);return `<line class="chart-grid" x1="0" x2="${chartWidth}" y1="${y}" y2="${y}"/>`;}).join('')}<g transform="translate(0 ${chartPad})"><path class="chart-min" d="${chartPath(s,'min',chartWidth,chartHeight-2*chartPad)}"/><path class="chart-avg" d="${chartPath(s,'price',chartWidth,chartHeight-2*chartPad)}"/></g><line class="chart-cursor" x1="0" x2="0" y1="0" y2="${chartHeight}"/></svg><div class="chart-tip" hidden></div></div>
 <div class="chart-axis"><span>${esc(chartDate(s.t0))}</span><span class="chart-legend"><i class="avg"></i>${esc(t('avgLine'))}<i class="min"></i>${esc(t('minLine'))}</span><span>${esc(chartDate(s.t1))}</span></div></section>`;
}
// Price labels for the grid lines, in a gutter left of the plot and as HTML
// so the stretched SVG does not distort them. The top line is the highest
// price of the scale.
function chartGrid(s){
 return Array.from({length:chartTicks},(_,i)=>{
  const top=(chartPad+(chartHeight-2*chartPad)*i/(chartTicks-1))/chartHeight*100;
  const value=s.hi-(s.hi-s.lo)*i/(chartTicks-1);
  return `<span class="chart-label" style="top:${top.toFixed(2)}%">${esc(shortPrice(value))}</span>`;
 }).join('');
}
// Hovering the chart shows the nearest sample without re-rendering the page.
document.addEventListener('pointermove',event=>{
 const box=event.target.closest?.('.chart-box');
 if(!box||!chart)return;
 // Positions come from the plot, which starts after the price labels.
 const r=box.querySelector('.price-chart').getBoundingClientRect(),offset=r.left-box.getBoundingClientRect().left,frac=Math.min(1,Math.max(0,(event.clientX-r.left)/r.width));
 const t0=chart.t0,span=Math.max(1,chart.t1-t0),at=t0+frac*span;
 let p=chart.pts[0];for(const q of chart.pts)if(Math.abs(q.t-at)<Math.abs(p.t-at))p=q;
 const x=(p.t-t0)/span;
 const line=box.querySelector('.chart-cursor');line.setAttribute('x1',x*chartWidth);line.setAttribute('x2',x*chartWidth);line.style.opacity=1;
 const tip=box.querySelector('.chart-tip'),money=v=>price(v,'RUB',state.language);
 tip.textContent=`${p.now?t('chartNow'):chartDate(p.t,true)} · ${p.price?`${t('avgLine')} ${money(p.price)} / `:''}${t('minLine')} ${money(p.min)}`;
 tip.hidden=false;
 tip.style.left=`${Math.min(Math.max(0,offset+x*r.width-tip.offsetWidth/2),box.clientWidth-tip.offsetWidth)}px`;
});
document.addEventListener('pointerout',event=>{
 const box=event.target.closest?.('.chart-box');
 if(!box||box.contains(event.relatedTarget))return;
 box.querySelector('.chart-tip').hidden=true;box.querySelector('.chart-cursor').style.opacity=0;
});
const modeNames={regular:'PvP',pve:'PvE','pvp-season':'Season'};
// The item sidebar: a search box, then the item (found or recognized):
// prices first (what to do with the item now), then what still needs it.
// Completed tasks and hideout levels sink to the bottom.
function itemPanel(){
 if(!state.itemOpen)return '';
 const bottom=state.itemDock==='bottom';
 return `<aside class="item-panel" aria-label="${esc(t('itemPanel'))}"><div class="item-resizer" role="separator" aria-orientation="${bottom?'horizontal':'vertical'}" aria-valuemin="${bottom?itemPanelHeights.min:itemPanelWidths.min}" aria-valuemax="${bottom?itemPanelHeights.max:itemPanelWidths.max}" aria-valuenow="${bottom?state.itemPanelHeight:state.itemPanelWidth}" title="${esc(t('resizeSidebar'))}"></div><div class="item-titlebar"><button class="item-button" data-action="itemClose" title="${esc(t('itemHide'))}" aria-label="${esc(t('itemHide'))}">${icon({left:'panelLeft',bottom:'panelBottom'}[state.itemDock]||'panelRight')}</button><button class="item-button ${itemSearchOpen?'selected':''}" data-action="itemSearchToggle" title="${esc(t('itemSearch'))}" aria-label="${esc(t('itemSearch'))}" aria-expanded="${itemSearchOpen}">${icon('search')}</button></div>${itemSearchOpen?itemSearchPopup():''}<div class="item-body"><div class="item-flow">${state.item?itemDetails():`<p class="item-start">${esc(t('itemSearchHint'))}</p>`}</div></div></aside>`;
}
// The search popup floats over the item, so the item does not move.
let itemSearchOpen=false;
function itemSearchPopup(){
 const search=state.itemSearch||{query:'',results:[]};
 const results=search.query.trim()?(search.results.length?`<ul class="item-results">${search.results.map(r=>`<li><button data-action="itemSelect" data-id="${esc(r.id)}" title="${esc(r.name)}">${r.iconUrl?`<img src="${esc(r.iconUrl)}" alt="" referrerpolicy="no-referrer" loading="lazy">`:`<span class="result-icon">${icon('package')}</span>`}<span class="need-name">${esc(itemName(r))}<small>${esc(r.shortName)}</small></span></button></li>`).join('')}</ul>`:`<p class="item-empty">${esc(t('itemNoResults'))}</p>`):'';
 return `<div class="item-search-pop" role="dialog" aria-label="${esc(t('itemSearch'))}"><label class="item-search">${icon('search')}<input id="item-search" type="search" autocomplete="off" spellcheck="false" placeholder="${esc(t('itemSearch'))}" aria-label="${esc(t('itemSearch'))}" value="${esc(search.query)}"></label>${results}</div>`;
}
// The item's page opens on the task site, like the item's tasks.
// A name in the shell's language, when the catalog has one (see
// internal/locale); English otherwise.
const localName=(names,english)=>names?.[state.language]||english;
const itemName=item=>localName(item.names,item.name);
// The item's icon and name open its page, in the popup.
function itemPageLink(item){
 const site=state.localHost&&state.connection.mode==='local'?state.hostQuestSite:state.questSite==='host'?item.questSite:state.questSite;
 const url=itemPageURL(item,site);
 const label=siteChoices().find(([key])=>key===site)?.[1]||'tarkov.dev';
 return url?{url,label:`${t('openItemPage')} · ${label}`}:null;
}
function itemDetails(){
 const item=state.item;
 const lang=state.language,slots=item.width*item.height,best=bestSale(item),flea=item.flea;
 const money=(v,c='RUB')=>esc(price(v,c,lang));
 const row=(label,value,cls='')=>`<div class="item-row ${cls}"><span>${esc(label)}</span><b>${value}</b></div>`;
 const change=flea?.changePercent||0;
 const done=s=>s.state==='completed'||s.complete===true;
 // Rows with an action (tasks) are buttons that open their page.
 const needs=(rows,render,action)=>rows.length?`<ul class="item-needs">${[...rows].sort((a,b)=>done(a)-done(b)).map(s=>{const inner=`${render(s)}${s.foundInRaid?`<span class="fir" title="Found in raid">${t('fir')}</span>`:''}<span class="need-count">×${s.count.toLocaleString()}</span>${done(s)?`<span class="need-done" title="${esc(t('needDone'))}">${icon('check')}</span>`:''}`;return `<li class="${done(s)?'done':''}">${action?`<button class="need-row ${state.popup?.key==='task:'+s.id?'popup-source':''}" data-action="${action}" data-id="${esc(s.id)}" title="${esc(t('openTask'))}">${inner}</button>`:`<div class="need-row">${inner}</div>`}</li>`;}).join('')}</ul>`:`<p class="item-empty">${t('noNeeds')}</p>`;
 const open=rows=>rows.filter(s=>!done(s)).length;
 const unknown=item.tasks.some(s=>!s.state)||item.hideout.some(s=>s.complete===null);
 return `
 <div class="item-head">${(()=>{const link=itemPageLink(item),inner=`${item.iconUrl?`<img class="item-icon" src="${esc(item.iconUrl)}" alt="" referrerpolicy="no-referrer">`:`<span class="item-icon">${icon('package')}</span>`}<span class="item-title"><h2 title="${esc(item.name)}">${esc(itemName(item))}</h2><span>${esc(localName(item.shortNames,item.shortName))} · ${item.width}×${item.height} · ${esc(modeNames[item.mode]||item.mode)}</span></span>`;return link?`<button class="item-page-link ${state.popup?.key==='item:'+item.id?'popup-source':''}" data-action="itemPage" data-id="${esc(link.url)}" title="${esc(link.label)}">${inner}</button>`:`<div class="item-page-link">${inner}</div>`;})()}</div>
 <p class="item-fresh ${item.live?'live':''}"><span class="fresh-dot"></span>${esc(item.live?t('itemLive'):t('itemCatalog'))} · ${esc(t('itemPriced'))} <span class="item-age" data-time="${esc(item.pricedAt)}">${esc(age(item.pricedAt,lang))}</span><button class="item-refresh" data-action="itemRefresh" title="${esc(t('itemRefresh'))}" aria-label="${esc(t('itemRefresh'))}" aria-busy="${!!state.itemBusy}">${icon('reload',state.itemBusy?'icon spin':'icon')}</button></p>
 ${best?`<div class="item-best"><span>${esc(t('bestSale'))} · ${esc(best.where==='flea'?t('flea'):best.where)}</span><strong>${money(best.priceRub)}</strong><small>${money(best.priceRub/slots)} / ${esc(t('perSlot'))}</small></div>`:''}
 ${itemChart(item)}
 <section class="item-section"><h3>${esc(t('flea'))}</h3>${flea?row(t('fleaLow'),money(flea.lastLow))+row(t('fleaAvg'),money(flea.avg24h))+(flea.low24h&&flea.high24h?row(t('fleaRange'),`${money(flea.low24h)} – ${money(flea.high24h)}`):'')+row(t('fleaChange'),`${change>0?'+':''}${change.toFixed(1)}%`,change>0?'up':change<0?'down':'')+row(t('fleaOffers'),flea.offers.toLocaleString())+(flea.minLevel?row(t('fleaLevel'),'Lv.'+flea.minLevel):''):`<p class="item-empty">${t('noFlea')}</p>`}</section>
 <section class="item-section"><h3>${esc(t('traders'))}</h3>${item.traders.map((s,i)=>row(s.trader,s.currency==='RUB'?money(s.price):`${money(s.price,s.currency)} <small>(${money(s.priceRub)})</small>`,i===0?'best':'')).join('')||`<p class="item-empty">${t('noNeeds')}</p>`}</section>
 <section class="item-section"><h3>${esc(t('itemTasks'))}<span class="count">${open(item.tasks)}</span></h3>${needs(item.tasks,s=>`<span class="need-name" title="${esc(s.name)}">${esc(localName(s.names,s.name))}<small>${esc(s.trader)}</small></span>`,'itemTask')}</section>
 <section class="item-section"><h3>${esc(t('itemHideout'))}<span class="count">${open(item.hideout)}</span></h3>${needs(item.hideout,s=>`<span class="need-name">${esc(s.station)}<small>Lv.${s.level}</small></span>`)}</section>
 ${unknown&&(item.tasks.length||item.hideout.length)?`<p class="hint item-hint">${esc(t('progressUnknown'))}</p>`:''}
 `;
}
function itemToggle(){
 const label=t(state.itemOpen?'itemHide':'itemShow');
 return `<button class="dock-button item-toggle ${state.itemOpen?'selected':''}" data-action="${state.itemOpen?'itemClose':'itemOpen'}" title="${esc(label)}" aria-label="${esc(label)}" aria-pressed="${state.itemOpen}">${icon('package')}</button>`;
}
// Ages tick every half minute without re-rendering; prices refresh every
// few minutes while the panel is visible.
setInterval(()=>{
 document.querySelectorAll('.item-age').forEach(el=>{el.textContent=age(el.dataset.time,state?.language);});
 document.querySelectorAll('[data-goon-time]').forEach(el=>{el.textContent=age(el.dataset.goonTime,state?.language);el.closest('[data-fresh]')?.setAttribute('data-fresh',goonFresh(el.dataset.goonTime));});
},30000);
setInterval(()=>{if(state?.itemOpen&&document.visibilityState==='visible')void action('itemRefresh',{auto:true});},180000);

// A thin bar along the bottom of the address field while the page loads.
// Its animation is offset by the clock, so a re-render does not restart it.
const loadBar=tab=>tab&&state.loadingTabs?.includes(tab.id)?`<span class="load-bar" style="--load-offset:-${Date.now()%1400}ms"></span>`:'';
// Areas that scroll on their own, by selector.
let scrolledTab='';
const scrollAreas=['.item-body','.tab-strip','main','.item-results'];
// The update toast sits over the toolbar row, which the shell owns: the page
// views are native windows above the shell, so a toast anywhere under them
// would be covered. It stays until the update is applied or put off.
function updateToastHTML(){
 const u=state.update;
 if(!u||!['available','downloading','ready'].includes(u.state)||!u.latest||u.latest===updateDismissed)return '';
 const text=t(u.state==='ready'?'updateReady':u.state==='downloading'?'updateDownloading':'updateAvailable').replace('{v}',u.latest);
 const progress=u.state==='downloading'?`<span class="update-progress"><i style="width:${Math.max(0,Math.min(100,u.progress|0))}%"></i></span>`:'';
 const action=u.state==='ready'?`<button class="primary" data-action="updateInstall">${esc(t('updateRestart'))}</button>`:u.state==='available'?`<button class="primary" data-action="updateDownload">${esc(t('updateDownload'))}</button>`:'';
 const notes=u.releaseUrl?`<button data-action="updateNotes">${esc(t('updateNotes'))}</button>`:'';
 return `<aside class="toast update" role="status" title="${esc(u.state==='ready'?t('updateReadyHint'):'')}"><span class="update-text">${esc(text)}</span>${progress}${notes}${action}<button data-action="updateDismiss">${esc(t('updateLater'))}</button></aside>`;
}
const tutorialSteps=['welcome','folders','key','map','remote','tracker','done'];
function tutorialHTML(){
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
async function openTutorial(){
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
async function handleTutorial(type){
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
  document.documentElement.lang=state.language;document.body.dataset.layout=state.layout;document.body.dataset.item=state.itemOpen?state.itemDock:'closed';document.body.dataset.nav=state.layout==='horizontal'?'top':state.sidebarSide;document.body.dataset.settings=settingsHost()?'host':'';document.body.dataset.sidebar=state.layout==='vertical'&&state.sidebarCollapsed?'collapsed':'open';if(!sidebarDrag)document.documentElement.style.setProperty('--sidebar-width',state.sidebarWidth+'px');if(!itemDrag){document.documentElement.style.setProperty('--item-width',state.itemPanelWidth+'px');document.documentElement.style.setProperty('--item-height',state.itemPanelHeight+'px');}
  const tab=state.tabs.find(tab=>tab.id===state.active);
  const isWeb=tab?.kind==='web';
  // A rebuilt element under the pointer would fade into its hover colour
  // again on every render (a page loading in the popup renders often).
  document.documentElement.classList.add('rendering');requestAnimationFrame(()=>requestAnimationFrame(()=>document.documentElement.classList.remove('rendering')));
  // Rebuilding would scroll these back to the top: keep where they were.
  // The page area starts at the top on another tab.
  const scrolled=scrollAreas.map(sel=>{const el=document.querySelector(sel);return el&&[sel,el.scrollTop,el.scrollLeft];}).filter(Boolean);
  document.querySelector('#app').innerHTML=`<div class="sidebar-resizer" role="separator" aria-orientation="vertical" aria-valuemin="${sidebarWidths.min}" aria-valuemax="${sidebarWidths.max}" aria-valuenow="${state.sidebarWidth}" title="${esc(t('resizeSidebar'))}"></div>${brandBar()}<nav class="tab-strip ${tab?.kind==='settings'?'settings-strip':''}" aria-label="${esc(t(tab?.kind==='settings'?'settings':'tabHelp'))}">${tab?.kind==='settings'?settingsSidebar():`${mapEntry()}${bookmarkSection()}${screenshotSection()}${bossSection()}<div class="section-label"><span>${esc(t('tabs'))}</span><button class="new-tab" data-action="newTab" title="${esc(t('newTab'))}" aria-label="${esc(t('newTab'))}">${icon('plus')}</button></div><div class="tabs" role="tablist" style="--n:${tabCount()}">${tabs()}</div>`}</nav><div class="layout-dock">${layoutToggle()}${itemToggle()}<button class="dock-button dock-settings ${tab?.kind==='settings'?'selected':''}" aria-pressed="${tab?.kind==='settings'}" data-action="settings" title="${esc(t('settings'))}" aria-label="${esc(t('settings'))}">${icon('settings')}</button>${state.layout==='horizontal'?`<span class="dock-indicators">${indicators()}</span>`:''}</div>${windowControls()}<div class="toolbar">${tab?.fixed?`<button data-action="back" aria-label="${t('back')}" title="${t('back')}" ${!tab.canBack?'disabled':''}>${icon('back')}</button><button data-action="forward" aria-label="${t('forward')}" title="${t('forward')}" ${!tab.canForward?'disabled':''}>${icon('forward')}</button><button data-action="reload" aria-label="${t('reload')}" title="${t('reload')}">${icon('reload')}</button><button data-action="home" aria-label="${esc(t('home'))}" title="${esc(t('homeHelp'))}" ${tab.url===tab.home?'disabled':''}>${icon('home')}</button><form id="address-form" class="readonly">${icon('globe','address-icon')}<input id="address" readonly aria-readonly="true" aria-label="${esc(tabName(tab))}" title="${esc(t('fixedAddress'))}" value="${esc(tab.url)}">${loadBar(tab)}</form>`:tab?.kind==='bookmarks'||tab?.kind==='screenshots'||tab?.kind==='bosses'?'':tab?.kind==='settings'?'':`<button data-action="back" aria-label="${t('back')}" title="${t('back')}" ${!tab?.canBack?'disabled':''}>${icon('back')}</button><button data-action="forward" aria-label="${t('forward')}" title="${t('forward')}" ${!tab?.canForward?'disabled':''}>${icon('forward')}</button><button data-action="reload" aria-label="${t('reload')}" title="${t('reload')}" ${!isWeb?'disabled':''}>${icon('reload')}</button><form id="address-form">${icon('search','address-icon')}<input id="address" aria-label="${t('address')}" placeholder="${t('address')}" value="${esc(isWeb?tab.url:'')}">${loadBar(tab)}</form>${tab?.task?`<select id="task-site" aria-label="${t('questSite')}">${siteChoices().map(([key,label])=>option(key,label,sitesForURL(tab))).join('')}</select><button data-action="wikiSearch" title="${t('searchWiki')}" aria-label="${t('searchWiki')}">${icon('search')}</button>`:''}`}${isWeb?`<button class="open-external" data-action="openExternal" title="${esc(t('openExternal'))}" aria-label="${esc(t('openExternal'))}">${icon('external')}</button>`:''}${state.layout==='vertical'?'<div class="titlebar-grip"></div>':''}</div><main>${tab?.kind==='settings'?settings():tab?.kind==='bookmarks'?bookmarksPage():tab?.kind==='screenshots'?screenshotsPage():tab?.kind==='bosses'?bossesPage():!tab?`<p class="empty-tabs">${t('noTabs')}</p>`:''}</main>${tab?.kind==='settings'?`<button class="page-close" data-action="closeSettings" title="${esc(t('closeSettings'))}" aria-label="${esc(t('closeSettings'))}">${icon('x')}</button>`:''}${itemPanel()}${contextMenuHTML()}${placeMenuHTML()}${state.error?`<aside class="toast error" role="alert"><span>${esc(state.error)}</span><button data-action="dismiss" title="${esc(t('dismiss'))}" aria-label="${esc(t('dismiss'))}">${icon('x')}</button></aside>`:''}${updateToastHTML()}${tutorialOpen?tutorialHTML():''}`;
  for(const [sel,top,left] of scrolled){const el=document.querySelector(sel);if(el&&!(sel==='main'&&scrolledTab!==state.active)){el.scrollTop=top;el.scrollLeft=left;}}
  scrolledTab=state.active;
  if(focusId||focusKey||focusName){const target=[...document.querySelectorAll('input,select,textarea')].find(el=>focusId?el.id===focusId:focusKey?el.dataset.key===focusKey:el.name===focusName);if(target){if(draft!==undefined)target.value=draft;target.focus();try{target.setSelectionRange(start,start);}catch{}}}
}
function sitesForURL(tab){return Object.keys(tab.task?.urls||{}).find(key=>tab.task.urls[key]===tab.url)||'tarkov-dev';}
async function action(type,data){const next=await api.action(type,data);if(next){state=next;render();}}
document.addEventListener('click',async event=>{
  const button=event.target.closest('[data-action]');if(!button||button.disabled)return;const type=button.dataset.action,id=button.dataset.id;
  if(type.startsWith('tutorial')){void handleTutorial(type);return;}
  if(type==='updateDismiss'){updateDismissed=state.update?.latest||'';render();return;}
  if(type==='updateNotes'){if(state.update?.releaseUrl)void Browser.OpenURL(state.update.releaseUrl);return;}
  if(contextMenu){closeContextMenu();if(type==='closeContext')return;}
  if(placeMenu){closePlaceMenu();if(type==='closePlaceMenu'||type==='placeMenu')return;}
  if(type==='placeMenu'){openPlaceMenu(button);return;}
  if(type==='placeNav'){void action('preferences',{navPosition:id});return;}
  if(type==='placeItem'){void action('preferences',{itemDock:id});return;}
  if(type==='openExternal'){const url=webURL(state.tabs.find(t=>t.id===state.active)?.url);if(url)void Browser.OpenURL(url);return;}
  if(type==='chartRange'){chartRange=id;try{localStorage.setItem('mayak.chartRange',id);}catch{}render();return;}
  // The popup opens next to the row clicked.
  if(type==='itemTask'||type==='itemPage'){const r=button.getBoundingClientRect();const at={anchor:r.top+r.height/2};void action(type,type==='itemTask'?{id,...at}:{url:id,...at});return;}
  if(type==='itemSearchToggle'){if(itemSearchOpen)closeItemSearch();else openItemSearch();return;}
  if(type==='itemSelect'){itemSearchOpen=false;void action('itemSelect',id);return;}
  // Opened without an item, the sidebar starts with the search.
  if(type==='itemOpen'&&!state.item){await action('itemOpen');openItemSearch();return;}
  if(type==='newTab'){await action(type);document.querySelector('#address')?.focus();return;}
  if(type==='toggleSidebar'){void action('preferences',{sidebarCollapsed:!state.sidebarCollapsed});return;}
  if(type==='toggleBossSection'){if(state.bossesView!=='closed')bossOpenView=state.bossesView;void action('preferences',{bossesView:state.bossesView==='closed'?bossOpenView:'closed'});return;}
  if(type==='bossDetail'){void action('preferences',{bossesView:state.bossesView==='full'?'goons':'full'});return;}
  if(type==='goonReportFromSidebar'){goonDraft={};await action('bosses');void action('goonReportOpen');return;}
  if(type==='goonReportOpen'){goonDraft={};void action('goonReportOpen');return;}
  if(type==='goonReportSend'){event.preventDefault();const [accountId,mode]=(document.querySelector('#goon-account')?.value||'').split('|');void action('goonReportSend',{map:document.querySelector('#goon-map')?.value||'',accountId,mode,startedAt:document.querySelector('input[name=goon-when]:checked')?.value||''});return;}
  if(type==='bossMap'){void action('preferences',{bossMap:id});return;}
  if(type==='toggleScreenshotSection'){void action('preferences',{screenshotsCollapsed:!state.screenshotsCollapsed});return;}
  if(type==='toggleBookmarkSection'){void action('preferences',{bookmarksCollapsed:!state.bookmarksCollapsed});return;}
  if(type==='bookmarkView'){void action('preferences',{bookmarkView:id});return;}
  if(type==='windowMinimise'){void Window.Minimise();return;}
  if(type==='windowMaximise'){void Window.ToggleMaximise().then(syncMaximised);return;}
  if(type==='windowClose'){void Window.Close();return;}
  if(type==='shotInfo'){shotInfoOpen=!shotInfoOpen;render();return;}
  if(type==='shotZoom'){zoomShot(id==='fit'?1:shotZoom.scale*(id==='in'?1.25:0.8));return;}
  if(type==='adblock')return;
  if(type==='peerCopy'||type==='peerCopyPair')copied=true;
  if(['peerInvite','peerAccept','peerJoin','peerClose'].includes(type))copied=false;
  if(type==='addBookmark'){editingBookmark={name:'',url:'',group:'other'};render();document.querySelector('[name="name"]')?.focus();return;}
  if(type==='editBookmark'){editingBookmark={...state.bookmarks.find(b=>b.id===id)};render();return;}
  if(type==='cancelBookmark'){editingBookmark=null;render();return;}
  if(type==='bookmarkOpen'){void action('open',state.bookmarks.find(b=>b.id===id).url);return;}
  if(type==='wikiSearch'){const tab=state.tabs.find(t=>t.id===state.active);const japanese=sitesForURL(tab)==='japanese-wiki';void action('navigate',japanese?`https://wikiwiki.jp/eft/?cmd=search&word=${encodeURIComponent(tab.task.name)}`:`https://escapefromtarkov.fandom.com/wiki/Special:Search?query=${encodeURIComponent(tab.task.name)}`);return;}
  void action(type,id);
});
document.addEventListener('change',event=>{const input=event.target;if(input.id==='goon-map')goonDraft.map=input.value;if(input.id==='goon-account')goonDraft.account=input.value;if(input.name==='goon-when')goonDraft.when=input.value;if(input.closest('#bookmark-form')&&editingBookmark)editingBookmark[input.name]=input.value;if(input.dataset.scope)void action(input.dataset.scope,{[input.dataset.key]:input.value});else if(input.id==='task-site')void action('site',input.value);else if(input.id==='host-quest-site')void action('hostQuestSite',input.value);else if(input.dataset.action==='adblock')void action('preferences',{adblock:input.checked});});
let itemSearchTimer=0;
function openItemSearch(){itemSearchOpen=true;render();const input=document.querySelector('#item-search');input?.focus();input?.select();}
function closeItemSearch(){if(!itemSearchOpen)return;itemSearchOpen=false;clearTimeout(itemSearchTimer);void action('itemSearch','');}
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
// A click outside the popup closes it.
document.addEventListener('pointerdown',event=>{if(itemSearchOpen&&!event.target.closest('.item-search-pop,[data-action="itemSearchToggle"]'))closeItemSearch();},true);
document.addEventListener('keydown',event=>{
 if(event.target.id!=='item-search')return;
 if(event.key==='Enter'){const first=state.itemSearch?.results?.[0];if(first){event.preventDefault();clearTimeout(itemSearchTimer);itemSearchOpen=false;void action('itemSelect',first.id);}}
 else if(event.key==='Escape'){event.preventDefault();closeItemSearch();}
});
document.addEventListener('input',event=>{
 if(event.target.id==='item-search'){const value=event.target.value;clearTimeout(itemSearchTimer);itemSearchTimer=setTimeout(()=>void action('itemSearch',value),150);return;}if(event.target.id==='bookmark-search'){bookmarkQuery=event.target.value;render();return;}if(event.target.closest('#bookmark-form')&&editingBookmark)editingBookmark[event.target.name]=event.target.value;if(event.target.name==='peerOffer')peerDrafts.offer=event.target.value;if(event.target.name==='peerAnswer')peerDrafts.answer=event.target.value;if(event.target.name==='pairCode')peerDrafts.pairCode=event.target.value;});
document.addEventListener('submit',event=>{
  event.preventDefault();if(event.target.id==='peer-join-form'){copied=false;void action('peerJoin',peerDrafts.pairCode);return;}if(event.target.id==='peer-offer-form'){copied=false;void action('peerAccept',peerDrafts.offer);return;}if(event.target.id==='peer-answer-form'){void action('peerAnswer',peerDrafts.answer);return;}// Fixed views show their address read-only; Enter must not navigate them.
  if(event.target.id==='address-form'&&event.target.classList.contains('readonly'))return;
  if(event.target.id==='address-form'){let url=document.querySelector('#address').value.trim();if(!/^\w+:/.test(url))url='https://'+url;void action('navigate',url);}
  if(event.target.id==='bookmark-form'){const data=Object.fromEntries(new FormData(event.target));data.id=editingBookmark?.id;data.group=groupKey(data.group);editingBookmark=null;void action('bookmark',data);}
});
installTabDrag({
 horizontal:()=>state.layout==='horizontal',
 drop:(id,before)=>{renderDeferred=false;void action('move',{id,before});},
 cancel:()=>{if(renderDeferred){renderDeferred=false;render();}},
 over:tabOverBookmarks,
 dropElsewhere:id=>{clearBookmarkDrop();renderDeferred=false;void action('bookmarkTab',{id,before:tabBookmarkBefore});},
});
api.onState(next=>{state=next;render();if(!tutorialStarted){tutorialStarted=true;void loadVersion();if(!state.tutorialDone&&state.localHost)setTimeout(()=>{if(!state.tutorialDone&&!tutorialOpen)void openTutorial();},1500);}});
// Title bar behavior for the frameless window: double-click toggles maximise;
// maximising by any means (snap, keyboard) updates the caption button.
document.addEventListener('dblclick',event=>{if(getComputedStyle(event.target).getPropertyValue('--wails-draggable').trim()==='drag')void Window.ToggleMaximise().then(syncMaximised);});
window.addEventListener('resize',()=>void syncMaximised());
// Wails resets the caption buttons to the OS mode on a system theme change;
// send the colors again once it has.
lightScheme.addEventListener('change',()=>{if(state)applyTheme();setTimeout(()=>{if(state){windowTheme='';applyTheme();}},300);});
document.querySelector('#host-settings')?.addEventListener('load',()=>{if(state)applyTheme();});
api.onFocusAddress(()=>{const input=document.querySelector('#address');input?.focus();input?.select();});
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

