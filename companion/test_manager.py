import gzip,json,os,sqlite3,unittest,uuid
from pathlib import Path
from contextlib import closing
from unittest.mock import patch
from manager import Manager,PORTABLE,BASE,write_json

class SaveIntegrationTests(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  cls.root=BASE/'验证'/('save-tests-'+uuid.uuid4().hex[:8]);cls.home=cls.root/'manager';cls.home.mkdir(parents=True)
  cls.portable=cls.root/'portable';profile=cls.portable/'profiles/solo';(profile/'db').mkdir(parents=True);(profile/'Resource/Excel').mkdir(parents=True)
  import shutil
  for n in ['Item.json','Character.json','String.json','Level.json','UI.json','Weapon.json']:shutil.copy2(PORTABLE/'profiles/solo/Resource/Excel'/n,profile/'Resource/Excel'/n)
  if (PORTABLE/'profiles/solo/client-compatibility.json').exists():shutil.copy2(PORTABLE/'profiles/solo/client-compatibility.json',profile/'client-compatibility.json')
  assets=PORTABLE/'profiles/solo/Resource/Config/LocalClientAssets.json'
  if assets.exists():
   (profile/'Resource/Config').mkdir(parents=True,exist_ok=True);shutil.copy2(assets,profile/'Resource/Config/LocalClientAssets.json')
  for n in ['launcher.settings.json','model_schema.json']:shutil.copy2(PORTABLE/n,cls.portable/n)
  shutil.copy2(PORTABLE/'profiles/solo/config.json',profile/'config.json')
  os.link(BASE/'localgm.exe',cls.home/'localgm.exe')
  with closing(sqlite3.connect(PORTABLE/'profiles/solo/db/lolo.db')) as src,closing(sqlite3.connect(profile/'db/lolo.db')) as dst:src.backup(dst)
  cls.source_uid=Manager().uid();cls.m=Manager(cls.portable,cls.home);cls.m.client_processes=lambda:[];cls.m.world=lambda *a,**k:''
  cls.offline=patch('manager.port_open',return_value=False);cls.offline.start()
 @classmethod
 def tearDownClass(cls):cls.offline.stop()
 def test_01_preset_and_isolation(self):
  with closing(sqlite3.connect(self.m.db)) as c:before=c.execute('SELECT bin_data FROM of_game WHERE user_id=?',(self.source_uid,)).fetchone()[0]
  r=self.m.mutate(10023991,{'operation':'preset','accountLevel':50,'name':'test','count':9999999})
  self.assertEqual(r['summary']['characters'],len(self.m.catalog()['characters']));self.assertEqual(r['summary']['skins'],sum(x['kind']=='Fashion' and x.get('available',True) for x in self.m.catalog()['items']));self.assertEqual(r['skipped'],[])
  basic,p=self.m.load(10023991);self.assertEqual(basic['level'],50)
  self.assertEqual(p['item']['itemBaseInfo']['101']['num'],9999999)
  self.assertIs(p['item']['itemFashionMap']['4011012']['dyeSchemes']['0']['isUnLock'],True)
  with closing(sqlite3.connect(self.m.db)) as c:self.assertEqual(before,c.execute('SELECT bin_data FROM of_game WHERE user_id=?',(self.source_uid,)).fetchone()[0])
 def test_02_idempotent(self):
  before=self.m.summary(10023991)['counts'];self.m.mutate(10023991,{'operation':'preset','count':9999999});self.assertEqual(before,self.m.summary(10023991)['counts'])
 def test_03_give_exact_amount(self):
  self.m.mutate(10023991,{'operation':'item','id':101,'count':7});self.assertEqual(self.m.load(10023991)[1]['item']['itemBaseInfo']['101']['num'],10000006)
 def test_04_reject_invalid_without_mutation(self):
  before=self.m.load(10023991)
  for request in [{'operation':'init','accountLevel':51},{'operation':'item','id':4294967295},{'operation':'team','team':[101001,101001,101003]},{'operation':'init','count':-1}]:
   with self.assertRaises(ValueError):self.m.mutate(10023991,request)
  self.assertEqual(before,self.m.load(10023991))
 def test_05_backup_restore(self):
  name=self.m.backup('test');before=self.m.load(10023991);self.m.mutate(10023991,{'operation':'init','accountLevel':12});self.m.restore(name);self.assertEqual(before,self.m.load(10023991))
 def test_06_reject_path_escape(self):
  with self.assertRaises(ValueError):self.m.restore('../outside.db')
 def test_07_reject_online_edit(self):
  self.m.client_processes=lambda:[123]
  try:
   with self.assertRaises(ValueError):self.m.mutate(10023991,{'operation':'init'})
  finally:self.m.client_processes=lambda:[]

 def test_08_permanent_items_do_not_stack(self):
  uid=10023991
  self.m.mutate(uid,{'operation':'preset','count':9999999})
  _,p=self.m.load(uid)
  for item in ('110001011','110001013'):
   self.assertEqual(p['item']['itemBaseInfo'][item]['num'],1)
  coins=p['item']['itemBaseInfo']['101']['num']
  self.m.mutate(uid,{'operation':'item','id':110001011,'count':9999999})
  self.m.mutate(uid,{'operation':'item','id':110001011,'count':9999999})
  _,p=self.m.load(uid)
  self.assertEqual(p['item']['itemBaseInfo']['110001011']['num'],1)
  self.assertEqual(p['item']['itemBaseInfo']['101']['num'],coins)
 def test_09_repair_legacy_counts_preserves_collection(self):
  _,p=self.m.load(10023991)
  old_counts=self.m.summary(10023991)['counts']
  p['item']['itemBaseInfo']['110001011']['num']=9999999
  result=self.m.model(p,{'Operation':'repair-abilities','Count':1})
  self.assertEqual(result['Player']['item']['itemBaseInfo']['110001011']['num'],1)
  self.assertEqual({k:result['Summary'][k] for k in old_counts},old_counts)
  self.assertEqual(result['Player']['character'],p['character'])
  self.assertEqual(result['Player']['item']['itemFashionMap'],p['item']['itemFashionMap'])

 def test_10_inventory_catalog_and_repair_preserve_account(self):
  entries={x['id']:x for x in self.m.catalog()['items']}
  for id in (5070001,5070002,2111008,2133009,1301100):self.assertFalse(entries[id]['available'],id)
  self.assertTrue(entries[2111001]['available'])
  uid=10023991
  basic,before=self.m.load(uid)
  result=self.m.mutate(uid,{'operation':'repair-inventory'})
  after_basic,after=self.m.load(uid)
  self.assertEqual(basic['nick_name'],after_basic['nick_name'])
  self.assertEqual(before['team'],after['team'])
  self.assertEqual(before['character'],after['character'])
  self.assertEqual(before.get('localProgress'),after.get('localProgress'))
  self.assertTrue(all(v.get('mainPropertyVal',0)>0 for v in after['item']['itemArmorMap'].values()))
  self.assertEqual(sum(result['inventoryRepair'].values()),0)

 def test_11_weapon_instance_persistence_and_isolation(self):
  import copy
  uid=10023991
  listed=self.m.weapons(uid);w=next(w for w in listed['weapons'] if w['ranges'])
  self.m.mutate(uid,{'operation':'item','id':w['weaponId'],'count':1})
  w=next(x for x in self.m.weapons(uid)['weapons'] if x['instanceId']==w['instanceId'])
  before_basic,before=self.m.load(uid)
  request={'action':'weapon-edit','uid':uid,'instanceId':w['instanceId'],'weaponId':w['weaponId'],'revision':w['revision'],'changes':{'level':w['ranges'][0]['min'],'attack':765432,'criticalRatio':100,'damageBalance':125,'strengthLevel':0}}
  result=self.m.action(request)
  self.assertTrue(result['saved']);self.assertFalse(result['online']);self.assertTrue((self.m.backups/result['backup']).is_file())
  basic,after=self.m.load(uid);self.assertEqual(basic,before_basic)
  key=str(w['instanceId']);edited=after['item']['itemWeaponMap'][key]
  self.assertEqual(edited['localStats']['attack'],765432);self.assertEqual(edited['level'],w['ranges'][0]['min'])
  self.assertEqual(edited['localStats']['criticalRatio'],100);self.assertEqual(edited['localStats']['damageBalance'],125)
  expected=copy.deepcopy(before);expected['item']['itemWeaponMap'][key]=edited
  self.assertEqual(after,expected)
  with self.assertRaises(ValueError):self.m.action(request)
  self.assertEqual(self.m.load(uid),(basic,after))
  reread=next(x for x in self.m.weapons(uid)['weapons'] if x['instanceId']==w['instanceId'])
  self.assertEqual(reread['attack'],765432);self.assertNotEqual(reread['revision'],w['revision'])
 def test_12_weapon_invalid_fields_do_not_write(self):
  uid=10023991;w=self.m.weapons(uid)['weapons'][0];before=self.m.load(uid)
  base={'action':'weapon-edit','uid':uid,'instanceId':w['instanceId'],'weaponId':w['weaponId'],'revision':w['revision']}
  for changes in ({'level':0},{'level':10000},{'attack':-1},{'attack':1.2},{'attack':True},{'criticalRatio':10001},{'strengthLevel':9999},{'wearerId':101001},{}):
   with self.assertRaises(ValueError):self.m.action(dict(base,changes=changes))
   self.assertEqual(self.m.load(uid),before)


 def test_13_weapon_reset_keeps_native_parameters(self):
  uid=10023991;w=next(w for w in self.m.weapons(uid)['weapons'] if w['attack']==765432)
  before_basic,before=self.m.load(uid);key=str(w['instanceId']);old=before['item']['itemWeaponMap'][key]
  result=self.m.action({'action':'weapon-edit','uid':uid,'instanceId':w['instanceId'],'weaponId':w['weaponId'],'revision':w['revision'],'changes':{'resetStats':1}})
  basic,after=self.m.load(uid);new=after['item']['itemWeaponMap'][key]
  self.assertTrue(result['saved']);self.assertEqual(basic,before_basic);self.assertFalse(new.get('localStats'))
  for field in ('attack','criticalRatio','damageBalance','level','strengthLevel','propertyIndex','wearerId'):
   self.assertEqual(new.get(field),old.get(field),field)
 def test_14_unlock_all_persists_and_keeps_party(self):
  uid=10023991;basic,before=self.m.load(uid)
  result=self.m.mutate(uid,{'operation':'unlock-all'})
  after_basic,after=self.m.load(uid)
  self.assertEqual(result['unlocks']['abilities'],67);self.assertGreater(result['unlocks']['actions'],0)
  self.assertEqual(after.get('team'),before.get('team'))
  import copy
  expected=copy.deepcopy(before['character'])
  for key,c in expected['characterMap'].items():
   a=c.setdefault('characterAppearance',{})
   for field in ('insectNetInstanceId','loggingAxeInstanceId','waterBottleInstanceId','miningHammerInstanceId','collectionGlovesInstanceId','fishingRodInstanceId','vehicleInstanceId','shovelInstanceId'):
    if field in after['character']['characterMap'][key]['characterAppearance']:a[field]=after['character']['characterMap'][key]['characterAppearance'][field]
  self.assertEqual(after['character'],expected)
  self.assertGreater(result['unlocks']['toolSlots'],0)
  self.assertEqual(after['item']['itemWeaponMap'],before['item']['itemWeaponMap'])
  self.assertEqual(after['item']['itemBaseInfo']['101'],before['item']['itemBaseInfo']['101'])
  self.assertEqual(after_basic['level'],basic['level'])
  self.assertTrue(after['localUnlocks']['actions']);self.assertTrue(after['localUnlocks']['blessTrees'])
  self.m.mutate(uid,{'operation':'unlock-all'})
  self.assertEqual(self.m.load(uid)[1],after)

if __name__=='__main__':unittest.main(verbosity=2)
