// The page is written in Japanese; every element with data-t has its
// English text here, and the toggle swaps them. The download button asks
// GitHub for the latest release so the version and size are current.
(function () {
  'use strict';

  var en = {
    navFeatures: 'Features', navHow: 'How it works', navSafety: 'Safety', navDocs: 'Docs',
    lead: 'A companion for Escape from Tarkov that reads only the screenshots and logs the game writes, and keeps maps, tasks and item info in sync in a browser. It never touches the game process.',
    downloadWindows: 'Download for Windows', allReleases: 'All releases',
    otherPlatforms: 'macOS and Linux builds run as receive-only clients and are untested.',
    featuresTitle: 'What it does',
    f1Title: 'Position on the map', f1Body: 'Reads the coordinates and heading embedded in screenshot file names and moves your marker on the tarkov.dev map. The map itself is detected from the logs.',
    f2Title: 'Task screen recognition', f2Body: 'Local OCR reads the selected task name from a Tasks screenshot and opens the matching tarkov.dev or wiki page.',
    f3Title: 'Item information', f3Body: 'Screenshot an item inspection window and the item panel shows prices, traders and the tasks and hideout stations that need it.',
    f4Title: 'TarkovTracker sync', f4Body: 'Task started, failed and completed events from the EFT notification logs go to TarkovTracker, per PvP, Season and PvE profile.',
    f5Title: 'Raid alerts', f5Body: 'Match found, raid start and the run-through timer are detected from the logs and announced with sounds of your choice.',
    f6Title: 'Automatic updates', f6Body: 'New versions are picked up from GitHub Releases, checksum-verified and installed when the app quits. Japanese and English UI.',
    howTitle: 'Three steps',
    s1Title: 'Register a Remote ID', s1Body: 'Enable Remote Control on the tarkov.dev map and enter the ID it shows. Browsers on the same PC are detected automatically.',
    s2Title: 'Pick the folders', s2Body: 'Choose the EFT Screenshots and Logs folders, or let MAYAK find them from the install location.',
    s3Title: 'Press PrintScreen in game', s3Body: 'Then just play. MAYAK notices each saved screenshot and handles it. No input is ever sent to the game.',
    safetyTitle: 'It never touches the game',
    safetyLead: 'MAYAK reads only the screenshots and logs the game saves, local settings files and public web APIs.',
    sa1: 'No process memory reads, DLL injection, hooks or packet capture',
    sa2: 'No automated keyboard or mouse input to the game',
    sa3: 'Screenshots are never uploaded; OCR and image analysis run locally',
    sa4: 'TarkovTracker API keys are stored encrypted with Windows DPAPI',
    sa5: 'The source code is published under GPL-3.0',
    safetyNote: 'Whether a companion tool is acceptable to you is still your own call. Read the game’s terms and use MAYAK at your own risk.',
    reqTitle: 'Requirements',
    req1: 'Windows 11 (needs the WebView2 Runtime, normally already installed)',
    req2: 'Read access to the Escape from Tarkov Screenshots and Logs folders',
    req3: 'OCR runs on the bundled Tesseract. Windows OCR needs the English language pack',
    req4: 'There is no installer: unzip and start Mayak.exe',
    footerCredit: 'Game data comes from the <a href="https://tarkov.dev/" rel="noopener">tarkov.dev</a> API and progress from <a href="https://tarkovtracker.org/" rel="noopener">TarkovTracker</a>. Escape from Tarkov is a trademark of Battlestate Games. MAYAK is an independent project, not affiliated with Battlestate Games, tarkov.dev or TarkovTracker.',
    footerDocs: 'Docs'
  };

  var ja = {};
  var nodes = document.querySelectorAll('[data-t]');
  nodes.forEach(function (node) { ja[node.dataset.t] = node.innerHTML; });

  function apply(lang) {
    var dict = lang === 'en' ? en : ja;
    nodes.forEach(function (node) {
      var text = dict[node.dataset.t];
      if (text !== undefined) node.innerHTML = text;
    });
    document.documentElement.lang = lang;
    document.documentElement.dataset.lang = lang;
    document.getElementById('lang-toggle').textContent = lang === 'en' ? '日本語' : 'EN';
    try { localStorage.setItem('mayak-lang', lang); } catch (e) { /* private mode */ }
  }

  var initial = 'ja';
  try { initial = localStorage.getItem('mayak-lang') || initial; } catch (e) { /* private mode */ }
  if (initial !== 'en' && initial !== 'ja') initial = 'ja';
  if (!localStorageHas() && !/^ja\b/.test(navigator.language || '')) initial = 'en';
  if (initial === 'en') apply('en');

  function localStorageHas() {
    try { return !!localStorage.getItem('mayak-lang'); } catch (e) { return false; }
  }

  document.getElementById('lang-toggle').addEventListener('click', function () {
    apply(document.documentElement.dataset.lang === 'en' ? 'ja' : 'en');
  });

  // Latest release: version and the Windows archive.
  var button = document.getElementById('download');
  var meta = document.getElementById('download-meta');
  fetch('https://api.github.com/repos/ichi0g0y/mayak/releases/latest', { headers: { Accept: 'application/vnd.github+json' } })
    .then(function (r) { return r.ok ? r.json() : null; })
    .then(function (release) {
      if (!release || !release.assets) return;
      var asset = release.assets.find(function (a) { return a.name === 'Mayak-windows-amd64.zip'; });
      if (!asset) return;
      button.href = asset.browser_download_url;
      meta.textContent = release.tag_name + ' · ' + (asset.size / 1048576).toFixed(0) + ' MB';
    })
    .catch(function () { /* The button keeps pointing at the releases page. */ });
})();
