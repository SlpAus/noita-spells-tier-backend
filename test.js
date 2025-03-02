fetch("http://114.55.238.72:8080/api/vote/sendVoting", {
    "headers": {
        "accept": "application/json, text/plain, */*",
        "accept-language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
        "cache-control": "no-cache",
        "content-type": "application/json",
        "pragma": "no-cache"
    },
    "referrer": "http://114.55.238.72:8088/",
    "referrerPolicy": "strict-origin-when-cross-origin",
    "body": "{\"type\":\"item\",\"winner\":628,\"loser\":2,\"filterNum\":705}",
    "method": "POST",
    "mode": "cors",
    "credentials": "omit"
}).then(response => response.json()).then(data => console.log(data));