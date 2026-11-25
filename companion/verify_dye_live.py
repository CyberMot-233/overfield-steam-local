"""Verify real dye requests, client readback, database persistence, and rollback."""
import copy,json,time,frida
from manager import Manager,write_json
m=Manager();uid=m.uid();report={};session=None;changed=False
snap=m.snapshot(uid);player=snap['player'];used=set()
for c in player['character']['characterMap'].values():
 for p in c.get('outfitPresets',{}).values():
  used.update(p.get(k,0) for k in ('hat','hair','clothes','ornament'))
ident=next(int(k) for k in player['item']['itemFashionMap'] if int(k) not in used)
original=copy.deepcopy(player['item']['itemFashionMap'][str(ident)]);index=4;pos=0
report.update(item=ident,index=index,pos=pos,backup=m.action({'action':'backup'})['backup'])
write_json(m.home/'验证/dye-test-recovery.json',dict(report,original=original))
def item():return m.snapshot(uid)['player']['item']['itemFashionMap'][str(ident)]
def wait_for(test):
 for _ in range(40):
  value=item()
  if test(value):return value
  time.sleep(.15)
 raise AssertionError('dye request did not update save')
def color(value):return next((v for v in value.get('dyeSchemes',{}).get(str(index),{}).get('colors',[]) if v.get('pos',0)==pos),None)
try:
 pid=m.local_request(18400,'/status')['result']['pid'];session=frida.attach(pid);script=session.create_script((m.home/'inspect_fashion.js').read_text(encoding='utf8'));script.load();r=script.exports_sync
 r.dyeunlock(ident,index);changed=True;wait_for(lambda i:i.get('dyeSchemes',{}).get(str(index),{}).get('isUnLock'));report['unlock']=True
 r.dyeselect(ident,402);wait_for(lambda i:i.get('colorantId')==402)
 r.dyestage(ident,index,pos,None);wait_for(lambda i:bool(i.get('pendingDyes')))
 r.dyesave(ident,index,pos,True);wait_for(lambda i:color(i) and color(i).get('red')==255 and color(i).get('green')==255 and color(i).get('blue')==255);report['fixedDyeSaved']=True
 desired={'red':12,'green':77,'blue':200}
 r.dyestage(ident,index,pos,desired);wait_for(lambda i:i.get('pendingDyes',{}).get(str(index*64+pos),{}).get('red')==12)
 r.dyesave(ident,index,pos,True);wait_for(lambda i:color(i) and all(color(i).get(k,0)==v for k,v in desired.items()));report['customDyeSaved']=True
 report['clientReadback']=r.dyeread(ident,index,pos);assert report['clientReadback']['color']==desired
 r.dyestage(ident,index,pos,{'red':255,'green':0,'blue':0});wait_for(lambda i:bool(i.get('pendingDyes')))
 r.dyesave(ident,index,pos,False);wait_for(lambda i:not i.get('pendingDyes'));assert all(color(item()).get(k,0)==v for k,v in desired.items());report['cancelPreservedSavedColor']=True
 session.detach();session=None;m.stop()
 stored=m.load(uid)[1]['item']['itemFashionMap'][str(ident)];assert all(color(stored).get(k,0)==v for k,v in desired.items());report['databasePersistence']=True
finally:
 if session:session.detach()
 if changed:
  if m.client_processes():m.stop()
  latest=m.load(uid)[1];latest['item']['itemFashionMap'][str(ident)]=original;m.save_live_snapshot({'uid':uid,'player':latest});report['testItemRestored']=True
  m.action({'action':'start','uid':uid})
 write_json(m.home/'验证/dye-live-test.json',report)
print(json.dumps(report,ensure_ascii=False,indent=2))
