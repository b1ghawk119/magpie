"""Catch up published upstream tags, including prereleases and missed polling windows."""
import json
import os
import re
import subprocess

REPO = os.environ['GITHUB_REPOSITORY']
START = '2026-11-24T03:13:55Z'
TAG = re.compile(r'v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?')


def gh(*args):
    return subprocess.check_output(['gh', *args], text=True)


def releases(repo):
    for page in range(1, 10000):
        rows = json.loads(gh('api', f'repos/{repo}/releases?per_page=100&page={page}'))
        yield from (r for r in rows if not r['draft'] and r['published_at'] >= START)
        if len(rows) < 100 or rows[-1]['created_at'] < '2026-11-23':
            break


built = {r['tag_name'] for r in releases(REPO)}
runs = json.loads(gh('run', 'list', '-R', REPO, '--workflow', 'fork-release.yml',
                     '-L', '200', '--json', 'status,displayTitle'))
active = {r['displayTitle'] for r in runs if r['status'] != 'completed'}
for release in sorted(releases('yetone/magpie-releases'), key=lambda r: r['published_at']):
    tag = release['tag_name']
    if TAG.fullmatch(tag) and tag + '+hawk.1' not in built and 'Build ' + tag not in active:
        gh('workflow', 'run', 'fork-release.yml', '-R', REPO, '--ref', 'main', '-f', f'tag={tag}')
