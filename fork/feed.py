"""Atomically publish channel manifests for complete, signed desktop releases."""
import json
import os
import re
import subprocess
import tempfile
from datetime import datetime, timedelta, timezone
from pathlib import Path

REPO = os.environ['GITHUB_REPOSITORY']


def run(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def version(tag):
    base, revision = tag.split('+hawk.')
    numbers, _, pre = base[1:].partition('-')
    ids = tuple((0, int(s)) if s.isdigit() else (1, s) for s in pre.split('.'))
    return tuple(map(int, numbers.split('.'))), not pre, ids, int(revision)


def inventory():
    rows = []
    for page in range(1, 10000):
        batch = json.loads(run('gh', 'api', f'repos/{REPO}/releases?per_page=100&page={page}'))
        rows.extend(r for r in batch if not r['draft'] and
                    re.fullmatch(r'v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?\+hawk\.[1-9]\d*', r['tag_name']))
        if len(batch) < 100:
            break
    return sorted(rows, key=lambda r: version(r['tag_name']), reverse=True)


rows = inventory()
if not rows:
    raise SystemExit(0)
ordinary = [r for r in rows if not r['prerelease']]
now = datetime.now(timezone.utc)
cut = now.replace(hour=20, minute=0, second=0, microsecond=0)
if cut > now:
    cut -= timedelta(days=1)
# The weekly channel needs three days of age by Monday 00:00 UTC. A new fork
# starts its picked channels with its first ordinary build. Urgent releases
# advance both. A build is eligible only after its desktop jobs have passed.
weekly = now.replace(hour=0, minute=0, second=0, microsecond=0) - timedelta(days=now.weekday()+3)


def picked(deadline):
    for r in ordinary:
        published = datetime.fromisoformat(r['published_at'].replace('Z', '+00:00'))
        if published <= deadline or re.search(r'^\*\*Urgent release:\*\*', r['body'] or '', re.M):
            return r
    return min(ordinary, key=lambda r: r['published_at']) if ordinary else None


with tempfile.TemporaryDirectory() as tmp:
    folder = Path(tmp)
    def git(*args):
        return run('git', *args, cwd=folder)
    git('init', '-q', '-b', 'update-feed')
    git('remote', 'add', 'origin', f'https://github.com/{REPO}.git')
    git('config', 'credential.https://github.com.helper', '!gh auth git-credential')
    if git('ls-remote', '--heads', 'origin', 'update-feed'):
        git('fetch', '--depth=1', 'origin', 'update-feed')
        git('checkout', '-B', 'update-feed', 'FETCH_HEAD')
    git('config', 'user.name', 'Magpie release bot')
    git('config', 'user.email', '41898282+github-actions[bot]@users.noreply.github.com')
    def write(name, obj):
        (folder/name).write_text(json.dumps(obj, ensure_ascii=False)+'\n')
    manifests = {}
    for filename, row in [('latest.json', rows[0]), ('daily.json', picked(cut)), ('stable.json', picked(weekly))]:
        if row is None:
            continue
        tag = row['tag_name']
        if tag not in manifests:
            manifests[tag] = json.loads(run('gh', 'release', 'download', tag, '-R', REPO, '-p', 'feed.json', '-O', '-'))
        write(filename, manifests[tag])
    write('notes.json', {'releases': [{'version': r['tag_name'][1:], 'notes': r['body'], 'url': r['html_url']} for r in rows[:300]]})
    git('add', '--all')
    if git('status', '--porcelain'):
        git('commit', '-qm', 'Publish signed fork update feeds')
        git('push', 'origin', 'HEAD:refs/heads/update-feed')
    if ordinary and run('gh','api', f'repos/{REPO}/releases/latest','--jq','.tag_name') != ordinary[0]['tag_name']:
        run('gh', 'release', 'edit', ordinary[0]['tag_name'], '-R', REPO, '--latest')
