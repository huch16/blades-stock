import paramiko, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())

try:
    ssh.connect('115.190.72.22', username='root', password='hu535386', timeout=10)
    
    # Query skills table via sqlite3
    # First find the DB
    cmds = [
        'find / -name "stock.db" -type f 2>/dev/null | head -5',
    ]
    for c in cmds:
        stdin, stdout, stderr = ssh.exec_command(c, timeout=15)
        print(f">>> {c}")
        print(stdout.read().decode())
    
    ssh.close()
except Exception as e:
    print(f"ERROR: {e}", file=sys.stderr)
