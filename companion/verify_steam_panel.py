import json,os
from pathlib import Path
from playwright.sync_api import sync_playwright,expect
from manager import Manager,write_json
m=Manager();errors=[];report={}
with sync_playwright() as p:
 browser=p.chromium.launch(executable_path=str(Path(os.environ['ProgramFiles(x86)'])/'Microsoft/Edge/Application/msedge.exe'),headless=True)
 page=browser.new_page(viewport={'width':1440,'height':1080})
 page.on('pageerror',lambda e:errors.append(str(e)))
 page.goto('http://127.0.0.1:18450',wait_until='networkidle')
 expect(page.locator('#resourceStatus')).to_contain_text('2026-11-24-18-31-00')
 expect(page.locator('#resourceStatus')).to_contain_text('17 个卡池')
 report['versionAndLocalPoolLabel']=True
 page.get_by_role('button',name='角色与队伍',exact=True).click()
 expect(page.locator('#character option')).to_have_count(len(m.catalog()['characters']))
 expect(page.locator('#character')).to_contain_text('米莉亚')
 expect(page.locator('#character')).to_contain_text('朱寅')
 report['newCharactersVisible']=True
 page.get_by_role('button',name='资源与衣橱',exact=True).click()
 page.locator('#itemSearch').fill('302003')
 expect(page.locator('[data-give="302003"]')).to_be_enabled()
 report['newCharacterGrantEnabled']=True
 page.get_by_role('button',name='账号与启动',exact=True).click()
 page.screenshot(path=str(m.home/'验证/管理面板-Steam更新.png'),full_page=True)
 report['javascriptErrors']=errors;assert not errors
 browser.close()
write_json(m.home/'验证/steam-panel-tests.json',report);print(json.dumps(report,ensure_ascii=False))
