fetch("http://114.55.238.72:8080/api/v1/vote/send", {
    "headers": {
        "accept": "application/json, text/plain, */*",
        "accept-language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
        "cache-control": "no-cache",
        "content-type": "application/json",
        "pragma": "no-cache",
        "proxy-connection": "keep-alive",
        "cookie": "user_id=f64a0ee6-fe5d-4d8a-a388-14d1139ab7c4",
        "Referer": "http://114.55.238.72:8088/",
        "Referrer-Policy": "strict-origin-when-cross-origin"
    },
    // winner: 628 loser : 111 filternum: 705
    "body": "{\"type\":\"item\",\"winner\":628,\"loser\":154,\"filterNum\":705}",
    "method": "POST"
}).then(response => response.json()).then(data => console.log(data));