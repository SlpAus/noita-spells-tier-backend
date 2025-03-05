for (let loser = 530; loser <= 535; loser++) {
    fetch("https://vote.qiuy.cloud/api/v1/vote/send", {
        headers: {
            "accept": "application/json, text/plain, */*",
            "accept-language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
            "cache-control": "no-cache",
            "content-type": "application/json",
            "pragma": "no-cache",
            "sec-ch-ua": "\"Not(A:Brand\";v=\"99\", \"Microsoft Edge\";v=\"133\", \"Chromium\";v=\"133\"",
            "sec-ch-ua-mobile": "?0",
            "sec-ch-ua-platform": "\"Linux\"",
            "sec-fetch-dest": "empty",
            "sec-fetch-mode": "cors",
            "sec-fetch-site": "same-origin",
            "cookie": "user_id=2a7b8050-b2a8-43a0-b5bd-d27805fbd160",
            "Referer": "https://vote.qiuy.cloud/",
            "Referrer-Policy": "strict-origin-when-cross-origin"
        },
        body: JSON.stringify({
            type: "item",
            winner: 628,       // 固定胜者ID
            loser: loser,      // loser 从1到30
            filterNum: 705
        }),
        method: "POST"
    })
        .then(response => response.json())
        .then(data => console.log(`loser:${loser}`, data))
        .catch(error => console.log(`loser:${loser}`, error));
}