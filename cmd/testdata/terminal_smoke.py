"""Exercise real raw-mode input and alternate screens through the Go test helper."""
import fcntl
import os
import pty
import select
import struct
import subprocess
import sys
import termios
import time

master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 80, 0, 0))
before = termios.tcgetattr(slave)
env = dict(os.environ, CRED_TEST_TERMINAL='1', TERM='xterm-256color')
proc = subprocess.Popen([sys.argv[1], '-test.run=^TestTerminalInterfaceHelper$'], stdin=slave, stdout=slave, stderr=slave, env=env, start_new_session=True)
output = bytearray()

def wait(token, start=0, timeout=10):
    deadline = time.monotonic()+timeout
    while time.monotonic()<deadline:
        if token in output[start:]:
            return
        if select.select([master], [], [], .05)[0]:
            output.extend(os.read(master, 65536))
    raise AssertionError('terminal milestone missing: '+token.decode())

def send(text):
    position = len(output)
    os.write(master, text)
    time.sleep(.07)
    return position

password = b'pty-fixture-password'
try:
    wait(b'Choose a vault')
    pos=send(b'\r');wait(b'Vault details',pos)
    pos=send(b'\x15ptyvault\r\r');wait(b'Create a password',pos)
    pos=send(password)
    # The default masked field must not draw plaintext.
    while select.select([master], [], [], 0)[0]:
        output.extend(os.read(master,65536))
    assert password not in output
    pos=send(b'\x12');wait(b'Visible',pos);wait(password,pos)
    send(b'\x12')
    pos=send(b'\r'+password+b'\r\r');wait(b'Review vault',pos)
    # Back from review must clear both secret inputs.
    pos=send(b'\t\r');wait(b'Create a password',pos)
    pos=send(password+b'\r'+password+b'\r\r');wait(b'Review vault',pos)
    pos=send(b'\r');wait(b'Vault ready',pos)
    pos=send(b'\r');wait(b'Master password',pos)
    pos=send(password+b'\r\r');wait(b'Projects',pos)
    pos=send(b'n');wait(b'New project',pos)
    pos=send(b'smoke-project\r\r');wait(b'created smoke-project',pos)
    pos=send(b'\r');wait(b'No services',pos)
    pos=send(b'n');wait(b'New service',pos)
    pos=send(b'smoke-service\r\r');wait(b'created smoke-service',pos)
    pos=send(b'\r');wait(b'No fields',pos)
    pos=send(b'n');wait(b'Choose field name',pos)
    # Choose password and start typing directly into the focused Value field.
    pos=send(b'\x1b[B\r');wait(b'New field',pos)
    pos=send(b'pty-field-secret\r\r\r');wait(b'created password',pos)
    assert b'pty-field-secret' not in output
    pos=send(b'm');wait(b'Saved emails',pos)
    pos=send(b'n');wait(b'Add email addresses',pos)
    # Bracketed paste preserves newlines without activating buttons.
    pos=send(b'\x1b[200~personal@example.com\nwork@example.com;PERSONAL@example.com\x1b[201~')
    pos=send(b'\t\r');wait(b'Review email addresses',pos);wait(b'2 to add, 1 duplicates skipped',pos)
    pos=send(b'\r');wait(b'Saved email addresses updated',pos)
    send(b'\x1b')
    pos=send(b'n');wait(b'Choose field name',pos)
    pos=send(b'\r');wait(b'New field',pos)
    pos=send(b'\x05');wait(b'Choose saved email',pos)
    pos=send(b'\x1b[B\r');wait(b'New field',pos)
    pos=send(b'\r\r\r');wait(b'created email',pos)
    # Edit chooses another address, then explicitly saves the copied value.
    pos=send(b'e');wait(b'Edit field',pos)
    pos=send(b'\x05');wait(b'Choose saved email',pos)
    send(b'\r')
    pos=send(b'\r\r\r');wait(b'updated email',pos)
    pos=send(b'q');wait(b'Master password',pos)
    pos=send(password+b'\r\r');wait(b'Projects',pos)
    send(b'\r\r')
    pos=send(b'e');wait(b'Edit field',pos)
    pos=send(b'\x05');wait(b'Choose saved email',pos)
    send(b'/')
    pos=send(b'work');wait(b'work',pos)
    send(b'\r')
    send(b'\r')
    pos=send(b'\r\r\r');wait(b'updated email',pos)
    pos=send(b'q');wait(b'PTY_DONE',pos)
    proc.wait(timeout=5)
    assert proc.returncode==0
    assert b'\x1b[?1049h' in output and b'\x1b[?1049l' in output
    assert password not in output.rsplit(b'\x1b[?1049l',1)[-1]
    after=termios.tcgetattr(slave)
    # PENDIN is a transient macOS kernel flag when canonical input resumes.
    before[3]&=~getattr(termios,'PENDIN',0)
    after[3]&=~getattr(termios,'PENDIN',0)
    assert before==after, 'terminal input modes were not restored'
    print('PTY: setup, reveal/hide, Back, unlock, CRUD, email batch/picker/restart, cleanup passed')
finally:
    if proc.poll() is None:
        proc.kill()
        proc.wait()
    os.close(master)
    os.close(slave)
