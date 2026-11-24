"""Dispatch each published upstream version from the fork's initial release onward."""
import json, os, re, subprocess
repo = os.environ['GITHUB_REPOSITORY']
def gh(*args):
    return subprocess.check_output(['gh', *args], text=True)
active = {r['displayTitle'] for r in json.loads(gh('run','list','-R',repo,'--workflow','fork-release.yml','-L','100','--json','status,displayTitle')) if r['status'] != 'completed'}
for page in range(1,10000):
    rows=json.loads(gh('api',f'repos/yetone/magpie-releases/releases?per_page=100&page={page}'))
    for r in reversed(rows):
        tag=r['tag_name']
        if r['draft'] or r['published_at'] < '2026-11-24T03:14:45Z' or not re.fullmatch(r'v\d+\.\d+\.\d+(?:-[\w.-]+)?',tag) or 'Build '+tag in active:
            continue
        status=subprocess.run(['gh','release','view',tag+'+hawk.1','-R',repo,'--json','isDraft'],capture_output=True,text=True)
        if status.returncode==0 and not json.loads(status.stdout)['isDraft']:
            continue
        if status.returncode and 'not found' not in status.stderr.lower():
            raise RuntimeError(status.stderr)
        gh('workflow','run','fork-release.yml','-R',repo,'--ref','main','-f',f'tag={tag}')
    if len(rows)<100 or rows[-1]['created_at'] < '2026-11-23': break
