'use strict';
let weaponData=null, weaponSelected=null, weaponBusy=false, weaponUID=null;
const weaponFields={level:'weaponLevel',attack:'weaponAttack',criticalRatio:'weaponCriticalRatio',damageBalance:'weaponDamageBalance',strengthLevel:'weaponStrengthLevel'};
function weaponDirty(){return !!weaponSelected&&Object.entries(weaponFields).some(([key,id])=>el(id).value===''||Number(el(id).value)!==weaponSelected[key]);}
function weaponMessage(text,error=false){el('weaponMessage').textContent=text;el('weaponMessage').className=error?'error':'muted';}
function weaponControls(disabled){el('weapons').querySelectorAll('button,input,select').forEach(x=>x.disabled=disabled);}
async function loadWeapons(force=true){
 if(weaponBusy||busy)return;
 const uid=num('uid');
 if(!force&&weaponUID===uid&&weaponData)return;
 const dirty=weaponUID===uid&&weaponDirty(), selected=weaponSelected;
 weaponBusy=true;weaponControls(true);el('weaponState').textContent='正在读取…';
 try{
  const data=await api('/api/weapons?uid='+uid);
  if(num('uid')!==uid)return;
  weaponData=data;weaponUID=uid;
  el('weaponState').textContent=data.online?'已读取游戏内武器':'已读取离线存档';
  filterWeapons();
  if(dirty){weaponSelected=selected;weaponMessage('列表已更新，未保存的输入已保留。保存时会检查这把武器是否已在游戏中变化。');}
  else if(selected&&data.weapons.some(w=>w.instanceId===selected.instanceId)){selectWeapon(selected.instanceId,true);}
  else {weaponSelected=null;el('weaponEditor').hidden=true;}
 }catch(e){el('weaponState').textContent='读取失败';notice(e.message,true);}
 finally{weaponBusy=false;weaponControls(false);}
}
function filterWeapons(){
 const list=weaponData?.weapons||[], q=val('weaponSearch').trim().toLowerCase();
 const rows=list.filter(w=>[w.name,w.weaponId,w.instanceId].some(v=>String(v).toLowerCase().includes(q)));
 el('weaponRows').innerHTML=rows.map(w=>'<tr><td>'+esc(w.name)+'<br><small>武器 '+w.weaponId+' · 实例 #'+w.instanceId+'</small></td><td>Lv.'+w.level+' / +'+w.strengthLevel+'</td><td>'+esc(w.wearerName||'未穿戴')+(w.isLock?' · 已锁定':'')+'</td><td><button data-weapon="'+w.instanceId+'">编辑</button></td></tr>').join('');
 el('weaponTotal').textContent='共 '+list.length+' 把武器，当前匹配 '+rows.length+' 把。同名武器按实例编号区分。';
 el('weaponRows').querySelectorAll('[data-weapon]').forEach(b=>b.onclick=()=>selectWeapon(Number(b.dataset.weapon)));
}
function selectWeapon(instanceId,force=false){
 if(weaponUID!==num('uid')){notice('账号已改变，请先读取该账号的武器。',true);return;}
 if(!force&&weaponDirty()&&!confirm('当前武器有未保存输入，放弃输入并选择这把武器？'))return;
 weaponSelected=weaponData.weapons.find(w=>w.instanceId===instanceId);
 if(!weaponSelected)return;
 el('weaponEditor').hidden=false;
 el('weaponTitle').textContent=weaponSelected.name;
 el('weaponIdentity').textContent='武器 ID '+weaponSelected.weaponId+' · 实例 #'+weaponSelected.instanceId+' · '+(weaponSelected.wearerName?'穿戴者：'+weaponSelected.wearerName:'未穿戴');
 resetWeaponInputs();
}
function resetWeaponInputs(){
 if(!weaponSelected)return;
 const latest=weaponData.weapons.find(w=>w.instanceId===weaponSelected.instanceId);
 if(latest)weaponSelected=latest;
 const w=weaponSelected;
 for(const [k,id] of Object.entries(weaponFields)){if(k!=='strengthLevel')el(id).value=w[k];}
 el('weaponAttack').max=w.maxAttack;el('weaponCriticalRatio').max=w.maxPercent;el('weaponDamageBalance').max=w.maxPercent;
 const ranges=w.ranges;
 el('weaponLevel').min=ranges.length?Math.min(...ranges.map(r=>r.min)):1;
 el('weaponLevel').max=ranges.length?Math.max(...ranges.map(r=>r.max)):0;
 updateWeaponRange(w.strengthLevel);weaponMessage('已读取保存值。修改输入后点击「保存这把武器」。');
}
function updateWeaponRange(preferred){
 if(!weaponSelected)return;
 const lv=num('weaponLevel'),r=weaponSelected.ranges.find(x=>lv>=x.min&&lv<=x.max);
 const strength=typeof preferred==='number'?preferred:(val('weaponStrengthLevel')===''?weaponSelected.strengthLevel:num('weaponStrengthLevel'));
 el('weaponLevel').setCustomValidity(r&&Number.isInteger(lv)?'':'此等级没有对应的武器配置');
 const allowed=r?.strengthLevels||[];
 el('weaponStrengthLevel').innerHTML=allowed.map(n=>'<option value="'+n+'">+'+n+'</option>').join('');
 if(allowed.includes(strength))el('weaponStrengthLevel').value=strength;
 else if(allowed.length){el('weaponStrengthLevel').value=allowed[allowed.length-1];}
 el('weaponRange').textContent='等级范围：'+weaponSelected.ranges.map(r=>r.min+'–'+r.max).join('、')+'。当前等级可用强化：'+(allowed.map(n=>'+'+n).join('、')||'无');
}
el('weaponForm').addEventListener('submit',async event=>{
 event.preventDefault();if(weaponBusy||busy||!weaponSelected)return;
 if(weaponUID!==num('uid')){weaponMessage('账号已改变，请重新读取武器。',true);return;}
 if(!el('weaponForm').reportValidity())return;
 const changes={};for(const [k,id] of Object.entries(weaponFields)){const n=Number(el(id).value);if(el(id).value===''||!Number.isFinite(n)||n<0||(['level','attack','strengthLevel'].includes(k)&&!Number.isSafeInteger(n))){weaponMessage('等级和攻击力填非负整数，百分比最多两位小数。',true);return;}if(n!==weaponSelected[k])changes[k]=n;}
 if(!Object.keys(changes).length){weaponMessage('数值没有改变，无需保存。');return;}
 const selected=weaponSelected,uid=weaponUID;
 weaponBusy=true;weaponControls(true);weaponMessage('正在备份并保存这把武器…');
 try{
  const data=await api('/api/action',{action:'weapon-edit',uid,instanceId:selected.instanceId,weaponId:selected.weaponId,revision:selected.revision,changes});
  const result=data.result;
  if(!result.saved)throw Error('服务端未确认保存，请重新读取实际值。');
  if(weaponUID!==uid||num('uid')!==uid){notice('账号 '+uid+' 的武器 #'+selected.instanceId+' 已保存。');return;}
  const updated={...selected,...result.weapon};weaponData.weapons=weaponData.weapons.map(w=>w.instanceId===selected.instanceId?updated:w);weaponSelected=updated;
  filterWeapons();resetWeaponInputs();weaponMessage(result.message+' 操作前备份：'+result.backup);
 }catch(e){weaponMessage('保存未确认：'+e.message+'。输入已保留，可读取最新值后核对。',true);}
 finally{weaponBusy=false;weaponControls(false);}
});

async function resetWeaponStats(){
 if(weaponBusy||busy||!weaponSelected)return;
 if(weaponUID!==num('uid')){weaponMessage('账号已改变，请重新读取武器。',true);return;}
 const selected=weaponSelected,uid=weaponUID;
 weaponBusy=true;weaponControls(true);weaponMessage('正在恢复配置计算属性…');
 try{
  const data=await api('/api/action',{action:'weapon-edit',uid,instanceId:selected.instanceId,weaponId:selected.weaponId,revision:selected.revision,changes:{resetStats:1}});
  const result=data.result;if(!result.saved)throw Error('服务端未确认保存');
  if(weaponUID!==uid||num('uid')!==uid)return;
  const updated={...selected,...result.weapon};weaponData.weapons=weaponData.weapons.map(w=>w.instanceId===selected.instanceId?updated:w);weaponSelected=updated;
  filterWeapons();resetWeaponInputs();weaponMessage('已清除攻击、暴击和平衡性的自定义值，等级及强化保留。'+result.message);
 }catch(e){weaponMessage('恢复未确认：'+e.message,true);}
 finally{weaponBusy=false;weaponControls(false);}
}
