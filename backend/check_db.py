import sqlite3
conn = sqlite3.connect(r'E:\IDEA\netops-platform\backend\data\netops.db')
c = conn.cursor()
print('=== Docker settings ===')
for row in c.execute("SELECT key,value FROM system_settings WHERE key LIKE 'docker%'"):
    print(row)
print('=== Docker hosts ===')
for row in c.execute('SELECT id,name,address,enabled FROM docker_hosts'):
    print(row)
conn.close()
