const PREFIX='MAYAK1.';
const MAX_AGE=10*60*1000;
const DEFAULT_STUN='stun:stun.cloudflare.com:3478';
// The pairing relay (site/worker/index.js) that parks an invitation under an
// 8-digit code for ten minutes so the two PCs need not copy the long codes.
const PAIR_RELAY='https://mayak.ich.sh/api/pair';
function iceServers(value=DEFAULT_STUN){
  if(!value)return [];
  // No TURN configuration, credentials, or implicit relay fallback.
  if(typeof value!=='string'||!/^stuns?:[a-z0-9.-]+(?::\d{1,5})?$/i.test(value))throw new Error('invalid-stun');
  return [{urls:value}];
}
function validate(value,now=Date.now()){
  if(!value||value.version!==1||!['offer','answer'].includes(value.type)||!/^[-a-f0-9]{36}$/i.test(value.id)||!Number.isFinite(value.createdAt)||now-value.createdAt>MAX_AGE||value.createdAt-now>60000)throw new Error('invalid-or-expired-code');
  if(typeof value.sdp!=='string'||value.sdp.length>65536||!value.sdp.startsWith('v=0')||!/^m=application /m.test(value.sdp)||/^m=(audio|video) /m.test(value.sdp)||!/^a=fingerprint:sha-256 /m.test(value.sdp)||/ typ relay(?: |\r?$)/m.test(value.sdp))throw new Error('invalid-direct-session');
  return value;
}
function encode(value){validate(value);return PREFIX+btoa(JSON.stringify(value)).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');}
function decode(code,now=Date.now()){
  if(typeof code!=='string'||code.length>100000)throw new Error('invalid-or-expired-code');
  code=code.trim();if(!code.startsWith(PREFIX)||!/^[A-Za-z0-9_-]+$/.test(code.slice(PREFIX.length)))throw new Error('invalid-or-expired-code');
  let value;try{value=JSON.parse(atob(code.slice(PREFIX.length).replace(/-/g,'+').replace(/_/g,'/')));}catch{throw new Error('invalid-or-expired-code');}
  return validate(value,now);
}
export {encode,decode,iceServers,DEFAULT_STUN,MAX_AGE,PAIR_RELAY};
