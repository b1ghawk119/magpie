import json, os, subprocess
from pathlib import Path
from urllib.parse import quote
repo, tag, ref = (os.environ[k] for k in ('GITHUB_REPOSITORY','VERSION','REF'))
def gh(*args):
    return subprocess.check_output(['gh', *args],text=True)
up=json.loads(gh('release','view',ref,'-R','yetone/magpie-releases','--json','isPrerelease,body,url'))
page=f'https://github.com/{repo}/releases/tag/{tag}'
base=f'https://github.com/{repo}/releases/download/{quote(tag,safe="")}/'
files=sorted(Path('dist').glob('magpie-*'))
sums=Path('dist/SHA256SUMS').read_text(); sig=Path('dist/SHA256SUMS.sig').read_text().strip()
assets={p.name:{'url':base+p.name,'github':base+p.name,'size':p.stat().st_size} for p in files}
notes=f"Upstream {up['url']}\n\nDownstream build `{os.environ['SOURCE']}`. Windows desktop executable and macOS DMG / update ZIP.\n\n"+up['body']
Path('dist/feed.json').write_text(json.dumps(dict(version=tag[1:],notes=notes,url=page,assets=assets,signed=dict(sums=sums,sig=sig))))
Path('release-notes.md').write_text(notes)
# A draft is invisible to update clients. Resume a partial upload after failure.
existing=subprocess.run(['gh','release','view',tag,'-R',repo,'--json','isDraft'],capture_output=True,text=True)
if existing.returncode:
    gh('release','create',tag,'-R',repo,'--draft','--target',os.environ['SOURCE'],'--title',f'Magpie {tag}','--notes-file','release-notes.md',*(['--prerelease'] if up['isPrerelease'] else []))
elif not json.loads(existing.stdout)['isDraft']:
    raise SystemExit('Already published; refusing to replace a released binary')
gh('release','upload',tag,'-R',repo,'--clobber',*map(str,Path('dist').iterdir()))
gh('release','edit',tag,'-R',repo,'--draft=false','--latest=false')
