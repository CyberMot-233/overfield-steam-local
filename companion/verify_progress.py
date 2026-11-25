"""Exercise real model operations on copies; never writes the live database."""
import json,subprocess,uuid
from pathlib import Path
from manager import Manager,write_json
m=Manager();folder=m.home/'验证'/('progress-tests-'+uuid.uuid4().hex[:8]);folder.mkdir()
exe=m.home/'localgm.exe'
snap=m.snapshot(m.uid());player=snap['player'] if snap else m.load(m.uid())[1]
def run(player,operation,**kwargs):
 inp=folder/'in.json';out=folder/'out.json';out.unlink(missing_ok=True);write_json(inp,dict(Player=player,Operation=operation,**kwargs))
 with (folder/'model.log').open('w',encoding='utf8') as log:
  result=subprocess.run([str(exe),'--config',str(m.profile/'config.json'),'--input',str(inp),'--output',str(out)],stdout=log,stderr=log)
 if result.returncode:raise RuntimeError((folder/'model.log').read_text(encoding='utf8')[-3000:])
 return json.loads(out.read_text(encoding='utf8'))
before=run(player,'validate');all_=run(before['Player'],'progress-all');again=run(all_['Player'],'progress-all')
p=all_['Player'];bp=before['Player'];report={'summary':all_['Progress'],'expected':m.progress_catalog(),'worldLevelID':p['scene']['worldLevel']}
assert p['team']==bp['team'],'team changed'
assert p['character']==bp['character'],'outfits or character changed'
assert p['item']==bp['item'],'inventory changed'
assert p==again['Player'],'repeated application changed save'
assert again['Applied']==0,'not idempotent'
assert all_['Progress']['quests']==report['expected']['quests']
assert all_['Progress']['maps']==report['expected']['maps']
assert all_['Progress']['collections']==report['expected']['collections']
assert p['scene']['worldLevel'] in [v['id'] for v in m.catalog()['worldLevels']]
# Serialized saves must reload through exactly the same model and protobuf conversion.
roundtrip=run(json.loads(json.dumps(p)),'validate');assert roundtrip['Player']==p
for kind in ('story','maps','collections'):
 one=run(bp,'progress-'+kind);assert one['Player']['team']==bp['team'];assert one['Player']['character']==bp['character']
 if kind=='story':assert one['Progress']['maps']==before['Progress']['maps'];assert one['Progress']['collections']==before['Progress']['collections']
 if kind=='maps':assert one['Progress']['quests']==before['Progress']['quests'];assert one['Progress']['collections']==before['Progress']['collections']
report['checks']=['narrative-count','available-map-count','permanent-collection-count','idempotence','json-roundtrip','outfit-preservation','team-preservation','inventory-preservation','independent-operations']
write_json(folder/'result.json',report);write_json(m.home/'验证/progress-model-test.json',report)
print(json.dumps(report,ensure_ascii=False,indent=2))
