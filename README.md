# Share Mail 临时域名邮箱服务

#### 可以用来干一切你想用邮箱干的事情，但是不会留下任何痕迹。


## 如何使用

### 1. 准备工作: 前往[`https://api.fishzemail.top/login`](https://api.fishzemail.top/login), 通过`Github`授权认证

前往上述链接, 授权登录`share mail`的`Github OAuth App`后, 您将会被重定向到`https://api.fishzemail.top/login/redirect`

此时, 会向您返回如下内容:
```json
{
	"code": 0,
	"data": {
		"email": "contact.github@fishze.top",
		"name": "FishZe",
		"uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	}
}
```
请记录下`data`中`uuid`的值, 该值将会在后续的认证过程中使用到.

当然了, 如果忘记也没关系, 重新访问链接, 会找回之前的`uuid`.

### 2. 发送邮箱申请

> `https://api.fishzemail.top/mail/new`

方式: POST

鉴权方式: 在请求头中添加`Authorization`字段, 内容为上文的`uuid`

返回值:
```json
{
  "code": 0,
  "data": {
    "email": "ak8iNNEtTg@fishzemail.top",
    "emailId": "667dac6a-62a3-41be-9dcb-33ffc58a9778"
  }
}
```
其中, `code`字段含义如下:

| 字段         | 说明             |
|------------|----------------|
| email      | 临时邮箱地址         |
| emailId    | 临时邮箱ID, 用于后续操作 |


### 3. 轮询验证结果

> `https://api.fishzemail.top/mail/query`
>
方式: POST

鉴权方式: 在请求头中添加`Authorization`字段, 内容为上文的`uuid`

数据内容: obj: `{"emailId": "667dac6a-62a3-41be-9dcb-33ffc58a9778"}`, 其中`emailId`为上文的`emailId`

返回值:

#### 已有邮件时:
```json
{
  "code": 0,
  "data": {
    "msg": "",
    "mails": [{
      "from": "xxx@example.com",
      "to": "ak8iNNEtTg@fishzemail.top",
      "data": 1671760151,
      "subject": "TEST",
      "plain_text": ["test"],
      "html_text": ["<div>test</div>"],
      "attachments": null
    }],
    "emailId": "667dac6a-62a3-41be-9dcb-33ffc58a9778",
    "email": "ak8iNNEtTg@fishzemail.top",
    "updateTime": 1671760153
  }
}

```
| 字段         | 说明             |
|------------|----------------|
| msg        | 附加信息           |
| mails      | 邮件列表           |
| emailId    | 临时邮箱ID, 用于后续操作 |
| email      | 临时邮箱地址         |
| updateTime | 邮件更新时间         |

其中, 邮件列表`mails`中的字段含义如下:

| 字段           | 说明                    |
|--------------|-----------------------|
| from         | 发件人邮箱地址               |
| to           | 收件人邮箱地址               |
| data         | 邮件发送时间戳               |
| subject      | 邮件主题                  |
| plain_text   | 邮件纯文本内容               |
| html_text    | 邮件HTML内容              |
| attachments  | 邮件附件列表, 为`null`时表示无附件 |

邮件列表最多显示最近的10封邮件.

#### 未收到邮件时:
```json
{
  "code": 0,
  "data": {
    "msg": "",
    "mails": null,
    "emailId": "667dac6a-62a3-41be-9dcb-33ffc58a9778",
    "email": "ak8iNNEtTg@fishzemail.top",
    "updateTime": 1671764011
  }
}
```
### 过期时间

1. 临时邮箱地址: `5分钟`, 即`5分钟`未收到邮件, 该邮箱将会被删除.
2. 轮询**查到**结果后, 过期时间顺延`5分钟`, 即`5分钟`内未收到邮件且未轮询成功, 该邮箱将会被删除.

### 频率限制

1. 单个用户每秒请求限制为`50次`
2. 总请求限制在`1000次/秒`

### 错误码

| 错误码    | 说明                    | 备注               |
|--------|-----------------------|------------------|
| 0      | SuccessCode           | 请求成功             |
| 1      | EmailNotFount         | 临时邮箱不存在, 可能是已过期  |
| 2      | EmailIdNotFount       | 临时邮箱ID不存在        |
| 3      | ServerErrorCode       | 服务器错误            |
| 4      | AuthorizationError    | 鉴权错误             |


## 如何部署

太麻烦了, 之后再写吧.

## `Python`快速使用脚本

请填入uuid即可使用

```python
import json
import sys
import time
import requests

uuid = ""

BASE_URL = "https://api.fishzemail.top/"
HEADER = {"Authorization": uuid}
LAST_DATA = 0

def get_new_account()-> (tuple[None, None] | tuple):
    url = BASE_URL + "mail/new"
    r = requests.post(url,headers=HEADER, timeout=10)
    res = json.loads(r.text)
    if res["code"] != 0:
        return None, None
    return res['data']['email'], res['data']['emailId']


def receive_new_email(emailId: str)-> (dict | None):
    url = BASE_URL + "mail/query"
    data = {"emailId": emailId}
    r = requests.post(url,headers=HEADER,data=data,timeout=10)
    res = json.loads(r.text)
    if res["code"] != 0:
        return None
    return res['data']['mails']


def print_email(now_mail:dict) -> None:
    print("From: {}".format(now_mail['from']))
    print("To: {}".format(now_mail['to']))
    print("Subject: {}".format(now_mail['subject']))
    print("TimeStalp: {}".format((now_mail['data'])))
    print("Content: {}".format(now_mail['plain_text']))
    print()

if __name__ == "__main__":
    email, emailId = get_new_account()
    if email is None:
        print("Failed to get new account")
        sys.exit(1)
    print("Your email is: {}".format(email))
    while True:
        emails = receive_new_email(emailId)
        if emails is not None:
            for email in emails:
                if LAST_DATA < email["data"]:
                    print_email(email)
                else :
                    LAST_DATA = email["data"]
            con = input("Continue? [Y/n]")
            if con.lower() == "n":
                break
        time.sleep(0.5)

```
