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

 function n32(v){const p=Memory.alloc(4);p.writeU32(v>>>0);return p}
 const slots=['hat','hair','clothes','ornament','pendTop','pendChest','pendPelvis','pendUpFace','pendDownFace','pendLeftHand','pendRightHand','pendLeftFoot','pendRightFoot'];
 function outfitRead(id,index){const c=call(meth(cls('Inutan','IPlayerSystem'),'MainPlayerGetCharacterModel',1),ptr(0),[n32(id)]),out={};if(c.isNull())throw Error('character missing');for(let i=0;i<slots.length;i++){const box=call(meth(objectClass(c),'GetFashionData',2),c,[n32(index),n32(i)]);const v=unbox(box);out[slots[i]]=v.readU32();out[slots[i]+'DyeSchemeIndex']=v.add(4).readU32()}const b=Memory.alloc(4);fieldGet(c,field(objectClass(c),'UseFashionSuitIdx'),b);return {active:b.readU32(),preset:out};}
 function outfitWrite(id,p){const mi=meth(cls('Inutan','INetworkHelper'),'ReqMessageFashionSuitUpdate',5),tp=A('il2cpp_method_get_param','pointer',['pointer','uint'])(mi,2),k=A('il2cpp_class_from_type','pointer',['pointer'])(tp),dict=A('il2cpp_object_new','pointer',['pointer'])(k),hold=A('il2cpp_gchandle_new','uint',['pointer','bool'])(dict,1);try{call(meth(k,'.ctor',0),dict);for(let i=0;i<slots.length;i++){const pair=Memory.alloc(8);pair.writeS32(p[slots[i]]||0);pair.add(4).writeS32(p[slots[i]+'DyeSchemeIndex']||0);call(meth(k,'Add',2),dict,[n32(i),pair])}const hides=Memory.alloc(2);hides.writeU8(p.outfitHideInfo?.hideOrn?1:0);hides.add(1).writeU8(p.outfitHideInfo?.hideBraid?1:0);call(mi,ptr(0),[n32(id),n32(p.presetIndex||0),dict,hides,ptr(0)]);return {submitted:true}}finally{A('il2cpp_gchandle_free','void',['uint'])(hold)}}

 function hide(id,orn,braid){const a=Memory.alloc(1),b=Memory.alloc(1);a.writeU8(orn?1:0);b.writeU8(braid?1:0);call(meth(cls('Inutan','INetworkHelper'),'ReqOutfitHideSwitch',4),ptr(0),[n32(id),b,a,ptr(0)]);return {submitted:true}}

 function net(name,args){call(meth(cls('Inutan','INetworkHelper'),name,args.length),ptr(0),args);return {submitted:true}}
 function dyeunlock(id,index){return net('RepOutfitDyeUnlockIndex',[n32(id),n32(index),ptr(0)])}
 function dyeselect(id,colorant){return net('ReqFashionDyeStuffSelect',[n32(id),n32(colorant),ptr(0)])}
 function dyestage(id,index,pos,color){let c=ptr(0),hold=0;if(color){const k=cls('Pb','PosColor');c=A('il2cpp_object_new','pointer',['pointer'])(k);hold=A('il2cpp_gchandle_new','uint',['pointer','bool'])(c,1);call(meth(k,'.ctor',0),c);for(const key of ['Pos','Red','Green','Blue'])call(meth(k,'set_'+key,1),c,[n32(key==='Pos'?pos:color[key.toLowerCase()])]);}const u=Memory.alloc(4);u.writeFloat(0);try{return net('ReqFashionColor',[n32(id),n32(index),n32(pos),u,n32(0),c,ptr(0)])}finally{if(hold)A('il2cpp_gchandle_free','void',['uint'])(hold)}}
 function dyesave(id,index,pos,save){const v=Memory.alloc(1);v.writeU8(save?1:0);return net('ReqFashionDyeMsg',[n32(id),n32(index),n32(pos),v,ptr(0)])}
 function dyeread(id,index,pos){const f=call(meth(cls('Inutan','IFashionSystem'),'GetFashionData',1),ptr(0),[n32(id)]);if(f.isNull())throw Error('fashion missing');const plans=get(f,'ColoringPlans'),scheme=call(meth(objectClass(plans),'get_Item',1),plans,[n32(index)]),colors=get(scheme,'PosColors');const state=Memory.alloc(4);fieldGet(scheme,field(objectClass(scheme),'UnlockState'),state);if(colors.isNull())return {state:state.readS32(),color:null};const color=unbox(call(meth(objectClass(colors),'get_Item',1),colors,[n32(pos)]));return {state:state.readS32(),color:{red:Math.round(color.readFloat()*255),green:Math.round(color.add(4).readFloat()*255),blue:Math.round(color.add(8).readFloat()*255)}}}

 runtimeApi={read:outfitRead,write:outfitWrite,hide,dyeunlock,dyeselect,dyestage,dyesave,dyeread};Interceptor.attach(meth(cls('DG.Tweening.Core','DOTweenComponent'),'Update',0).readPointer(),{onEnter(){if(runtimeQueue.length){const q=runtimeQueue.shift();try{q.resolve(runtimeApi[q.action](...q.args))}catch(e){q.reject(new Error(String(e)))}}}});
}
function request(action,args){return new Promise((resolve,reject)=>{const q={action,args,resolve,reject};runtimeQueue.push(q);setTimeout(()=>{const i=runtimeQueue.indexOf(q);if(i>=0){runtimeQueue.splice(i,1);reject(Error('game is not updating'))}},10000)})}
rpc.exports={dyeunlock(...args){return request('dyeunlock',args)},dyeselect(...args){return request('dyeselect',args)},dyestage(...args){return request('dyestage',args)},dyesave(...args){return request('dyesave',args)},dyeread(...args){return request('dyeread',args)},read(id,index){return request('read',[id,index])},write(id,preset){return request('write',[id,preset])},hide(id,orn,braid){return request('hide',[id,orn,braid])}};
setupRuntime(Process.getModuleByName('GameAssembly.dll'));
