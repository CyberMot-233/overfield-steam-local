import frida,json
from manager import Manager,write_json
m=Manager();pid=m.local_request(18400,'/status')['result']['pid'];s=frida.attach(pid);report={}
try:
 script=s.create_script((m.home/'inspect_units.js').read_text(encoding='utf8'));script.load()
 before=script.exports_sync.units();report['before']=before
 m.local_request(18400,'/attributes',{'values':{'attack':234567,'moveSpeed':6.7}})
 after=script.exports_sync.units();report['after']=after
 old={x['unit']:x for x in before};others=0;main=False
 for x in after:
  if x['main']:
   assert x['AttackMax']==234567,x
   assert abs(x['MoveSpeed']-6.7)<1e-4,x
   main=True
  elif x['unit'] in old:
   assert x==old[x['unit']],(x,old[x['unit']]);others+=1
 assert main and others>0,(main,others)
 report['otherUnitsUnchanged']=others;print('主控角色生效，其他单位保持原值:',others)
finally:
 m.local_request(18400,'/attributes/reset',{'key':'all'});s.detach();write_json(m.home/'验证/live-unit-isolation.json',report)
