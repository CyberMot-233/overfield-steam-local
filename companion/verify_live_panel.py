import json,os
from pathlib import Path
from playwright.sync_api import sync_playwright,expect
from manager import Manager,BASE,write_json
m=Manager();report={};errors=[]
try:
 with sync_playwright() as p:
  browser=p.chromium.launch(executable_path=str(Path(os.environ['ProgramFiles(x86)'])/'Microsoft/Edge/Application/msedge.exe'),headless=True)
  page=browser.new_page(viewport={'width':1600,'height':1200},device_scale_factor=1)
  page.on('pageerror',lambda e:errors.append(str(e)))
  page.goto('http://127.0.0.1:18450',wait_until='networkidle')
  page.get_by_role('button',name='实时属性',exact=True).click()
  expect(page.locator('#liveState')).to_contain_text('实时连接',timeout=15000)
  expect(page.locator('#liveRows tr')).to_have_count(9)
  page.locator('#liveInput-moveSpeed').fill('6.8');page.locator('#liveInput-moveSpeed').press('Tab')
  expect(page.locator('#liveValue-moveSpeed')).to_have_text('6.8',timeout=15000)
  assert abs(m.local_request(18400,'/attributes')['result']['values']['moveSpeed']-6.8)<1e-4
  report['autoApplyNumber']=True
  page.locator('#liveRange-moveSpeed').evaluate("e=>{e.value='7.2';e.dispatchEvent(new Event('input',{bubbles:true}))}")
  expect(page.locator('#liveValue-moveSpeed')).to_have_text('7.2',timeout=15000)
  report['slider']=True
  page.locator('#liveInput-attack').fill('123456');page.locator('#liveInput-attack').press('Tab')
  expect(page.locator('#liveValue-attack')).to_have_text('123,456',timeout=15000)
  report['attack']=True
  page.locator('#liveInput-hp').fill('5000');page.locator('#liveInput-hp').press('Tab')
  expect(page.locator('#liveValue-hp')).to_have_text('5,000',timeout=15000)
  assert m.local_request(18400,'/attributes')['result']['hp']==5000
  report['currentHp']=True
  page.get_by_role('button',name='恢复满血',exact=True).click()
  expect(page.locator('#liveMessage')).to_contain_text('当前生命',timeout=15000)
  page.locator('#liveInput-moveSpeed').fill('999999');page.locator('#liveInput-moveSpeed').press('Tab')
  expect(page.locator('#message')).to_contain_text('范围')
  assert abs(m.local_request(18400,'/attributes')['result']['values']['moveSpeed']-7.2)<1e-4
  report['invalidRejected']=True
  page.get_by_role('button',name='全部恢复默认 / 正常速度',exact=True).click()
  expect(page.locator('#liveMessage')).to_contain_text('已恢复正常倍率',timeout=15000)
  d=m.local_request(18400,'/attributes')['result'];assert d['values']==d['defaults'] and d['timeScale']==1
  report['reset']=True
  page.screenshot(path=str(BASE/'验证/管理面板-实时属性.png'),full_page=True)
  report['javascriptErrors']=errors
  assert not errors,errors
  browser.close()
finally:
 m.local_request(18400,'/attributes/reset',{'key':'all'})
 write_json(BASE/'验证/live-panel-tests.json',report)
print(json.dumps(report,ensure_ascii=False))
