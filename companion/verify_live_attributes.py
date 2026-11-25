"""Integration checks against the running isolated client. Always restore defaults."""
from manager import Manager,write_json
m=Manager();r={}
try:
 before=m.local_request(18400,'/attributes')['result'];r['before']=before
 assert before['ready'],before
 values={'timeScale':1.2,'moveSpeed':6.5,'attack':123456,'defense':5432,'maxHp':50000,'jumpSpeed':15,'criticalChance':0.5,'criticalDamage':2.5,'hp':45000}
 a=m.local_request(18400,'/attributes',{'values':values})['result'];r['applied']=a
 for k,v in values.items():
  real=a[k] if k in ('timeScale','hp') else a['values'][k]
  assert abs(real-v)<=max(abs(v)*1e-5,1e-5),(k,v,real)
 assert a['defaults']==before['defaults'],(before['defaults'],a['defaults'])
 try:m.local_request(18400,'/attributes',{'values':{'attack':999,'moveSpeed':999999}})
 except ValueError:r['invalidBatchRejected']=True
 else:raise AssertionError('Invalid batch accepted')
 assert m.local_request(18400,'/attributes')['result']['values']['attack']==123456
 restored_one=m.local_request(18400,'/attributes/reset',{'key':'attack'})['result']
 assert restored_one['values']['attack']==restored_one['defaults']['attack']
 assert restored_one['values']['moveSpeed']==6.5
 r['singleReset']=True
finally:
 r['restored']=m.local_request(18400,'/attributes/reset',{'key':'all'})['result']
 write_json(m.home/'验证/live-attributes-api-test.json',r)
assert r['restored']['timeScale']==1
assert r['restored']['values']==r['restored']['defaults']
assert r['restored']['hp']<=r['restored']['values']['maxHp']
print('实时读回、默认值隔离、非法输入原子拒绝、单项/全部恢复、生命上限校正：通过')
