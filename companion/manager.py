"""Local OverField save and runtime manager; no external service required."""
from __future__ import annotations
import argparse, math, datetime as dt, gzip, hmac, json, os, re, secrets, shutil, socket, sqlite3, subprocess, sys, threading, time, webbrowser
from contextlib import closing
from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import Request, ProxyHandler, build_opener
from urllib.error import HTTPError

BASE=Path(__file__).resolve().parent
PORTABLE=BASE.parent/'OverFieldLauncher-portable'
PORT=18450
LOCK=threading.Lock()
SUPPORTED_GM={'add_item','add_item_temp_pack','weaponadd','armoradd','posteradd','spellcardadd','change_item','gm_get_item_num','player_level','charlv','addallgmitem','add_gm_count_item'}
FIXED_GM=set()
SUPPORTED_GM.update({'quest_complete','forcequestcomplete','unlockscene','unlockallareaandaddflagrecord'})
NO_WINDOW=getattr(subprocess,'CREATE_NO_WINDOW',0)
OPENER=build_opener(ProxyHandler({}))

def read_json(p,default=None):
    try:return json.loads(Path(p).read_text(encoding='utf-8-sig'))
    except FileNotFoundError:return default

def write_json(p,data):
    p=Path(p);p.parent.mkdir(parents=True,exist_ok=True)
    tmp=p.with_suffix(p.suffix+'.tmp');tmp.write_text(json.dumps(data,ensure_ascii=False,indent=2),encoding='utf8');tmp.replace(p)

def port_open(port):
    try:
        with socket.create_connection(('127.0.0.1',port),.2):return True
    except OSError:return False

class Manager:
    def __init__(self,portable=PORTABLE,home=BASE):
        self.portable=Path(portable).resolve();self.home=Path(home).resolve()
        self.profile=self.portable/'profiles/solo';self.db=self.profile/'db/lolo.db'
        self.backups=self.home/'存档备份';self.backups.mkdir(parents=True,exist_ok=True)
        self.runtime=self.home/'runtime';self.runtime.mkdir(parents=True,exist_ok=True)
        self.catalog_cache=None
    def settings(self):return read_json(self.portable/'launcher.settings.json',{})
    def uid(self):return int(self.settings().get('uid',1000000))
    def key(self):return (self.profile/'admin-key.txt').read_text(encoding='utf8').strip()
    def local_request(self,port,endpoint,body=None,timeout=15):
        data=None if body is None else json.dumps(body).encode()
        req=Request('http://127.0.0.1:'+str(port)+endpoint,data=data,headers={'X-Local-Token':self.key(),'Content-Type':'application/json'})
        try:
            with OPENER.open(req,timeout=timeout) as rsp:return json.load(rsp)
        except HTTPError as exc:
            raw=exc.read().decode('utf8','replace')
            try:raw=json.loads(raw).get('error',raw)
            except ValueError:pass
            raise ValueError(raw) from exc
    def snapshot(self,uid):
        if not port_open(18080):return None
        try:return self.local_request(18080,'/local-admin/snapshot/'+str(uid))
        except ValueError as exc:
            if 'offline' in str(exc).lower() or '玩家不在线' in str(exc):return None
            raise ValueError('读取实时存档失败：'+str(exc)) from exc
    def client_processes(self):
        # Only enumerate our isolated executable. No official-client process is stopped.
        target=str(self.profile/'client/launcher.exe').replace("'","''")
        code="@(Get-CimInstance Win32_Process -Filter \"Name='launcher.exe'\" | Where-Object { $_.ExecutablePath -eq '"+target+"' } | Select-Object -ExpandProperty ProcessId) | ConvertTo-Json -Compress"
        p=subprocess.run(['powershell.exe','-NoProfile','-NonInteractive','-Command',code],capture_output=True,creationflags=NO_WINDOW)
        try:
            value=json.loads(p.stdout.decode('utf8','replace') or '[]');return value if isinstance(value,list) else [value]
        except ValueError:return []
    def world(self,action,launch=False,uid=None):
        setting=self.settings();args=['powershell.exe','-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',str(self.portable/'World-Control.ps1'),'-Action',action,'-Mode','solo','-GameRoot',setting.get('gameRoot',''),'-Uid',str(uid or self.uid()),'-PythonExe',str(self.portable/'runtime-python/python.exe')]
        if launch:args.append('-LaunchClient')
        p=subprocess.run(args,capture_output=True,encoding='utf8',errors='replace',creationflags=NO_WINDOW,timeout=100)
        if p.returncode:raise ValueError(p.stderr.strip() or p.stdout.strip() or '启动器操作失败')
        return p.stdout.strip()
    def _close_client(self):
        ids=self.client_processes()
        if not ids:return
        code='$ids=@('+','.join(str(int(v)) for v in ids)+'); foreach($i in $ids){$p=Get-Process -Id $i -ErrorAction SilentlyContinue;if($p){$null=$p.CloseMainWindow();if(-not $p.WaitForExit(8000)){Stop-Process -Id $i}}}'
        subprocess.run(['powershell.exe','-NoProfile','-NonInteractive','-Command',code],capture_output=True,creationflags=NO_WINDOW,timeout=30)
    def stop(self):
        active=read_json(self.portable/'runtime/world-state.json',{}) or {}
        uid=int(active.get('Uid') or self.uid());snap=self.snapshot(uid)
        if self.client_processes() and not snap:
            raise ValueError('无法读取正在游玩的存档。请先在游戏中退出登录，再停止服务。')
        self._close_client();self.world('Stop')
        if snap:self.save_live_snapshot(snap)
        return {'message':'单机已关闭，最新进度已保存。'}
    def save_live_snapshot(self,snap):
        uid=int(snap['uid']);blob=gzip.compress(json.dumps(snap['player'],ensure_ascii=False,separators=(',',':')).encode(),mtime=0)
        with closing(sqlite3.connect(self.db,timeout=10)) as c,c:
            c.execute('INSERT INTO of_game(user_id,bin_data) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET bin_data=excluded.bin_data',(uid,blob))
    def backup(self,label='manual'):
        name=dt.datetime.now().strftime('%Y%m%d-%H%M%S-%f')+'-'+label+'.db';dest=self.backups/name
        with closing(sqlite3.connect(self.db.resolve().as_uri()+'?mode=ro',uri=True)) as source,closing(sqlite3.connect(dest)) as target:source.backup(target)
        return name
    def load(self,uid):
        with closing(sqlite3.connect(self.db,timeout=10)) as c:
            c.row_factory=sqlite3.Row
            row=c.execute('SELECT * FROM of_game_basic WHERE user_id=?',(uid,)).fetchone()
            data=c.execute('SELECT bin_data FROM of_game WHERE user_id=?',(uid,)).fetchone()
            player=json.loads(gzip.decompress(data[0])) if data and data[0] else {}
            return dict(row) if row else {},player
    def accounts(self):
        with closing(sqlite3.connect(self.db)) as c:
            return [dict(uid=r[0],name=r[1] or '',level=r[2] or 1) for r in c.execute('SELECT u.user_id,b.nick_name,b.level FROM of_user u LEFT JOIN of_game_basic b ON b.user_id=u.user_id ORDER BY u.user_id')]
    def catalog(self):
        if self.catalog_cache is not None:return self.catalog_cache
        excel=self.profile/'Resource/Excel'
        tables=read_json(excel/'String.json');names={int(x['ID']):x.get('Text',[''])[0] for x in tables['StringItemText']['Datas']}
        charnames={int(x['ID']):x.get('Text',[''])[0] for x in tables['StringCharacterName']['Datas']}
        char=read_json(excel/'Character.json')['Character']['Datas']
        items=read_json(excel/'Item.json')['Item']['Datas']
        enums=read_json(self.portable/'model_schema.json')['enums'];tags={v:k.replace('EBagItemTag_','') for k,v in enums.items() if k.startswith('EBagItemTag_')}
        unavailable=set(read_json(self.profile/'client-compatibility.json',{}).get('unavailableItems',[]))
        assets=read_json(self.profile/'Resource/Config/LocalClientAssets.json',{}).get('characterAssets')
        def playable(c):
            present=(assets.get(str(c['ID']),False) if assets is not None else bool(c.get('IsShow')))
            return present and bool(c.get('AssetFullName')) and len(c.get('SpellIDs',[]))>=2 and bool(c.get('ExSpellIDs'))
        unavailable.difference_update(c['ID'] for c in char if playable(c))
        unavailable.update(c['ID'] for c in char if not playable(c))
        ui=read_json(excel/'UI.json',{})
        icons={row['ID']:row.get('AssetFullName',[]) for row in ui.get('Icon',{}).get('Datas',[])}
        for item in items:
            if item['NewBagItemTag']==53:
                unavailable.add(item['ID'])
            elif item['NewBagItemTag'] in (2,3) and icons:
                paths=icons.get(item.get('IconID'),[])
                if len(paths)<3 or not all(paths):unavailable.add(item['ID'])
        self.catalog_cache={'items':[{'id':x['ID'],'name':names.get(x.get('TextID',x['ID']),str(x['ID'])),'tag':x['NewBagItemTag'],'available':x['ID'] not in unavailable,'kind':tags.get(x['NewBagItemTag'],str(x['NewBagItemTag']))} for x in items],
          'characters':[{'id':x['ID'],'name':charnames.get(x.get('NameID',x['ID']),names.get(x['ID'],str(x['ID'])))} for x in char if playable(x)],
          'maxLevel':max(x['ID'] for x in read_json(excel/'Level.json')['PlayerLevel']['Datas']),
          'worldLevels':[{'id':x['ID'],'level':x.get('Level',0)} for x in read_json(excel/'Level.json')['WorldLevel']['Datas']],
          'progress':self.progress_catalog()}
        return self.catalog_cache
    def progress_catalog(self):
        excel=self.profile/'Resource/Excel'
        quests=read_json(excel/'Quest.json',{}).get('Quest',{}).get('Datas',[])
        collections=read_json(excel/'CollectionItem.json',{}).get('CollectionItem',{}).get('Datas',[])
        scenes=read_json(self.profile/'Resource/Config/ScenesConfigAsset.json',{}).get('Scenes',[])
        return {'quests':sum(q.get('NewType') in (1,2,3,9) for q in quests),'maps':len(scenes),'collections':sum(1<=q.get('NewCollectionType',0)<=9 for q in collections),'sceneIds':[q['ID'] for q in scenes]}
    def summary(self,uid):
        basic,player=self.load(uid);item=player.get('item',{});char=player.get('character',{}).get('characterMap',{})
        result={'uid':uid,'name':basic.get('nick_name',''),'level':basic.get('level',1),'exp':basic.get('exp',0),'sign':basic.get('sign',''),
          'counts':{'characters':len(char),'skins':len(item.get('itemFashionMap',{})),'resources':len(item.get('itemBaseInfo',{})),'weapons':len(item.get('itemWeaponMap',{})),'armor':len(item.get('itemArmorMap',{})),'pets':len(item.get('itemPetInfoMap',{}))},
          'characters':list(char.values()),'team':player.get('team',{}).get('TeamInfo',{}),'worldLevel':player.get('scene',{}).get('worldLevel',1),
          'progress':{'quests':len(player.get('localProgress',{}).get('quests',{})),'maps':len(player.get('localProgress',{}).get('maps',{})),'collections':len(player.get('localProgress',{}).get('collections',{})),'completedMaps':len(player.get('localProgress',{}).get('completedScenes',{}))}}
        unlocked=player.get('localUnlocks',{})
        result['unlocks']={'actions':len(unlocked.get('actions',{})),'abilities':len(unlocked.get('abilities',{})),'blessNodes':sum(len(v) for v in unlocked.get('blessTrees',{}).values())}
        return result
    def status(self,uid=None):
        uid=int(uid or self.uid());return {'account':self.summary(uid),'accounts':self.accounts(),'server':port_open(18080),'game':port_open(18400),'profile':str(self.profile),'backups':sorted((p.name for p in self.backups.glob('*.db')),reverse=True),'localGacha':read_json(self.profile/'Resource/Config/LocalGacha.json',{}),'resourceVersion':read_json(self.profile/'Resource/Config/LocalClientAssets.json',{}).get('resourceVersion','未同步'),'questPolicy':'按当前客户端配置保存剧情、地图与永久收集记录。在线操作会先备份并保存退出，写入后自动重新启动。'}
    def model(self,player,request):
        token=secrets.token_hex(6);inp=self.runtime/(token+'-in.json');out=self.runtime/(token+'-out.json')
        write_json(inp,dict(request,Player=player));logpath=self.runtime/'model.log'
        try:
            with logpath.open('w',encoding='utf8') as log:
                p=subprocess.run([str(self.home/'localgm.exe'),'--config',str(self.profile/'config.json'),'--input',str(inp),'--output',str(out)],stdout=log,stderr=log,cwd=self.home,creationflags=NO_WINDOW,timeout=120)
            if p.returncode:raise ValueError('账号模型验证失败：'+logpath.read_text(encoding='utf8',errors='replace')[-1500:])
            return read_json(out)
        finally:inp.unlink(missing_ok=True);out.unlink(missing_ok=True)
    def mutate(self,uid,request):
        if not 1<=uid<=4294967295:raise ValueError('UID 必须为 1..4294967295')
        if self.client_processes():raise ValueError('请先点击「保存并退出单机」，再编辑账号。')
        if port_open(18080):self.world('Stop')
        basic,player=self.load(uid);op=request.get('operation','init')
        result=self.model(player,dict(Operation=op,ID=int(request.get('id',0)),Count=int(request.get('count',9999999)),Level=int(request.get('level',1)),Star=int(request.get('star',0)),SkillLevel=int(request.get('skillLevel',1)),WorldLevel=int(request.get('worldLevel',0)),Team=request.get('team',[])))
        name=str(request.get('name') or basic.get('nick_name') or ('Local'+str(uid))).strip()
        if not 1<=len(name)<=32:raise ValueError('昵称长度须为 1..32 个字符')
        level=int(request.get('accountLevel',basic.get('level',1)));exp=int(request.get('exp',basic.get('exp',0)))
        maxlevel=self.catalog()['maxLevel']
        if not 1<=level<=maxlevel:raise ValueError('账号等级范围：1..'+str(maxlevel))
        if not 0<=exp<=4294967295:raise ValueError('经验值范围错误')
        backup=self.backup('before-'+op)
        data=gzip.compress(json.dumps(result['Player'],ensure_ascii=False,separators=(',',':')).encode(),mtime=0)
        now=dt.datetime.now().isoformat(sep=' ')
        with closing(sqlite3.connect(self.db,timeout=10)) as c,c:
            conflict=c.execute('SELECT user_id FROM of_user WHERE sdk_uid=? AND user_id<>?',(uid,uid)).fetchone()
            if conflict:raise ValueError('此 UID 已用作另一个账号的 SDK UID')
            c.execute('INSERT OR IGNORE INTO of_user(user_id,sdk_uid,ban) VALUES(?,?,0)',(uid,uid))
            c.execute('INSERT OR IGNORE INTO of_game_basic(user_id,nick_name) VALUES(?,?)',(uid,name))
            c.execute('UPDATE of_game_basic SET nick_name=?,level=?,exp=?,sign=?,updated_at=? WHERE user_id=?',(name,level,exp,str(request.get('sign',basic.get('sign') or '本地单机存档')),now,uid))
            c.execute('INSERT INTO of_game(user_id,bin_data,created_at,updated_at) VALUES(?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET bin_data=excluded.bin_data,updated_at=excluded.updated_at',(uid,data,now,now))
            if c.execute('PRAGMA integrity_check').fetchone()[0]!='ok':raise ValueError('SQLite 完整性检查失败')
        setting=self.settings();setting['uid']=str(uid);setting['mode']='solo';write_json(self.portable/'launcher.settings.json',setting)
        audit={'time':now,'uid':uid,'operation':op,'summary':result['Summary'],'progress':result.get('Progress',{}),'applied':result['Applied'],'skipped':result['Skipped'],'unavailable':result.get('Unavailable',[]),'inventoryRepair':result.get('InventoryRepair',{}),'unlocks':result.get('Unlocks',{}),'backup':backup}
        with (self.home/'操作记录.jsonl').open('a',encoding='utf8') as f:f.write(json.dumps(audit,ensure_ascii=False)+'\n')
        return dict(audit,message='存档已保存。重新启动游戏后生效。'+(' 当前版本的未发布/不完整配置已排除 '+str(len(audit['unavailable']))+' 项。' if audit['unavailable'] else ''),account=self.summary(uid))
    def restore(self,name):
        if self.client_processes() or port_open(18080):raise ValueError('恢复前请保存并退出单机')
        if name!=Path(name).name or not name.endswith('.db'):raise ValueError('无效备份名称')
        src=self.backups/name
        with closing(sqlite3.connect(src.resolve().as_uri()+'?mode=ro',uri=True)) as origin:
            if origin.execute('PRAGMA integrity_check').fetchone()[0]!='ok':raise ValueError('备份已损坏')
            self.backup('before-restore')
            with closing(sqlite3.connect(self.db)) as dest:origin.backup(dest)
        return {'message':'备份已恢复。'}
    def commands(self):
        data=self.local_request(18400,'/commands')
        for cmd in data['result']:
            name=cmd['command'].lower()
            cmd['support']='客户端指令 · 按游戏条件生效' if not cmd.get('NeedSendToServer') else ('本地服务端已实现' if name in SUPPORTED_GM else '当前版本固定解锁' if name in FIXED_GM else '服务端尚未实现')
        return data
    def apply_shop_catalog(self,data):
        def normalize(obj):
            if isinstance(obj,dict):return {('Datas' if k=='datas' else k):normalize(v) for k,v in obj.items()}
            if isinstance(obj,list):return [normalize(v) for v in obj]
            return obj
        data=normalize(data)
        for name in ('Info','Grid','Pool'):
            if not isinstance(data.get(name,{}).get('Datas'),list) or not data[name]['Datas']:
                raise ValueError('客户端商店配置不完整：'+name)
        if port_open(18080) or self.client_processes():raise ValueError('写入配置前请保存并退出单机')
        target=self.profile/'Resource/Excel/Shop.json'
        folder=self.home/'配置备份';folder.mkdir(exist_ok=True)
        backup=folder/(dt.datetime.now().strftime('%Y%m%d-%H%M%S-%f')+'-Shop.json')
        shutil.copy2(target,backup);write_json(target,data)
        return {'message':'商店配置已与当前 Steam 客户端同步。请重新启动单机。','pools':len(data['Pool']['Datas'])}
    def sync_shop(self):
        data=self.local_request(18400,'/shop-catalog')['result']
        self.stop()
        return self.apply_shop_catalog(data)
    def apply_progress(self,uid,mode):
        if mode not in ('story','maps','collections','all','unlocks'):raise ValueError('未知解锁范围')
        if uid!=self.uid() and (self.client_processes() or port_open(18080)):raise ValueError('在线时只能操作当前账号')
        running=bool(self.client_processes())
        if running or port_open(18080):
            self.action({'action':'backup','uid':uid})
            self.stop()
        result=self.mutate(uid,{'operation':'unlock-all' if mode=='unlocks' else 'progress-'+mode})
        result['message']='解锁记录已保存，操作前存档已备份。'
        if mode=='unlocks':
            u=result.get('unlocks',{});result['message']+=' 动作 '+str(u.get('actions',0))+' 个，星云树能力 '+str(u.get('abilities',0))+' 项，采集工具槽已补齐；祈愿树节点 '+str(u.get('blessNodes',0))+' 个；剧情、地图、永久收集和已拥有服装的染色方案已一并解锁。'
        if running:
            try:
                self.action({'action':'start','uid':uid})
                result['message']+=' 游戏已重新启动。'
            except Exception as exc:result['message']+=' 自动启动失败，请点击启动单机游戏：'+str(exc)
        return result
    def weapons(self,uid):
        if not 1<=uid<=4294967295:raise ValueError('无效账号 UID')
        if port_open(18080):
            data=self.local_request(18080,'/local-admin/weapons/'+str(uid))
        else:
            _,player=self.load(uid)
            if not player:raise ValueError('此账号没有存档')
            data={'weapons':self.model(player,{'Operation':'weapons'})['Weapons'],'uid':uid,'online':False}
        names={x['id']:x['name'] for x in self.catalog()['items']}
        chars={x['id']:x['name'] for x in self.catalog()['characters']}
        for w in data['weapons']:
            w['name']=names.get(w['weaponId'],str(w['weaponId']))
            w['wearerName']=chars.get(w['wearerId'],str(w['wearerId'])) if w['wearerId'] else ''
        return data
    def edit_weapon(self,uid,body):
        if not 1<=uid<=4294967295:raise ValueError('无效账号 UID')
        fields={'level','attack','damageBalance','criticalRatio','strengthLevel','resetStats'}
        changes=body.get('changes')
        if not isinstance(changes,dict) or not changes or not set(changes)<=fields:raise ValueError('武器修改字段无效')
        for name,value in changes.items():
            if type(value) not in (int,float) or not math.isfinite(value) or not 0<=value<=4294967295:raise ValueError('武器数值必须是有限的非负数')
            if name in ('level','attack','strengthLevel') and value!=int(value):raise ValueError('等级和攻击力必须是整数')
        for key in ('instanceId','weaponId'):
            if type(body.get(key)) is not int or not 1<=body[key]<=4294967295:raise ValueError('武器实例或 ID 无效')
        if not isinstance(body.get('revision'),str) or not re.fullmatch(r'[a-f0-9]{64}',body['revision']):raise ValueError('请先读取武器再保存')
        req={k:body[k] for k in ('instanceId','weaponId','revision','changes')}
        if port_open(18080):
            active=read_json(self.portable/'runtime/world-state.json',{}) or {}
            if uid!=int(active.get('Uid') or self.uid()):raise ValueError('在线时只能编辑当前游戏账号')
            if not self.snapshot(uid):raise ValueError('请等待账号进入游戏场景后再保存武器')
            backup=self.action({'action':'backup','uid':uid})['backup']
            result=self.local_request(18080,'/local-admin/weapon/'+str(uid),req,timeout=30)
            try:
                sync=self.local_request(18400,'/weapons/sync',{})['result'];result['sync']=sync
                if sync.get('lastError') or sync.get('pending'):raise ValueError(sync.get('lastError') or '武器模型仍在加载')
                result['message']='武器已保存并同步。已装备武器重新装备后刷新战斗属性。'
            except Exception as exc:result['message']='武器已保存，客户端同步暂未完成，将自动重试；重新进入游戏也会恢复。'+str(exc)
        else:
            if self.client_processes():raise ValueError('游戏服务尚未就绪，请稍后重试')
            _,player=self.load(uid)
            if not player:raise ValueError('此账号没有存档')
            modeled=self.model(player,{'Operation':'weapon-edit','Weapon':req})
            backup=self.backup('before-weapon-edit')
            blob=gzip.compress(json.dumps(modeled['Player'],ensure_ascii=False,separators=(',',':')).encode(),mtime=0)
            with closing(sqlite3.connect(self.db,timeout=10)) as c,c:
                c.execute('BEGIN IMMEDIATE')
                current=c.execute('SELECT bin_data FROM of_game WHERE user_id=?',(uid,)).fetchone()
                if port_open(18080) or not current or json.loads(gzip.decompress(current[0]))!=player:raise ValueError('存档已发生变化，请重新读取武器')
                c.execute('UPDATE of_game SET bin_data=?,updated_at=? WHERE user_id=?',(blob,dt.datetime.now().isoformat(sep=' '),uid))
            result={'uid':uid,'weapon':next(w for w in modeled['Weapons'] if w['instanceId']==req['instanceId']),'saved':True,'online':False,'message':'武器已保存。下次进入游戏时生效。'}
        result['backup']=backup
        audit={'time':dt.datetime.now().isoformat(),'uid':uid,'operation':'weapon-edit','instanceId':req['instanceId'],'weaponId':req['weaponId'],'changes':changes,'online':result['online'],'backup':backup}
        with (self.home/'操作记录.jsonl').open('a',encoding='utf8') as f:f.write(json.dumps(audit,ensure_ascii=False)+'\n')
        return result
    def action(self,body):
        action=body['action'];uid=int(body.get('uid',self.uid()))
        if action=='weapon-edit':return self.edit_weapon(uid,body)
        if action=='progress':return self.apply_progress(uid,body.get('mode',''))
        if action=='mutate':return self.mutate(uid,body)
        if action=='start':
            if not self.client_processes() and not port_open(18080):
                self.mutate(uid,{'operation':'repair-abilities'})
            elif not self.load(uid)[0].get('nick_name'):self.mutate(uid,{'operation':'init'})
            return {'message':self.world('Start',True,uid)}
        if action=='stop':return self.stop()
        if action=='backup':
            snap=self.snapshot(uid)
            if snap:
                name=self.backup('manual-live');dest=self.backups/name
                with closing(sqlite3.connect(dest)) as c,c:c.execute('UPDATE of_game SET bin_data=? WHERE user_id=?',(gzip.compress(json.dumps(snap['player'],ensure_ascii=False).encode(),mtime=0),uid))
            else:name=self.backup()
            return {'message':'备份完成：'+name,'backup':name}
        if action=='restore':return self.restore(body['name'])
        if action=='sync-shop':return self.sync_shop()
        if action=='runtime':
            if body['endpoint'] not in ('/status','/execute','/open-gm','/speed','/hide-errors','/attributes','/attributes/reset'):raise ValueError('未知运行时操作')
            return self.local_request(18400,body['endpoint'],body.get('data'))
        raise ValueError('未知操作')

def serve(open_browser=False):
    manager=Manager();tokenpath=manager.runtime/'panel-token.txt'
    if port_open(PORT):
        if open_browser:webbrowser.open('http://127.0.0.1:'+str(PORT))
        return
    token=secrets.token_urlsafe(32);tokenpath.write_text(token,encoding='utf8')
    class Handler(BaseHTTPRequestHandler):
        def log_message(self,*args):pass
        def reply(self,value,code=200):
            data=json.dumps(value,ensure_ascii=False).encode();self.send_response(code);self.send_header('Content-Type','application/json; charset=utf-8');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
        def do_GET(self):
            if self.headers.get('Host') not in ('127.0.0.1:'+str(PORT),'localhost:'+str(PORT)):
                self.send_error(403);return
            try:
                if self.path=='/':
                    data=(BASE/'index.html').read_text(encoding='utf8').replace('__TOKEN__',token).encode();self.send_response(200);self.send_header('Content-Type','text/html; charset=utf-8');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
                elif self.path.startswith('/api/status'):
                    from urllib.parse import urlparse,parse_qs
                    query=parse_qs(urlparse(self.path).query);self.reply(manager.status(int(query.get('uid',[manager.uid()])[0])))
                elif self.path=='/weapon_editor.js':
                    data=(BASE/'weapon_editor.js').read_bytes();self.send_response(200);self.send_header('Content-Type','text/javascript; charset=utf-8');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
                elif self.path.startswith('/api/weapons?'):
                    from urllib.parse import urlparse,parse_qs
                    uid=int(parse_qs(urlparse(self.path).query).get('uid',[manager.uid()])[0])
                    with LOCK:data=manager.weapons(uid)
                    self.reply(data)
                elif self.path=='/api/catalog':self.reply(manager.catalog())
                elif self.path=='/api/commands':self.reply(manager.commands())
                elif self.path=='/api/attributes':self.reply(manager.local_request(18400,'/attributes'))
                else:self.send_error(404)
            except Exception as e:self.reply({'error':str(e)},400)
        def do_POST(self):
            if self.path!='/api/action' or not hmac.compare_digest(self.headers.get('X-Panel-Token',''),token):self.send_error(403);return
            if self.headers.get('Origin') not in (None,'http://127.0.0.1:'+str(PORT),'http://localhost:'+str(PORT)):self.send_error(403);return
            try:
                size=int(self.headers.get('Content-Length','0'))
                if not 0<size<100000:raise ValueError('请求大小无效')
                body=json.loads(self.rfile.read(size))
                with LOCK:result=manager.action(body)
                self.reply({'ok':True,'result':result})
            except Exception as e:self.reply({'ok':False,'error':str(e)},400)
    server=ThreadingHTTPServer(('127.0.0.1',PORT),Handler)
    if open_browser:threading.Timer(.4,lambda:webbrowser.open('http://127.0.0.1:'+str(PORT))).start()
    server.serve_forever()

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--open',action='store_true');args=parser.parse_args();serve(args.open)
