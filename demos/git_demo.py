#!/usr/bin/env python3
"""Drive real concurrent cred processes through PTYs; no mocked vault or output."""
import errno
import fcntl
import os
from pathlib import Path
import pty
import select
import shutil
import struct
import subprocess
import sys
import termios
import time

PASSWORD = 'demo-only-password'
WORK = Path(os.environ['CRED_DEMO_ROOT'])
REPO = WORK / 'team.git'


class Session:
    def __init__(self, args, visible=False):
        self.visible = visible
        self.buffer = b''
        self.all_output = b''
        self.master, slave = pty.openpty()
        size = shutil.get_terminal_size((80, 24))
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', size.lines, size.columns, 0, 0))
        self.process = subprocess.Popen(['cred', *args], stdin=slave, stdout=slave,
                                        stderr=slave, start_new_session=True)
        os.close(slave)

    def read(self, timeout=0.1):
        if select.select([self.master], [], [], timeout)[0]:
            try:
                data = os.read(self.master, 65536)
            except OSError as exc:
                if exc.errno != errno.EIO:
                    raise
                return False
            if not data:
                return False
            self.buffer += data
            self.all_output += data
            if self.visible:
                sys.stdout.buffer.write(data)
                sys.stdout.buffer.flush()
        return True

    def expect(self, text):
        target = text.encode()
        deadline = time.monotonic() + 40
        while target not in self.buffer:
            if time.monotonic() > deadline or not self.read():
                raise RuntimeError(f'Expected {text!r}: {self.all_output.decode(errors="replace")}')
        self.buffer = self.buffer.split(target, 1)[1]

    def send(self, text):
        if self.visible:
            self.pause(1.2)
        os.write(self.master, (text + '\r').encode())

    def pause(self, seconds):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            if not self.read(min(0.1, max(0, deadline - time.monotonic()))):
                break

    def form(self, text):
        # Enter first focuses Continue/Open, then submits the form.
        self.send(text)
        self.send('')

    def finish(self):
        deadline = time.monotonic() + 40
        while self.process.poll() is None:
            self.read()
            if time.monotonic() > deadline:
                self.process.kill()
                raise RuntimeError('cred did not finish: ' + self.all_output.decode(errors='replace'))
        # Drain available output without waiting for PTY EOF: descendants may
        # retain a descriptor even after the command itself has exited.
        while select.select([self.master], [], [], 0)[0]:
            if not self.read(0):
                break
        code = self.process.wait(timeout=5)
        os.close(self.master)
        if code:
            raise RuntimeError(self.all_output.decode(errors='replace'))
        return self.all_output.decode(errors='replace')


def unlock(args, visible=False):
    s = Session(args, visible)
    s.expect('Master password')
    s.form(PASSWORD)
    return s


def setup():
    subprocess.run(['git', 'init', '--bare', '--initial-branch=main', str(REPO)],
                   check=True, stdout=subprocess.DEVNULL)
    s = Session(['init', 'seed'])
    s.expect('Create a password')
    s.send(PASSWORD)
    s.form(PASSWORD)
    s.expect('Review vault')
    s.send('')
    s.expect('Vault ready')
    s.send('')
    s.finish()
    s = unlock(['--vault', 'seed', 'add', 'my-project', 'vercel'])
    for name, value, secret in [('email', 'original@example.invalid', 'n'),
                                 ('password', 'original-demo-token', 'y')]:
        s.expect('Choose field name')
        s.send('j' if name == 'password' else '')
        s.expect('Field value')
        s.form(value)
        s.expect('Field visibility')
        s.send('j' if secret == 'y' else '')
    s.expect('Choose field name')
    s.send('jjjjjjj')
    s.finish()
    for client, extra in [('alice', ['--from', 'seed']), ('bob', [])]:
        s = unlock(['connect', client, str(REPO), *extra])
        s.expect('Review vault')
        s.send('')
        s.expect('Vault ready')
        s.send('')
        s.finish()
    print('DEMO READY', flush=True)


def start_edit(client, field, value):
    print(f'$ cred --vault {client} set my-project vercel {field}', flush=True)
    s = unlock(['--vault', client, 'set', 'my-project', 'vercel', field], visible=True)
    s.expect('Field value')
    s.form(value)
    s.expect('Field visibility')
    # Hold Alice before save, so Bob starts from the same remote commit.
    return s


def get(field):
    s = unlock(['--vault', 'alice', 'my-project', 'vercel', field, '--show'])
    return s.finish()


def merge():
    print('# Alice and Bob start from the same vault.', flush=True)
    alice = start_edit('alice', 'password', 'alice-demo-token')
    print('\n# Alice is still editing; Bob publishes a different field.', flush=True)
    bob = start_edit('bob', 'email', 'bob@example.invalid')
    bob.send('')
    bob.finish()
    print('\n# Alice saves: cred merges the two fields and publishes.', flush=True)
    alice.send('')
    output = alice.finish()
    assert 'Choose a value' not in output, output
    assert 'alice-demo-token' in get('password')
    assert 'bob@example.invalid' in get('email')
    print('Independent edits verified: both fields survived.', flush=True)


def conflict():
    print('# Both writers change the same field.', flush=True)
    alice = start_edit('alice', 'password', 'alice-new-demo-token')
    print('\n# Bob publishes first.', flush=True)
    bob = start_edit('bob', 'password', 'bob-new-demo-token')
    bob.send('')
    bob.finish()
    print('\n# Alice saves; this time a choice is required.', flush=True)
    alice.send('')
    alice.expect('Choose a value')
    alice.expect('Use my value')
    alice.pause(3)
    for _ in range(2):
        os.write(alice.master, b'\x1b[B')
        alice.pause(0.7)
    alice.send('')
    alice.finish()
    assert 'alice-new-demo-token' in get('password')
    assert 'bob@example.invalid' in get('email')
    print('Conflict resolution verified: mine kept; email preserved.', flush=True)


if __name__ == '__main__':
    {'setup': setup, 'merge': merge, 'conflict': conflict}[sys.argv[1]]()
