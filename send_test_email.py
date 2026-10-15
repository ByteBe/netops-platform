import smtplib
from email.mime.text import MIMEText

msg = MIMEText("链路告警：测试链路已中断\n时间：2026-10-15 17:10:00\n级别：严重\n目标：测试链路\n详情：链路连通性检测失败，丢包率100%。", 'plain', 'utf-8')
msg['From'] = '3259034703@qq.com'
msg['To'] = '3259034703@qq.com'
msg['Subject'] = '[严重] NetOps 链路告警 - 测试链路中断'

s = smtplib.SMTP_SSL('smtp.qq.com', 465)
s.login('3259034703@qq.com', 'jdwrhfqazjeqcgig')
s.sendmail('3259034703@qq.com', ['3259034703@qq.com'], msg.as_string())
s.quit()
print("OK")
