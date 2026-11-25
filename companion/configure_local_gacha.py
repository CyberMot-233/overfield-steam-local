"""Create an explicitly local, permanent pool selection from installed Steam tables.
Does not claim to reproduce the official server's current event schedule.
"""
import argparse, datetime as dt, json
from manager import Manager, read_json, write_json

def build(manager=None):
    m=manager or Manager();catalog=m.catalog()
    items={i['id'] for i in catalog['items'] if i['available']}
    characters={c['id'] for c in catalog['characters']}
    g=read_json(m.profile/'Resource/Excel/Gacha.json')
    pools={r['ID']:r for r in g['Pool']['Datas']}
    rewards={r['ID']:r for r in g['RewardPool']['Datas']}
    selected={};excluded=[];now=dt.datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    for row in g['Info']['Datas']:
        cid=row.get('CharacterID1',0)
        if cid and cid not in characters:
            excluded.append({'id':row['ID'],'reason':'character assets or skills incomplete'});continue
        pool=pools.get(row.get('Method1ID'))
        if pool is None:
            excluded.append({'id':row['ID'],'reason':'unsupported draw method'});continue
        reward_ids={v.get('FreeGachaPoolID',0) for v in pool.get('Items',[])}
        if row.get('BigGuaranteePoolID'):reward_ids.add(row['BigGuaranteePoolID'])
        missing=[v['ItemID'] for rid in reward_ids for v in rewards.get(rid,{}).get('Items',[]) if v['ItemID'] not in items]
        if not reward_ids or any(rid not in rewards for rid in reward_ids) or missing:
            excluded.append({'id':row['ID'],'reason':'reward configuration incomplete','items':sorted(set(missing))});continue
        key=(row.get('NewUIGachaType'),cid or row['ID'])
        # Prefer a released rerun for the same character; retain each pool's saved history.
        rank=(row.get('OpeningTime','')<=now,row.get('OpeningTime',''),row['ID'])
        if key not in selected or rank>selected[key][0]:selected[key]=(rank,row)
    rows=[v[1] for v in selected.values()]
    rows.sort(key=lambda r:(r.get('NewUIGachaType')!=3,r.get('CharacterID1') not in (201002,402002,301004,302003),r['ID']))
    version=read_json(m.profile/'Resource/Config/LocalClientAssets.json',{}).get('resourceVersion','unknown')
    return {'mode':'all-available','label':'单机常驻卡池（本地配置）','resourceVersion':version,
            'openingTime':'2020-01-01 00:00:00','closingTime':'2099-12-31 23:59:59',
            'poolIds':[r['ID'] for r in rows],'characters':{str(r['ID']):r.get('CharacterID1',0) for r in rows},
            'excluded':excluded,'officialSchedule':False}

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--apply',action='store_true');args=parser.parse_args()
    m=Manager();result=build(m)
    if args.apply:
        if m.client_processes():raise SystemExit('请先保存并退出单机，再应用卡池配置。')
        write_json(m.profile/'Resource/Config/LocalGacha.json',result)
    write_json(m.home/'验证/local-gacha-catalog.json',result)
    print(json.dumps(result,ensure_ascii=False,indent=2))
