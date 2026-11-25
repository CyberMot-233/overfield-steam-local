"""Export the running isolated Steam client's configuration for local inspection.
No game resources are included in the source repository.
"""
import argparse,json,shutil
from pathlib import Path
from manager import Manager,write_json

def normalize(value):
    if isinstance(value,dict):return {('Datas' if k=='datas' else k):normalize(v) for k,v in value.items()}
    if isinstance(value,list):return [normalize(v) for v in value]
    return value

def export():
    m=Manager();state=m.local_request(18400,'/status')['result']
    if not state.get('ready'):raise ValueError('请先启动单机并进入场景')
    dest=m.home/'Steam客户端配置';(dest/'Excel').mkdir(parents=True,exist_ok=True)
    report={}
    sources={p.stem:p for p in (m.profile/'Resource/Excel').glob('*.json')}
    for name in ('Expression','Action','Ability','BlessingTree'):
        sources.setdefault(name,m.profile/'Resource/Excel'/(name+'.json'))
    for source in sorted(sources.values()):
        try:
            data=m.local_request(18400,'/config-catalog/'+source.stem,timeout=30)['result']
            write_json(dest/'Excel'/source.name,normalize(data));report[source.stem]='exported'
        except Exception as exc:
            if source.stem!='Exploration':raise
            # This unused legacy table has no corresponding Steam manager.
            shutil.copy2(source,dest/'Excel'/source.name);report[source.stem]='retained legacy table'
    shutil.copytree(m.profile/'Resource/Config',dest/'Config',dirs_exist_ok=True)
    write_json(dest/'export-report.json',report)
    print('配置已导出到：'+str(dest))
    print('导出仅写入候选目录；更新服务端前请备份、退出并运行模型验证。')

if __name__=='__main__':export()
