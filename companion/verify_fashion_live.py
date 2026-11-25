"""End-to-end wardrobe persistence using the client's real request helper."""
import copy,json,time,frida
from manager import Manager,write_json
m=Manager();uid=m.uid();report={};script=None;session=None

def attach():
 global session,script
 state=m.local_request(18400,'/status')['result'];session=frida.attach(state['pid']);script=session.create_script((m.home/'inspect_fashion.js').read_text(encoding='utf8'));script.load();return script.exports_sync

def character():return m.snapshot(uid)['player']['character']['characterMap'][str(cid)]
def wait_for(test):
 for _ in range(40):
  c=character()
  if test(c):return c
  time.sleep(.15)
 raise AssertionError('server save did not reach expected state')
a=m.local_request(18400,'/attributes')['result'];cid=a['characterId'];before=character();active=before.get('inUseOutfitPresetIndex',0);slot=(active+1)%3
original=copy.deepcopy(before['outfitPresets'][str(slot)]);trial=copy.deepcopy(original);trial['presetIndex']=slot
# Use a real owned hat already present in another preset; never grant test items.
candidates=[p.get('hat',0) for p in before['outfitPresets'].values() if p.get('hat',0)!=trial.get('hat',0)]
if not candidates:raise RuntimeError('No alternate owned hat available for the test')
trial['hat']=candidates[0];trial['hatDyeSchemeIndex']=0
backup=m.action({'action':'backup'})['backup'];report.update(backup=backup,character=cid,slot=slot,original=original,trial=trial)
write_json(m.home/'验证/fashion-test-recovery.json',report)
r=attach();changed=False
try:
 r.write(cid,trial);changed=True
 c=wait_for(lambda c:c['outfitPresets'][str(slot)].get('hat')==trial['hat']);assert c.get('inUseOutfitPresetIndex',0)==active
 report['saveRequestApplied']=True
 # Verify the formerly missing visibility route and restore it immediately.
 hide0=copy.deepcopy(c['outfitPresets'][str(active)].get('outfitHideInfo',{}))
 try:
  r.hide(cid,not hide0.get('hideOrn',False),hide0.get('hideBraid',False))
  wait_for(lambda c:c['outfitPresets'][str(active)].get('outfitHideInfo',{}).get('hideOrn',False)!=hide0.get('hideOrn',False));report['hideRequestApplied']=True
 finally:
  r.hide(cid,hide0.get('hideOrn',False),hide0.get('hideBraid',False));wait_for(lambda c:c['outfitPresets'][str(active)].get('outfitHideInfo',{}).get('hideOrn',False)==hide0.get('hideOrn',False))
 session.detach();session=None;script=None
 m.stop();m.action({'action':'start','uid':uid})
 for _ in range(90):
  try:
   if m.local_request(18400,'/attributes')['result']['ready'] and m.snapshot(uid):break
  except Exception:pass
  time.sleep(.5)
 else:raise AssertionError('client did not enter the scene')
 r=attach();actual=r.read(cid,slot);report['clientAfterRestart']=actual
 assert actual['preset']['hat']==trial['hat'];assert actual['active']==active
 report['restartPersistence']=True
finally:
 if changed:
  if session is None:r=attach()
  r.write(cid,original);wait_for(lambda c:c['outfitPresets'][str(slot)].get('hat')==original.get('hat'))
  report['testOutfitRestored']=True
 if session:session.detach()
 write_json(m.home/'验证/fashion-live-test.json',report)
print(json.dumps(report,ensure_ascii=False,indent=2))
