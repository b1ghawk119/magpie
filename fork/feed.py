"""Publish all channel pointers in a single fast-forward Git commit."""
import json, os, subprocess, tempfile
from pathlib import Path
from urllib.request import urlopen
from packaging.version import Version

repo=os.environ['GITHUB_REPOSITORY']
def run(*args,cwd=None):
    return subprocess.check_output(args,cwd=cwd,text=True).strip()
rows=json.loads(run('gh','release','list','-R',repo,'-L','1000','--exclude-drafts','--json','tagName,isPrerelease,name'))
rows=sorted((r for r in rows if r['tagName'].endswith('+hawk.1')),key=lambda r:Version(r['tagName'][1:-7]),reverse=True)
if not rows: raise RuntimeError('No finished fork release')
with tempfile.TemporaryDirectory() as tmp:
    feed=Path(tmp); remote=f'https://github.com/{repo}.git'
    run('git','init','-q','-b','update-feed',tmp)
    def git(*args): return run('git',*args,cwd=feed)
    git('remote','add','origin',remote); existing=git('ls-remote','--heads','origin','update-feed')
    if existing:
        git('fetch','--depth=1','origin','update-feed');git('reset','--keep','FETCH_HEAD')
    git('config','user.name','Magpie release bot');git('config','user.email','41898282+github-actions[bot]@users.noreply.github.com')
    tags={r['tagName'] for r in rows}; manifests={}
    def manifest(tag):
        if tag not in manifests:
            manifests[tag]=json.loads(run('gh','release','download',tag,'-R',repo,'-p','feed.json','-O','-'))
        return manifests[tag]
    def write(name,obj): (feed/name).write_text(json.dumps(obj,ensure_ascii=False)+'\n')
    latest=rows[0]['tagName'];write('latest.json',manifest(latest))
    ordinary=next((r['tagName'] for r in rows if not r['isPrerelease']),None)
    for channel in ['stable','daily']:
        path=feed/(channel+'.json')
        with urlopen('https://usemagpie.ai/api/latest?channel='+channel,timeout=45) as response:
            picked='v'+json.load(response)['version'].lstrip('v')+'+hawk.1'
        if picked in tags: write(path.name,manifest(picked))
        elif not path.exists() and ordinary: write(path.name,manifest(ordinary))
    notes=[{'version':r['tagName'][1:],'notes':r['name'],'url':f"https://github.com/{repo}/releases/tag/{r['tagName']}"} for r in rows];notes[0]['notes']=manifest(latest)['notes'];write('notes.json',{'releases':notes})
    git('add','--all')
    if git('status','--porcelain'):
        git('commit','-qm','Publish signed fork update feeds');
        # Reuse checkout's GitHub Actions credential header without reading it.
        git('config','credential.https://github.com.helper','!gh auth git-credential')
        git('push','origin','HEAD:refs/heads/update-feed')
    if ordinary: run('gh','release','edit',ordinary,'-R',repo,'--latest')
