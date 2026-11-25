"""Exercise the actual local panel in an isolated headless Edge profile."""
import json,os
from pathlib import Path
from playwright.sync_api import sync_playwright,expect
from manager import Manager,BASE

m=Manager();report={};errors=[]
with sync_playwright() as p:
    browser=p.chromium.launch(executable_path=str(Path(os.environ['ProgramFiles(x86)'])/'Microsoft/Edge/Application/msedge.exe'),headless=True)
    page=browser.new_page(viewport={'width':1440,'height':1080},device_scale_factor=1)
    page.on('pageerror',lambda error:errors.append(str(error)))
    page.goto('http://127.0.0.1:18450',wait_until='networkidle')
    expect(page.locator('#title')).to_have_text(m.summary(m.uid())['name'])
    expect(page.locator('#counts .card')).to_have_count(6)
    report['account']=True
    page.screenshot(path=str(BASE/'验证/管理面板-账号.png'),full_page=True)
    page.get_by_role('button',name='资源与衣橱',exact=True).click()
    page.locator('#itemSearch').fill('101')
    expect(page.locator('#itemRows')).to_contain_text('金币')
    report['catalogSearch']=True
    page.get_by_role('button',name='角色与队伍',exact=True).click()
    expect(page.locator('#character option')).to_have_count(len(m.catalog()['characters']))
    report['characters']=True
    page.get_by_role('button',name='开发者工具',exact=True).click()
    page.get_by_role('button',name='读取全部已注册指令',exact=True).click()
    expect(page.locator('#commands [data-command]')).to_have_count(124,timeout=20000)
    expect(page.locator('#commands')).to_contain_text('服务端尚未实现')
    expect(page.locator('#commands')).to_contain_text('本地服务端已实现')
    page.locator('#speed').fill('1.5')
    page.get_by_role('button',name='应用倍率',exact=True).click()
    expect(page.locator('#message')).to_contain_text('1.5',timeout=20000)
    page.get_by_role('button',name='恢复 1 倍',exact=True).click()
    expect(page.locator('#message')).to_contain_text('timeScale',timeout=20000)
    report['runtimeSpeed']=m.local_request(18400,'/speed',{'value':1})['result']['timeScale']==1
    page.screenshot(path=str(BASE/'验证/管理面板-开发者.png'),full_page=True)
    page.get_by_role('button',name='存档与恢复',exact=True).click()
    page.get_by_role('button',name='备份当前存档',exact=True).click()
    expect(page.locator('#message')).to_contain_text('备份完成',timeout=20000)
    report['liveBackup']=True
    denied=page.request.post('http://127.0.0.1:18450/api/action',data={'action':'stop'})
    report['unauthenticatedWriteRejected']=denied.status==403
    report['javascriptErrors']=errors
    browser.close()
assert not errors,errors
assert all(v is True for k,v in report.items() if k!='javascriptErrors'),report
(BASE/'验证/panel-tests.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf8')
print(json.dumps(report,ensure_ascii=False,indent=2))
