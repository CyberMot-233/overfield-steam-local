"""Experimental, version-locked isolated Steam private-client runner.

Never attach to a running official client. Only spawn a source-fingerprinted copy
under profiles/<mode>/client. Fail closed if instrumentation cannot install.
"""
import argparse
import hashlib
import ipaddress
import json
import threading
import time
from pathlib import Path

SUPPORTED_GAME_ASSEMBLY = '4787fc3abaf911890fd6bccae6d0f85904f7c959a5d14c55f5dc56b75fe82cab'


def validated_config(client, host, port, uid):
    from client_identity import classify
    client = Path(client).resolve()
    if 'profiles' not in client.parts or client.name != 'client':
        raise ValueError('Adapter must run an isolated profiles/<mode>/client copy.')
    if classify(client) != 'steam-pc':
        raise ValueError('This adapter supports verified Steam PC layout only.')
    address = ipaddress.IPv4Address(host)
    if not (address.is_loopback or any(address in ipaddress.IPv4Network(n)
                                     for n in ('10.0.0.0/8','172.16.0.0/12','192.168.0.0/16'))):
        raise ValueError('Only loopback or private LAN server addresses are permitted.')
    if not 1 <= int(port) <= 65535 or not 1 <= int(uid) <= 0xffffffff:
        raise ValueError('Invalid local port or UID.')
    digest = hashlib.sha256((client/'GameAssembly.dll').read_bytes()).hexdigest()
    if digest != SUPPORTED_GAME_ASSEMBLY:
        raise ValueError('Unverified GameAssembly version; refuse to apply this experimental adapter.')
    config = {'host':str(address),'port':int(port),'uid':int(uid)}
    local_file = client.parent / 'Resource/Config/LocalGacha.json'
    if local_file.is_file():
        from datetime import datetime
        local = json.loads(local_file.read_text(encoding='utf-8-sig'))
        if local.get('mode') != 'all-available':
            raise ValueError('Unsupported local gacha mode')
        ids = local.get('poolIds')
        if not isinstance(ids,list) or not ids or len(ids)>200 or any(type(x) is not int or not 0<x<2147483647 for x in ids) or len(set(ids)) != len(ids):
            raise ValueError('Invalid local gacha pool list')
        times = [datetime.strptime(local[k], '%Y-%m-%d %H:%M:%S') for k in ('openingTime','closingTime')]
        if times[0] >= times[1]:
            raise ValueError('Invalid local gacha date range')
        config['localGacha'] = {k:local[k] for k in ('mode','poolIds','openingTime','closingTime','resourceVersion')}
    return config


def agent_source(config):
    text=Path(__file__).with_name('steam_private_agent.js').read_text(encoding='utf-8')
    return text.replace('__LOCAL_CONFIGURATION__',json.dumps(config),1) + '\n' + Path(__file__).with_name('steam_live_tools.js').read_text(encoding='utf-8') + '\n' + Path(__file__).with_name('steam_local_compat.js').read_text(encoding='utf-8') + '\n' + Path(__file__).with_name('steam_runtime_agent.js').read_text(encoding='utf-8')


def start_runtime_bridge(script, client, port, pid, uid=None, stop=None):
    import hmac
    from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
    key = (client.parent / 'admin-key.txt').read_text(encoding='utf-8').strip()
    from urllib.request import Request, build_opener, ProxyHandler
    opener=build_opener(ProxyHandler({}))
    def sync_weapons():
        if uid is None:return {'updated':0}
        req=Request('http://127.0.0.1:'+str(port)+'/local-admin/client-state/'+str(uid),headers={'X-Local-Token':key})
        with opener.open(req,timeout=5) as response:data=json.load(response)
        if data.get('uid')!=uid:raise ValueError('Local weapon account mismatch')
        return script.exports_sync.syncweapons(data)
    sync_status={'clientStateSynced':False,'clientStateError':''}
    def weapon_worker():
        while not stop.is_set():
            try:
                result=sync_weapons()
                sync_status.update(clientStateSynced=not bool(result.get('lastError')),clientStateError=result.get('lastError',''))
            except Exception as exc:
                sync_status.update(clientStateSynced=False,clientStateError=str(exc))
            stop.wait(2)
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args): pass
        def handle_request(self):
            if not hmac.compare_digest(self.headers.get('X-Local-Token',''), key):
                self.send_error(403); return
            try:
                body = {}
                if self.command == 'POST':
                    size = int(self.headers.get('Content-Length','0'))
                    if not 0 <= size <= 8192: raise ValueError('Request too large')
                    body = json.loads(self.rfile.read(size) or b'{}')
                api=script.exports_sync
                if self.path == '/status' and self.command == 'GET': result=dict(api.status(),**sync_status)
                elif self.path.startswith('/config-catalog/') and self.command == 'GET': result=api.configcatalog(self.path.rsplit('/',1)[-1])
                elif self.path == '/shop-catalog' and self.command == 'GET': result=api.shopcatalog()
                elif self.path == '/enter-game' and self.command == 'POST': result=api.entergame()
                elif self.path == '/hide-errors' and self.command == 'POST': result=api.hideerrors()
                elif self.path == '/commands' and self.command == 'GET': result=api.commands()
                elif self.path == '/execute' and self.command == 'POST': result=api.execute(body['command'])
                elif self.path == '/open-gm' and self.command == 'POST': result=api.opengm()
                elif self.path == '/attributes' and self.command == 'GET': result=api.attributes()
                elif self.path == '/attributes' and self.command == 'POST': result=api.applyattributes(body['values'])
                elif self.path == '/weapons/sync' and self.command == 'POST': result=sync_weapons()
                elif self.path == '/attributes/reset' and self.command == 'POST': result=api.resetattributes(body.get('key','all'))
                elif self.path == '/speed' and self.command == 'POST': result=api.speed(body['value'])
                else: self.send_error(404); return
                output={'ok':True,'result':result};code=200
            except Exception as exc: output={'ok':False,'error':str(exc)};code=400
            data=json.dumps(output,ensure_ascii=False).encode('utf-8')
            self.send_response(code);self.send_header('Content-Type','application/json; charset=utf-8');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
        do_GET=handle_request
        do_POST=handle_request
    server=ThreadingHTTPServer(('127.0.0.1',18400 if port==18080 else 19400),Handler)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    if uid is not None and stop is not None:threading.Thread(target=weapon_worker,daemon=True).start()
    return server


def run(client,host,port,uid,log_path,result_path=None):
    config=validated_config(client,host,port,uid)
    import frida
    client=Path(client).resolve()
    device=frida.get_local_device()
    ready=threading.Event(); done=threading.Event(); failed=[]
    log=Path(log_path).resolve();log.parent.mkdir(parents=True,exist_ok=True)
    def message(payload,data):
        event=payload.get('payload',{}) if payload.get('type')=='send' else {'event':'adapter-error','message':payload.get('description','Agent error')}
        with log.open('a',encoding='utf-8') as out:out.write(json.dumps(event,ensure_ascii=False)+'\n')
        if event.get('event')=='adapter-ready': ready.set()
        if event.get('event') in ('adapter-error','runtime-tools-error'):failed.append(event);done.set()
    pid=device.spawn([str(client/'launcher.exe'),'-logFile',str(log.with_name('Player.log'))],cwd=str(client))
    try:
        session=device.attach(pid)
        session.on('detached',lambda *args:done.set())
        script=session.create_script(agent_source(config))
        script.on('message',message);script.load();device.resume(pid)
        start=time.monotonic()
        while not ready.is_set() and not done.is_set() and time.monotonic()-start<40:done.wait(.1)
        if not ready.is_set() or failed:
            raise RuntimeError('Steam adapter failed before ready; terminating private test client.')
        bridge=start_runtime_bridge(script,client,port,pid,uid,done)
        result={'pid':pid,'adapter':'steam-local','ready':True,'runtimePort':bridge.server_port}
        if result_path:
            from profile_manager import write_json
            write_json(Path(result_path),result)
        print(json.dumps(result),flush=True)
        done.wait()
        if failed:raise RuntimeError('Private adapter reported an error; terminating client.')
    finally:
        if 'bridge' in locals(): bridge.shutdown(); bridge.server_close()
        try:device.kill(pid)
        except Exception:pass


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--client',type=Path,required=True)
    p.add_argument('--host',default='127.0.0.1')
    p.add_argument('--port',type=int,default=18080)
    p.add_argument('--uid',type=int,default=1000000)
    p.add_argument('--log',type=Path,required=True)
    p.add_argument('--result-path',type=Path)
    a=p.parse_args();run(a.client,a.host,a.port,a.uid,a.log,a.result_path)
