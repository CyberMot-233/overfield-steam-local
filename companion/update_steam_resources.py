"""Check/download the official Steam resource update into the isolated client.
All replacements are atomic so hard links never modify the original Steam files.
"""
import argparse, concurrent.futures, datetime, hashlib, io, json, re, struct, urllib.parse, urllib.request, zlib
from pathlib import Path
from manager import Manager, write_json

DISPATCH='http://dsp-global-of.inutan.com:18881/dispatch/client_hot_update'
STAMP=re.compile(r'\d{4}(?:-\d{2}){5}')

def fetch(url,data=None):
    with urllib.request.urlopen(urllib.request.Request(url,data=data),timeout=45) as r:
        return r.read()

def parse_manifest(raw):
    stream=io.BytesIO(raw)
    def val(fmt):
        size=struct.calcsize('<'+fmt);data=stream.read(size)
        if len(data)!=size:raise ValueError('truncated manifest')
        return struct.unpack('<'+fmt,data)[0]
    def text():return stream.read(val('H')).decode('utf8')
    def texts():return [text() for _ in range(val('H'))]
    def ints():return [val('i') for _ in range(val('H'))]
    val('I');schema=text();flags=[val('?') for _ in range(3)]
    style=val('i');pipeline=text();package=text();version=text()
    if schema!='2.0.0' or package!='DefaultPackage' or style!=0:raise ValueError('unsupported resource manifest')
    count=val('i')
    if not 0<count<1000000:raise ValueError('invalid asset count')
    assets=[dict(address=text(),path=text(),guid=text(),tags=texts(),bundle=val('i')) for _ in range(count)]
    count=val('i')
    if not 0<count<100000:raise ValueError('invalid bundle count')
    bundles=[dict(id=i,name=text(),unitycrc=val('I'),hash=text(),crc=text(),size=val('q'),encrypted=val('?'),tags=texts(),deps=ints()) for i in range(count)]
    if stream.tell()!=len(raw):raise ValueError('unexpected manifest trailer')
    for b in bundles:
        if not re.fullmatch('[0-9a-f]{32}',b['hash']) or not re.fullmatch('[0-9a-f]{8}',b['crc']) or b['size']<0:raise ValueError('invalid bundle')
    return dict(schema=schema,flags=flags,style=style,pipeline=pipeline,package=package,version=version,assets=assets,bundles=bundles)

def atomic(path,data):
    path.parent.mkdir(parents=True,exist_ok=True)
    tmp=path.with_name(path.name+'.local-update');tmp.write_bytes(data);tmp.replace(path)

def crc(raw):return struct.pack('<I',zlib.crc32(raw)&0xffffffff).hex()

def bundle_path(client,b):
    cache=client/'yoo/DefaultPackage/CacheFiles'/b['hash'][:2]/b['hash']/'__data'
    builtin=client/'StreamingAssets/yoo/DefaultPackage'/(b['hash']+'.bundle')
    for path in (cache,builtin):
        if path.is_file() and path.stat().st_size==b['size']:return path
    return None

def asset_catalog(client,manifest,characters):
    bypath={x['path'].lower():x for x in manifest['assets']};result={}
    def available(location):
        entry=bypath.get(('Assets/ResBundle/'+location+'.prefab').lower())
        if not entry:return False
        bundle=manifest['bundles'][entry['bundle']]
        return all(bundle_path(client,manifest['bundles'][i]) for i in [entry['bundle']]+bundle['deps'])
    for character in characters:
        result[str(character['ID'])]=bool(character.get('AssetFullName') and character.get('FacePath') and available(character['AssetFullName']) and available(character['FacePath']))
    return {'resourceVersion':manifest['version'],'characterAssets':result}

def update(apply=False):
    m=Manager();client=m.profile/'client/launcher_Data'
    program=(client/'StreamingAssets/yoo/DefaultPackage/PackageManifest_DefaultPackage.version').read_text(encoding='utf-8-sig').strip()
    folder=client/'yoo/DefaultPackage/ManifestFiles';versionfile=folder/'PackageManifest_DefaultPackage.version'
    old=versionfile.read_text(encoding='utf-8-sig').strip()
    if not STAMP.fullmatch(program):raise ValueError('invalid program version')
    request=urllib.parse.urlencode({'version':program,'version2':program+'_'+old,'accountType':16666,'os':'WindowsPlayer'}).encode()
    info=json.loads(fetch(DISPATCH,request));info.pop('ipAddress',None)
    full=info.get('currentVersion','');parts=full.split('_')
    if len(parts)!=2 or parts[0]!=program or not STAMP.fullmatch(parts[1]):raise ValueError('official update requires a different Steam program build')
    new=parts[1];cdn=next((s for s in info['hotOssUrl'].split(';') if s.startswith('https://')),None)
    if not cdn:raise ValueError('official update has no HTTPS CDN')
    host=cdn.rstrip('/')+'/'+program+'/';name='PackageManifest_DefaultPackage_'+new
    expected=fetch(host+name+'.hash').decode().strip();raw=fetch(host+name+'.bytes')
    if hashlib.md5(raw).hexdigest()!=expected:raise ValueError('manifest checksum failed')
    manifest=parse_manifest(raw)
    if manifest['version']!=new:raise ValueError('resource version mismatch')
    missing=[b for b in manifest['bundles'] if bundle_path(client,b) is None]
    report={'checkedAt':datetime.datetime.now().isoformat(),'programVersion':program,'previous':old,'resourceVersion':new,'manifestMD5':expected,'downloadCount':len(missing),'downloadBytes':sum(b['size'] for b in missing),'applied':False}
    if apply:
        if m.client_processes():raise ValueError('请先保存并退出单机')
        backup=m.home/'配置备份'/('steam-update-'+datetime.datetime.now().strftime('%Y%m%d-%H%M%S'))
        backup.mkdir(parents=True,exist_ok=True);(backup/'resource.version').write_bytes(versionfile.read_bytes());write_json(backup/'official-update.json',info)
        def download(b):
            data=fetch(host+b['hash']+'.bundle')
            if len(data)!=b['size'] or crc(data)!=b['crc']:raise ValueError('bundle verification failed: '+b['hash'])
            dest=client/'yoo/DefaultPackage/CacheFiles'/b['hash'][:2]/b['hash']
            atomic(dest/'__data',data);atomic(dest/'__info',struct.pack('<H',8)+b['crc'].encode()+struct.pack('<q',b['size']))
            return b['hash']
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:list(pool.map(download,missing))
        atomic(folder/(name+'.bytes'),raw);atomic(folder/(name+'.hash'),expected.encode());atomic(versionfile,new.encode())
        mapping=json.loads((m.profile/'data/ClientVersion.json').read_text(encoding='utf-8-sig'));mapping['VersionMap'][program]=full;mapping['HotOssUrl']=info['hotOssUrl'];write_json(m.profile/'data/ClientVersion.json',mapping)
        characters=json.loads((m.profile/'Resource/Excel/Character.json').read_text(encoding='utf-8-sig'))['Character']['Datas']
        write_json(m.profile/'Resource/Config/LocalClientAssets.json',asset_catalog(client,manifest,characters))
        report.update(applied=True,backup=str(backup))
    write_json(m.home/'验证/steam-update-check.json',report)
    return report

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--apply',action='store_true');args=parser.parse_args()
    print(json.dumps(update(args.apply),ensure_ascii=False,indent=2))
