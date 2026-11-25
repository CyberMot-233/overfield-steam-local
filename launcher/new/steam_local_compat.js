// Compatibility for the isolated Steam client. All work runs on Unity's update thread.
// Uses IL2CPP metadata, never version-dependent code offsets.
'use strict';
function createLocalCompatibility({A,field,cls,meth,call,objectClass,unbox,get,singleton,str,text,config}) {
 let nextCheck=0,registered=false,applied=false,lastError='',checkedNetwork='',originalGacha=null;
 const fieldSet=A('il2cpp_field_set_value','void',['pointer','pointer','pointer']);
 const errors=[],seenPools=new Set(),poolOrder=new Map((config.localGacha?.poolIds||[]).map((id,index)=>[id,index+1]));
 function i32(value){const p=Memory.alloc(4);p.writeS32(value);return p;}
 function eventState(){
  const player=singleton('Inutan','PlayerSystem');
  const network=singleton('Inutan','NetworkSystem');
  const events=get(network,'EventNew');if(events.isNull())return null;
  const dict=get(events,'eventDictionary');if(dict.isNull())return null;
  const k=objectClass(dict),has=id=>!!unbox(call(meth(k,'ContainsKey',1),dict,[i32(id)])).readU8();
  return {player,events,has};
 }
 function repairEvents(){
  const state=eventState();if(!state)return;
  const identity=state.events.toString()+':'+state.player.toString();
  if(registered&&checkedNetwork===identity)return;
  // PlayerSystem.OnInit can precede NetworkSystem.OnInit and skip all bindings.
  // Only retry the original registration when all three wardrobe handlers are absent.
  if(!state.has(1650)&&!state.has(1658)&&!state.has(1660)){
   call(meth(objectClass(state.player),'AddEventHandler',0),state.player);
   if(!state.has(1658)||!state.has(1660))throw Error('Player event registration did not finish');
   send({event:'local-player-events-repaired'});
  }
  registered=state.has(1658)&&state.has(1660);checkedNetwork=identity;
 }
 function applyRow(row){
  const k=objectClass(row),id=unbox(call(meth(k,'get_ID',0),row)).readS32();
  if(!poolOrder.has(id))return;
  call(meth(k,'set_Index',1),row,[i32(poolOrder.get(id))]);
  call(meth(k,'set_OpeningTime',1),row,[str(config.localGacha.openingTime)]);
  call(meth(k,'set_ClosingTime',1),row,[str(config.localGacha.closingTime)]);
 }
 let startTime=null,endTime=null;
 function applyModels(list){
  if(!config.localGacha||list.isNull())return;
  if(startTime===null){
   const parse=meth(cls('System','DateTime'),'Parse',1);
   function timeValue(value){const b=Memory.alloc(8);b.writeU64(unbox(call(parse,ptr(0),[str(value)])).readU64());return b;}
   startTime=timeValue(config.localGacha.openingTime);endTime=timeValue(config.localGacha.closingTime);
  }
  const k=objectClass(list),count=unbox(call(meth(k,'get_Count',0),list)).readS32();
  for(let i=0;i<count;i++){
   const row=call(meth(k,'get_Item',1),list,[i32(i)]),id=get(row,'GachaID').toInt32();
   if(!poolOrder.has(id)){const type=objectClass(row);fieldSet(row,field(type,'EndTime'),startTime);call(meth(type,'CheckState',0),row);continue;}
   applyRow(get(row,'Config'));
   const type=objectClass(row);fieldSet(row,field(type,'StartTime'),startTime);fieldSet(row,field(type,'EndTime'),endTime);
   call(meth(type,'CheckState',0),row);seenPools.add(id);
  }
 }
 function refreshModels(){if(!config.localGacha||!registered||seenPools.size===poolOrder.size)return;const o=singleton('Inutan','GachaSystem');const list=call(meth(objectClass(o),'GetGachaDatas',0),o);applyModels(list);}
 function applyGacha(){
  if(applied||!config.localGacha)return;
  const c=config.localGacha,manager=cls('Protocol','GachaConfigureManager');
  // Wait for the installed resource tables before touching configuration objects.
  const table=call(meth(manager,'get_ConfDatas',0),ptr(0));if(table.isNull())return;
  if(originalGacha===null)originalGacha=text(call(meth(objectClass(table),'ToString',0),table));
  const find=meth(manager,'FindInfo',2),rows=[];
  for(const id of c.poolIds){const row=call(find,ptr(0),[i32(id),i32(0)]);if(row.isNull())throw Error('Local pool missing from installed client: '+id);rows.push(row);}
  for(const row of rows)applyRow(row);
  applied=true;send({event:'local-gacha-ready',mode:c.mode,count:rows.length,resourceVersion:c.resourceVersion});
 }
 function tick(){
  const now=Date.now();if(now<nextCheck)return;nextCheck=now+1000;
  for(const fn of [repairEvents,applyGacha,refreshModels])try{fn();}catch(e){
   const message=String(e);
   if(message.includes('Enter the game scene first'))continue;
   if(message!==lastError){lastError=message;errors.push(message);if(errors.length>5)errors.shift();send({event:'local-compatibility-error',message});}
  }
 }
 if(config.localGacha){
  const label=str('单机常驻卡池 · 本地配置');
  A('il2cpp_gchandle_new','uint',['pointer','bool'])(label,0);
  Interceptor.attach(meth(cls('Inutan','UIGachaSystem'),'OnClickPage',1).readPointer(),{onEnter(a){this.self=a[0];},onLeave(){
   try{const view=get(this.self,'_ViewAPI');if(!view.isNull())call(meth(objectClass(view),'Set_Gacha_EndTime',1),view,[label]);}
   catch(e){send({event:'local-gacha-label-error',message:String(e)});}
  }});
  Interceptor.attach(meth(cls('Inutan','GachaSystem'),'GetGachaDatas',0).readPointer(),{onLeave(list){try{applyModels(list);}catch(e){send({event:'local-gacha-list-error',message:String(e)});}}});
  // GachaData caches DateTime values and may hold a different config instance.
  // Update its exact constructor argument before those values are parsed.
  Interceptor.attach(meth(cls('Inutan','GachaData'),'.ctor',1).readPointer(),{onEnter(a){
   try{applyRow(a[1]);const id=unbox(call(meth(objectClass(a[1]),'get_ID',0),a[1])).readS32();if(poolOrder.has(id))seenPools.add(id);}
   catch(e){send({event:'local-gacha-model-error',message:String(e)});}
  }});
  Interceptor.attach(meth(cls('Inutan','GachaData'),'IsShow',0).readPointer(),{onEnter(a){this.allowed=poolOrder.has(get(a[0],'GachaID').toInt32());},onLeave(result){if(!this.allowed)result.replace(ptr(0));}});
 }
 Interceptor.attach(meth(cls('Inutan','GachaSystem'),'Init',1).readPointer(),{onEnter(){try{applyGacha();}catch(e){send({event:'local-gacha-init-error',message:String(e)});}}});
 return {tick,originalGacha(){return originalGacha===null?null:JSON.parse(originalGacha);},status(){return {playerEventsReady:registered,localGachaMode:config.localGacha?.mode||'client-schedule',localGachaReady:applied&&seenPools.size===poolOrder.size,localGachaPools:seenPools.size,compatibilityErrors:errors.slice()}}};
}

// Persisted weapon targets come only from the authenticated local server.
// Preserve the existing object so equipment/UI references stay valid.
function createWeaponTools({A,cls,meth,call,objectClass,unbox,field,get,singleton,str}) {
 const set=A('il2cpp_field_set_value','void',['pointer','pointer','pointer']);
 const read=A('il2cpp_field_get_value','void',['pointer','pointer','pointer']);
 const getObject=A('il2cpp_field_get_value_object','pointer',['pointer','pointer']);
 const setObject=A('il2cpp_field_set_value_object','void',['pointer','pointer','pointer']);
 const targets=new Map(),applied=new Map();let lastError='';
 function i64(value){const p=Memory.alloc(8);p.writeS64(value);return p;}
 function i32(value){const p=Memory.alloc(4);p.writeS32(value);return p;}
 function integer(o,name){const p=Memory.alloc(8);read(o,field(objectClass(o),name),p);return name==='UUID'?p.readS64().toNumber():p.readS32();}
 function override(o){
  const row=targets.get(integer(o,'UUID'));if(!row||integer(o,'ItemID')!==row.weaponId)return;
  const values=get(o,'RandomBaseValueProperties');if(values.isNull())return;
  for(const [key,value] of Object.entries(row.stats||{})){
   const property={attack:2,criticalRatio:7,damageBalance:4}[key];if(!property)continue;
   const b=Memory.alloc(4);b.writeFloat(key==='attack'?value:value/100);
   call(meth(objectClass(values),'set_Item',2),values,[i32(property),b]);
   const data=get(o,'EquipPropertyData');if(!data.isNull())set(data,field(objectClass(data),{attack:'Attack',criticalRatio:'CriticalRatio',damageBalance:'DamageBalance'}[key]),b);
  }
 }
 const createClass=cls('Inutan','ItemUtils'),it=Memory.alloc(8);it.writePointer(ptr(0));let creator=ptr(0),method;
 while(!(method=A('il2cpp_class_get_methods','pointer',['pointer','pointer'])(createClass,it)).isNull()){
  if(A('il2cpp_method_get_name','pointer',['pointer'])(method).readUtf8String()!=='CreateItemModel'||A('il2cpp_method_get_param_count','uint',['pointer'])(method)!==1)continue;
  const type=A('il2cpp_type_get_name','pointer',['pointer'])(A('il2cpp_method_get_param','pointer',['pointer','uint'])(method,0)).readUtf8String();if(type==='Pb.WeaponInstance'){creator=method;break;}
 }
 if(creator.isNull())throw Error('Weapon decoder unavailable');
 const copyFields=['PropertyGroup','StrengthenConfigure','Level','RandomBaseValueProperties','RandomPropertyEntries','Score','StrengthLevel','StrengthExp','StrengthMaxLevel','MaxLevel','EquipPropertyData','StrengthPropertyData','PassiveIDs','EquipUIPropertyData'];
 Interceptor.attach(meth(cls('Inutan','WeaponItemModel'),'UpdateProperty',0).readPointer(),{onEnter(args){this.weapon=args[0];},onLeave(){try{override(this.weapon);}catch(e){lastError=String(e);}}});
 function sync(rows){
  if(!Array.isArray(rows)||rows.length>2000)throw Error('Invalid local weapon list');
  lastError='';let count=0,pending=0;const present=new Set();
  for(const row of rows){
   if(!Number.isSafeInteger(row.instanceId)||!row.data||row.data.instanceId!==row.instanceId||row.data.weaponId!==row.weaponId)throw Error('Invalid local weapon identity');
   present.add(row.instanceId);targets.set(row.instanceId,row);
   const bags=singleton('Inutan','BagsSystem'),old=call(meth(objectClass(bags),'GetWeapon',1),bags,[i64(row.instanceId)]);
   if(old.isNull()||integer(old,'ItemID')!==row.weaponId){pending++;continue;}
   const signature=row.revision+':'+old.toString();if(applied.get(row.instanceId)===signature)continue;
   const parser=call(meth(cls('Pb','WeaponInstance'),'get_Parser',0),ptr(0));
   const packet=call(meth(objectClass(parser),'ParseJson',1),parser,[str(JSON.stringify(row.data))]);
   const packetHandle=A('il2cpp_gchandle_new','uint',['pointer','bool'])(packet,0);
   let freshHandle=0;
   try{
    const fresh=call(creator,ptr(0),[packet]);freshHandle=A('il2cpp_gchandle_new','uint',['pointer','bool'])(fresh,0);
    for(const name of copyFields){const sourceField=field(objectClass(fresh),name),destField=field(objectClass(old),name),valueClass=A('il2cpp_class_from_type','pointer',['pointer'])(A('il2cpp_field_get_type','pointer',['pointer'])(sourceField));
     if(A('il2cpp_class_is_valuetype','bool',['pointer'])(valueClass)){const buffer=Memory.alloc(16);read(fresh,sourceField,buffer);set(old,destField,buffer);}
     else setObject(old,destField,getObject(sourceField,fresh));
    }
    override(old);applied.set(row.instanceId,signature);count++;
   }finally{if(freshHandle)A('il2cpp_gchandle_free','void',['uint'])(freshHandle);A('il2cpp_gchandle_free','void',['uint'])(packetHandle);}
  }
  for(const id of targets.keys())if(!present.has(id)){targets.delete(id);applied.delete(id);}
  if(count){try{const ui=singleton('Inutan','UIBagSystem'),flag=Memory.alloc(4);read(ui,field(objectClass(ui),'IsActive'),flag);if(flag.readU8())call(meth(objectClass(ui),'ShowPage',1),ui,[i32(0)]);}catch(e){lastError=String(e);}}
  return {updated:count,pending,persistedTargets:rows.length,lastError};
 }

 let unlockSignature='';
 function syncUnlocks(data){
  if(!data||(!data.actions?.length&&!data.abilities?.length))return {actions:0,abilities:0};
  const player=call(meth(cls('Inutan','IPlayerSystem'),'get_MainPlayerModel',0),ptr(0));if(player.isNull())throw Error('Enter the game scene first');
  const signature=player.toString()+':'+JSON.stringify(data);
  if(signature===unlockSignature)return {actions:data.actions.length,abilities:data.abilities.length};
  const actions=get(player,'ExpressionActionIDs');if(actions.isNull())throw Error('Action model not ready');
  for(const id of data.actions)call(meth(objectClass(actions),'Add',1),actions,[i32(id)]);
  if(data.abilities.length){
   const system=singleton('Inutan','AbilitySystem'),model=get(system,'_AbilityModel');if(model.isNull())throw Error('Ability model not ready');
   for(const ability of data.abilities)call(meth(objectClass(model),'SetAbilityLevel',2),model,[i32(ability.ability_id||ability.abilityId),i32(ability.level)]);
   call(meth(objectClass(player),'InitPlayerAbility',0),player);
  }
  unlockSignature=signature;return {actions:data.actions.length,abilities:data.abilities.length};
 }
 return {syncUnlocks,sync,status(){return {weaponTargets:targets.size,weaponSyncError:lastError};}};
}
