// Runs in the trusted Wails shell. External websites use separate native views.
// Only navigation events and recognized item details travel over the encrypted, ordered data channel.
class MayakPeer {
  constructor({onState=()=>{},onMessage=()=>{}}={}) {this.onState=onState;this.onMessage=onMessage;this.pc=null;this.channel=null;this.role='';this.timer=null;this.verified=false;this.messages=[];}
  close(){clearTimeout(this.timer);this.timer=null;const pc=this.pc;this.pc=null;this.channel?.close();this.channel=null;pc?.close();}
  create(role,iceServers){
    this.close();this.role=role;this.verified=false;this.messages=[];
    if(!Array.isArray(iceServers)||iceServers.some(s=>typeof s.urls!=='string'||!/^stuns?:/i.test(s.urls)))throw new Error('invalid-stun');
    const pc=new RTCPeerConnection({iceServers,iceTransportPolicy:'all'});this.pc=pc;
    pc.onconnectionstatechange=()=>{if(this.pc!==pc)return;const state=pc.connectionState;if(['failed','disconnected','closed'].includes(state)){this.verified=false;this.onState({phase:state});}else if(state==='connected'&&this.channel?.readyState==='open')void this.connected(pc);};
    pc.ondatachannel=event=>{if(role!=='receiver'||this.channel){event.channel.close();return;}this.attach(event.channel,pc);};
    return pc;
  }
  attach(channel,pc){
    if(channel.label!=='mayak-display-v1'){channel.close();throw new Error('invalid-channel');}
    this.channel=channel;
    channel.onopen=()=>void this.connected(pc);
    channel.onclose=()=>{if(this.pc===pc)this.onState({phase:'disconnected'});};
    channel.onerror=()=>{if(this.pc===pc)this.onState({phase:'failed'});};
    channel.onmessage=event=>{
      if(this.pc!==pc||this.role!=='receiver'||typeof event.data!=='string'||event.data.length>65536)return;
      try{const message=JSON.parse(event.data);if(['browser:task','browser:map','browser:item'].includes(message.event)&&Array.isArray(message.args)&&message.args.length===1){if(this.verified)this.onMessage(message);else if(this.messages.length<32)this.messages.push(message);}}catch{/* Invalid or non-display messages are never passed to the application. */}
    };
  }
  async connected(pc){
    if(this.pc!==pc||this.verified)return;
    try{
      const stats=await pc.getStats();if(this.pc!==pc||this.verified)return;
      let route='direct';for(const item of stats.values()){if(item.type==='transport'&&item.selectedCandidatePairId){const pair=stats.get(item.selectedCandidatePairId);const local=stats.get(pair?.localCandidateId),remote=stats.get(pair?.remoteCandidateId);if(local?.candidateType==='relay'||remote?.candidateType==='relay'){this.close();this.onState({phase:'failed',reason:'relay-rejected'});return;}route=local?.candidateType==='host'&&remote?.candidateType==='host'?'local':'direct';}}
      clearTimeout(this.timer);this.timer=null;this.verified=true;this.onState({phase:'connected',route});
      for(const message of this.messages.splice(0))this.onMessage(message);
    }catch{if(this.pc===pc)this.onState({phase:'failed',reason:'transport-stopped'});}
  }
  async gathered(pc){
    if(pc.iceGatheringState==='complete')return;
    await new Promise((resolve,reject)=>{
      const finish=error=>{clearTimeout(timer);pc.removeEventListener('icegatheringstatechange',changed);pc.removeEventListener('connectionstatechange',closed);error?reject(error):resolve();};
      const changed=()=>{if(pc.iceGatheringState==='complete')finish();};
      const closed=()=>{if(pc.connectionState==='closed')finish(new Error('session-closed'));};
      const timer=setTimeout(finish,10000);pc.addEventListener('icegatheringstatechange',changed);pc.addEventListener('connectionstatechange',closed);
    });
    if(this.pc!==pc||pc.signalingState==='closed')throw new Error('session-closed');
  }
  watchConnection(timeout=45000){clearTimeout(this.timer);this.timer=setTimeout(()=>{if(this.channel?.readyState!=='open'){this.close();this.onState({phase:'failed',reason:'direct-unreachable'});}},timeout);}
  async offer({iceServers}){
    const pc=this.create('sender',iceServers);this.attach(pc.createDataChannel('mayak-display-v1',{ordered:true}),pc);
    await pc.setLocalDescription(await pc.createOffer());await this.gathered(pc);return pc.localDescription.sdp;
  }
  async answer({iceServers,sdp}){
    const pc=this.create('receiver',iceServers);await pc.setRemoteDescription({type:'offer',sdp});await pc.setLocalDescription(await pc.createAnswer());await this.gathered(pc);this.watchConnection(120000);return pc.localDescription.sdp;
  }
  async acceptAnswer({sdp}){if(!this.pc||this.role!=='sender'||this.pc.signalingState!=='have-local-offer')throw new Error('no-pending-invite');await this.pc.setRemoteDescription({type:'answer',sdp});this.watchConnection();}
  send(message){
    if(this.role!=='sender'||this.channel?.readyState!=='open'||!['browser:task','browser:map','browser:item'].includes(message?.event))return false;
    const data=JSON.stringify(message);if(data.length>65536||this.channel.bufferedAmount>262144){this.close();this.onState({phase:'failed',reason:'slow-peer'});return false;}
    this.channel.send(data);return true;
  }
}
globalThis.MayakPeer=MayakPeer;
