// Runtime tools for the isolated Steam client. Managed calls run on Unity's update thread.
'use strict';
const runtimeQueue=[];let runtimeApi=null;
function setupRuntime(mod){
 const A=(n,r,a)=>new NativeFunction(mod.getExportByName(n),r,a), keep=[];
 const domain=A('il2cpp_domain_get','pointer',[]), assemblies=A('il2cpp_domain_get_assemblies','pointer',['pointer','pointer']);
 const image=A('il2cpp_assembly_get_image','pointer',['pointer']),find=A('il2cpp_class_from_name','pointer',['pointer','pointer','pointer']);
 const methodFind=A('il2cpp_class_get_method_from_name','pointer',['pointer','pointer','int']);
 const parent=A('il2cpp_class_get_parent','pointer',['pointer']),fieldFind=A('il2cpp_class_get_field_from_name','pointer',['pointer','pointer']);
 const fieldGet=A('il2cpp_field_get_value','void',['pointer','pointer','pointer']),staticGet=A('il2cpp_field_static_get_value','void',['pointer','pointer']);
 const objectClass=A('il2cpp_object_get_class','pointer',['pointer']),invoke=A('il2cpp_runtime_invoke','pointer',['pointer','pointer','pointer','pointer']);
 const unbox=A('il2cpp_object_unbox','pointer',['pointer']),strNew=A('il2cpp_string_new','pointer',['pointer']),strLen=A('il2cpp_string_length','int',['pointer']),strChars=A('il2cpp_string_chars','pointer',['pointer']);
 function utf(s){return Memory.allocUtf8String(s)}
 function str(s){return strNew(utf(s))}
 function text(p){return p.isNull()?'':strChars(p).readUtf16String(strLen(p))}
 function cls(ns,n){const count=Memory.alloc(8);count.writeU64(0);const arr=assemblies(domain(),count);for(let i=0;i<count.readU64().toNumber();i++){const k=find(image(arr.add(i*8).readPointer()),utf(ns),utf(n));if(!k.isNull())return k;}throw Error('Missing class '+ns+'.'+n)}
 function meth(k,n,a){for(let t=k;!t.isNull();t=parent(t)){const p=methodFind(t,utf(n),a);if(!p.isNull())return p;}throw Error('Missing method '+n)}
 function field(k,n){for(let t=k;!t.isNull();t=parent(t)){const p=fieldFind(t,utf(n));if(!p.isNull())return p;}throw Error('Missing field '+n)}
 function get(obj,n){const out=Memory.alloc(8);fieldGet(obj,field(objectClass(obj),n),out);return out.readPointer()}
 function call(mi,obj,args=[]){const pp=Memory.alloc(Math.max(8,args.length*8));args.forEach((v,i)=>pp.add(i*8).writePointer(v));const ex=Memory.alloc(8);ex.writePointer(ptr(0));const result=invoke(mi,obj,pp,ex);if(!ex.readPointer().isNull())throw Error('Managed call failed');return result;}
 function singleton(ns,n){const k=cls(ns,n),out=Memory.alloc(8);staticGet(field(k,'_Instance'),out);const obj=out.readPointer();if(obj.isNull())throw Error('Enter the game scene first');return obj}
 function gm(){return singleton('Inutan','GMSystem')}
 const gmClass=cls('Inutan','GMSystem');
 const fields=A('il2cpp_class_get_fields','pointer',['pointer','pointer']),fieldName=A('il2cpp_field_get_name','pointer',['pointer']),fieldType=A('il2cpp_field_get_type','pointer',['pointer']),typeName=A('il2cpp_type_get_name','pointer',['pointer']);
 function commands(){const model=get(gm(),'_GmModel'),dict=get(model,'_GMCommands');if(dict.isNull())return [];
  const dk=objectClass(dict),count=unbox(call(meth(dk,'get_Count',0),dict)).readS32();const keys=call(meth(dk,'get_Keys',0),dict);
  const arr=A('il2cpp_array_new','pointer',['pointer','uint64'])(cls('System','String'),count);const zero=Memory.alloc(4);zero.writeS32(0);
  call(meth(objectClass(keys),'CopyTo',2),keys,[arr,zero]);const result=[];
  for(let i=0;i<count;i++){const key=arr.add(32+i*8).readPointer(),v=call(meth(dk,'get_Item',1),dict,[key]),item={command:text(key)};
    const it=Memory.alloc(8);it.writePointer(ptr(0));let f;while(!(f=fields(objectClass(v),it)).isNull()){const name=fieldName(f).readUtf8String(),type=typeName(fieldType(f)).readUtf8String(),o=Memory.alloc(8);if(type==='System.String'){fieldGet(v,f,o);item[name]=text(o.readPointer())}else if(type==='System.Boolean'){fieldGet(v,f,o);item[name]=!!o.readU8()}}
    result.push(item);
  }return result.sort((a,b)=>a.command.localeCompare(b.command));
 }
 function execute(command){if(typeof command!=='string'||!command.trim()||command.length>2048)throw Error('Invalid command');const obj=gm();call(meth(gmClass,'Execution',2),obj,[str(command),ptr(0)]);return {submitted:true,command};}
 function enterGame(){const o=singleton('Inutan','UILoginSystem');call(meth(objectClass(o),'OnLoginGateWay',0),o);return {submitted:true};}
 function openGM(){const obj=singleton('','UIMainSystem');call(meth(objectClass(obj),'OnGMToolButton',1),obj,[ptr(0)]);return {submitted:true,message:'Native GM entry invoked; this client may not expose a panel. Use the companion command list.'};}
 function speed(value){value=Number(value);if(!Number.isFinite(value)||value<0.1||value>5)throw Error('speed must be 0.1..5');const p=Memory.alloc(4);p.writeFloat(value);call(meth(cls('UnityEngine','Time'),'set_timeScale',1),ptr(0),[p]);return {timeScale:unbox(call(meth(cls('UnityEngine','Time'),'get_timeScale',0),ptr(0))).readFloat()}}
 // Only suppress the game's screen overlay. Unity's Player.log remains enabled.
 let overlayClosed=false;
 const outLogClass=cls('','OutLog');
 for(const name of ['SetEnabled','SetTextUIShow']){
  Interceptor.attach(meth(outLogClass,name,1).readPointer(),{onEnter(args){args[1]=ptr(0);}});
 }
 function hideErrors(){
  const obj=singleton('','OutLog'),flag=Memory.alloc(1);flag.writeU8(0);
  call(meth(outLogClass,'SetTextUIShow',1),obj,[flag]);
  call(meth(outLogClass,'SetEnabled',1),obj,[flag]);
  overlayClosed=true;return {hidden:true};
 }
 function configCatalog(name){if(name==='Gacha'){const original=compatibility.originalGacha();if(original)return original;}if(!/^[A-Za-z][A-Za-z0-9]{0,63}$/.test(name))throw Error('Invalid table name');const o=call(meth(cls('Protocol',name+'ConfigureManager'),'get_ConfDatas',0),ptr(0));if(o.isNull())throw Error('Table not loaded: '+name);return JSON.parse(text(call(meth(objectClass(o),'ToString',0),o)));}
 function shopCatalog(){const o=call(meth(cls('Protocol','ShopConfigureManager'),'get_ConfDatas',0),ptr(0));if(o.isNull())throw Error('Shop config not loaded');return JSON.parse(text(call(meth(objectClass(o),'ToString',0),o)));}
 const compatibility=createLocalCompatibility({A,field,cls,meth,call,objectClass,unbox,get,singleton,str,text,config:cfg});
 const weapons=createWeaponTools({A,cls,meth,call,objectClass,unbox,field,get,singleton,str});
 const live=createAttributeTools({A,cls,meth,call,objectClass,unbox,field,text,speed});
 runtimeApi={syncWeapons(state){return {...weapons.sync(state.weapons),unlocks:weapons.syncUnlocks(state.unlocks)}},attributes:live.attributes,applyAttributes:live.applyAttributes,resetAttributes:live.resetAttributes,configCatalog,enterGame,shopCatalog,commands,execute,openGM,speed,hideErrors,status(){let ready=false;try{gm();ready=true}catch(e){}return {ready,pid:Process.id,errorOverlayHidden:overlayClosed,timeScale:live.timeScale(),...compatibility.status(),...weapons.status()}}};
 let enterRequested=false,autoEntered=false;
 Interceptor.attach(meth(cls('Inutan','UILoginSystem'),'OnSDKLoginDone',0).readPointer(),{onLeave(){if(!autoEntered)enterRequested=true;}});
 const update=meth(cls('DG.Tweening.Core','DOTweenComponent'),'Update',0).readPointer();
 Interceptor.attach(update,{onEnter(){live.tick();compatibility.tick();if(enterRequested){enterRequested=false;autoEntered=true;try{enterGame();send({event:'local-enter-requested'});}catch(e){send({event:'local-enter-error',message:String(e)});}}if(!overlayClosed){try{hideErrors()}catch(e){}}if(runtimeQueue.length){const q=runtimeQueue.shift();try{q.resolve(runtimeApi[q.action](...q.args))}catch(e){q.reject(new Error(String(e)))}}}});
 send({event:'runtime-tools-ready'});
}
function queueRuntime(action,args=[]){return new Promise((resolve,reject)=>{if(!runtimeApi){reject(Error('Runtime not ready'));return;}const q={action,args,resolve,reject};runtimeQueue.push(q);setTimeout(()=>{const i=runtimeQueue.indexOf(q);if(i>=0){runtimeQueue.splice(i,1);reject(Error('Game is not updating; return to the scene and retry'));}},10000);});}
rpc.exports={syncweapons(rows){return queueRuntime("syncWeapons",[rows])},attributes(){return queueRuntime("attributes")},applyattributes(values){return queueRuntime("applyAttributes",[values])},resetattributes(key){return queueRuntime("resetAttributes",[key])},configcatalog(name){return queueRuntime('configCatalog',[name])},entergame(){return queueRuntime('enterGame')},shopcatalog(){return queueRuntime('shopCatalog')},hideerrors(){return queueRuntime('hideErrors')},status(){return queueRuntime('status')},commands(){return queueRuntime('commands')},execute(command){return queueRuntime('execute',[command])},opengm(){return queueRuntime('openGM')},speed(value){return queueRuntime('speed',[value])}};
Process.attachModuleObserver({onAdded(mod){if(mod.name.toLowerCase()!=='gameassembly.dll')return;Interceptor.attach(mod.getExportByName('il2cpp_init'),{onLeave(){try{setupRuntime(mod)}catch(e){send({event:'runtime-tools-error',message:String(e)})}}});}});
