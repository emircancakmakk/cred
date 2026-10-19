#!/usr/bin/env python3
"""Build and record in a temporary subprocess environment, never the user's vault."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    for tool in ('go', 'git', 'vhs', 'ffmpeg', 'ttyd', 'python3'):
        if not shutil.which(tool):
            raise SystemExit(f'Missing {tool}; see demos/README.md')
    with tempfile.TemporaryDirectory(prefix='cred-demo-', dir='/tmp') as tmp:
        work = Path(tmp)
        bin_dir = work / 'bin'
        bin_dir.mkdir()
        subprocess.run(['go', 'build', '-o', str(bin_dir / 'cred'), '.'], cwd=ROOT, check=True)
        env = os.environ.copy()
        # Only child processes get a disposable home/config directory. macOS Go
        # derives UserConfigDir from HOME; XDG_CONFIG_HOME alone is insufficient.
        env.update(HOME=str(work / 'home'), XDG_CONFIG_HOME=str(work / 'config'),
                   PATH=str(bin_dir) + os.pathsep + env['PATH'],
                   CRED_DEMO_ROOT=str(work), PS1='$ ', BASH_SILENCE_DEPRECATION_WARNING='1',
                   GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
                   GIT_AUTHOR_NAME='Demo', GIT_AUTHOR_EMAIL='demo@example.invalid',
                   GIT_COMMITTER_NAME='Demo', GIT_COMMITTER_EMAIL='demo@example.invalid')
        env.pop('CRED_VAULT', None)
        for key in ('HOME', 'XDG_CONFIG_HOME'):
            Path(env[key]).mkdir(parents=True, exist_ok=True)
        if sys.argv[1:] == ['--check']:
            for scenario in ('setup', 'merge', 'conflict'):
                subprocess.run(['python3', 'demos/git_demo.py', scenario], cwd=ROOT, env=env, check=True)
            return
        for tape in (sys.argv[1:] or ('quick-start', 'git-merge')):
            if tape not in ('quick-start', 'git-merge'):
                raise SystemExit(f'Unknown tape: {tape}')
            subprocess.run(['vhs', f'demos/{tape}.tape'], cwd=ROOT, env=env, check=True)


if __name__ == '__main__':
    main()
