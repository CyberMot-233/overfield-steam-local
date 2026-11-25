import time
from manager import Manager,write_json
m=Manager();r={}
r['before']=m.local_request(18400,'/attributes')['result']
coins_before=m.snapshot(m.uid())['player']['item']['itemBaseInfo']['101']['num']
r['temporary']=m.local_request(18400,'/attributes',{'values':{'moveSpeed':7.1,'attack':123456,'timeScale':1.3}})['result']
print(m.stop(),flush=True)
r['normalSaveBackup']=m.backup('速度修复-全收集账号-'+str(m.uid()))
print(m.action({'action':'start','uid':m.uid()}),flush=True)
deadline=time.monotonic()+45
while time.monotonic()<deadline:
 try:
  d=m.local_request(18400,'/attributes')['result']
  if d['ready']:break
 except Exception:pass
 time.sleep(.5)
else:raise AssertionError('Client did not enter the scene')
r['after']=d
assert d['pid']!=r['before']['pid']
assert d['overrides']=={} and d['timeScale']==1
assert d['values']==d['defaults']
assert abs(d['values']['moveSpeed']-r['before']['defaults']['moveSpeed'])<1e-4,d
snap=m.snapshot(m.uid());items=snap['player']['item']['itemBaseInfo']
assert items['110001011']['num']==items['110001013']['num']==1
assert items['101']['num']==coins_before
r['account']=m.summary(m.uid());r['passed']=True
write_json(m.home/'验证/live-restart-test.json',r)
print('重启清除临时覆盖、正常速度、能力数量和全收集存档保留：通过',flush=True)
