// Real-time tools for the isolated, fingerprinted Steam client only.
// Values are returned only for the current local leader's property object.
function createAttributeTools(ctx) {
 const {A,cls,meth,call,objectClass,unbox,field,text,speed}=ctx;
 const mainMI=meth(cls('Inutan','IPlayerSystem'),'get_MainCharacterUnit',0);
 const nativeMain=new NativeFunction(mainMI.readPointer(),'pointer',['pointer']);
 const offset=A('il2cpp_field_get_offset','int',['pointer'])(field(cls('','Unit'),'_Property'));
 const typeName=A('il2cpp_type_get_name','pointer',['pointer']);
 const returnType=A('il2cpp_method_get_return_type','pointer',['pointer']);
 const propertyClass=cls('','UnitPropertyGetter');
 const definitions={
  moveSpeed:{label:'移动速度',min:0.1,max:30,step:0.1,getters:['MoveSpeed']},
  attack:{label:'攻击力',min:0,max:10000000,step:100,getters:['AttackMin','AttackMax']},
  defense:{label:'防御力',min:0,max:1000000,step:100,getters:['Defense']},
  maxHp:{label:'最大生命',min:1,max:100000000,step:1000,getters:['MaxHp']},
  jumpSpeed:{label:'跳跃速度',min:0,max:50,step:0.5,getters:['JumpSpeed']},
  criticalChance:{label:'暴击率（0–1）',min:0,max:1,step:0.01,getters:['CriticalRatio']},
  criticalDamage:{label:'暴击伤害倍率',min:1,max:20,step:0.1,getters:['CriticalDamagePercent']}
 };
 const overrides={},readers={},callbacks=[];
 function unit(){const u=nativeMain(mainMI);if(u.isNull())throw Error('请先进入游戏场景');return u;}
 function property(){const p=unit().add(offset).readPointer();if(p.isNull())throw Error('角色属性尚未就绪');return p;}
 for(const [key,def] of Object.entries(definitions)){
  readers[key]=[];
  for(const name of def.getters){
   const mi=meth(propertyClass,'get_'+name,0);
   if(typeName(returnType(mi)).readUtf8String()!=='System.Single')throw Error('Unsupported property signature: '+name);
   // Native replacements also apply during managed readback on the update thread.
   // A trampoline retains the unmodified calculation for the default column.
   const slot=Memory.alloc(32);slot.writeByteArray(new Uint8Array(32));
   const module=new CModule(
    '#include <stdint.h>\n' +
    'extern unsigned char state[]; extern unsigned char main_info[];\n' +
    'extern void *get_main(void *method);\n' +
    'float replacement(void *self, void *method) {\n' +
    '  if (*(uint32_t *)state) {\n' +
    '    unsigned char *u=(unsigned char *)get_main(main_info);\n' +
    '    if (u && *(void **)(u+'+offset+')==self) return *(float *)(state+4);\n' +
    '  }\n' +
    '  return (*(float (**)(void *, void *))(state+8))(self,method);\n' +
    '}\n', {state:slot,main_info:mainMI,get_main:mainMI.readPointer()});
   const originalAddress=Interceptor.replaceFast(mi.readPointer(),module.replacement);
   slot.add(8).writePointer(originalAddress);
   const original=new NativeFunction(originalAddress,'float',['pointer','pointer']);
   callbacks.push(module);readers[key].push({mi,original,slot});
  }
 }
 Interceptor.flush();
 function read(p,name){return unbox(call(meth(propertyClass,'get_'+name,0),p)).readFloat()}
 function timeScale(){return unbox(call(meth(cls('UnityEngine','Time'),'get_timeScale',0),ptr(0))).readFloat()}
 function setHp(value){const p=property(),arg=Memory.alloc(4);arg.writeFloat(value);call(meth(propertyClass,'SetHP',1),p,[arg]);return read(p,'Hp')}
 function clampHp(){try{const p=property(),max=read(p,'MaxHp'),hp=read(p,'Hp');if(hp>max)setHp(max)}catch(e){}}
 function schema(){return Object.fromEntries(Object.entries(definitions).map(([k,v])=>[k,{label:v.label,min:v.min,max:v.max,step:v.step}]))}
 function attributes(){
  const out={ready:false,pid:Process.id,timeScale:timeScale(),schema:schema(),overrides:{...overrides}};
  try{
   const u=unit(),p=property();out.characterId=unbox(call(meth(propertyClass,'get_ID',0),p)).readS32();
   out.instanceId=unbox(call(meth(objectClass(u),'GetInstanceID',0),u)).readS32();
   out.name=text(call(meth(propertyClass,'get_Name',0),p));out.values={};out.defaults={};
   for(const [key,list] of Object.entries(readers)){const r=list[list.length-1];out.values[key]=unbox(call(r.mi,p)).readFloat();out.defaults[key]=r.original(p,r.mi)}
   out.hp=read(p,'Hp');out.attackMin=read(p,'AttackMin');out.attackMax=read(p,'AttackMax');
   const tr=call(meth(objectClass(u),'get_transform',0),u),pos=unbox(call(meth(objectClass(tr),'get_position',0),tr));
   out.position=[pos.readFloat(),pos.add(4).readFloat(),pos.add(8).readFloat()];out.ready=true;
  }catch(e){out.message=String(e)}
  return out;
 }
 function applyAttributes(values){
  if(!values||typeof values!=='object'||Array.isArray(values)||!Object.keys(values).length)throw Error('请选择要修改的属性');
  const p=property();
  for(const [key,value] of Object.entries(values)){
   const def=key==='timeScale'?{min:0.1,max:5}:key==='hp'?{min:1,max:values.maxHp??read(p,'MaxHp')}:definitions[key];
   if(!def||typeof value!=='number'||!Number.isFinite(value)||value<def.min||value>def.max)throw Error('属性超出范围: '+key+(def?' ('+def.min+'–'+def.max+')':''));
  }
  for(const [key,value] of Object.entries(values))if(definitions[key]){overrides[key]=Math.fround(value);for(const r of readers[key]){r.slot.add(4).writeFloat(value);r.slot.writeU32(1)}}
  if(values.timeScale!==undefined)speed(values.timeScale);
  if(values.hp!==undefined)setHp(values.hp);else clampHp();
  return attributes();
 }
 function resetAttributes(key='all'){
  if(key==='hp'){setHp(read(property(),'MaxHp'));return attributes()}
  if(key!=='all'&&key!=='timeScale'&&!definitions[key])throw Error('未知属性');
  if(key==='all'){for(const name of Object.keys(overrides)){for(const r of readers[name])r.slot.writeU32(0);delete overrides[name]}speed(1)}
  else if(key==='timeScale')speed(1);else {for(const r of readers[key])r.slot.writeU32(0);delete overrides[key]}
  clampHp();return attributes();
 }
 // Only compare addresses; never dereference a stored object after a scene change.
 let lastProperty='',lastCheck=0;
 function tick(){if(Date.now()-lastCheck<250)return;lastCheck=Date.now();try{const p=property().toString();if(p!==lastProperty){lastProperty=p;clampHp()}}catch(e){lastProperty=''}}
 return {attributes,applyAttributes,resetAttributes,tick,timeScale,callbacks};
}
