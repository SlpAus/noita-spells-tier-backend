const fs = require('fs');
const path = require('path');
const cheerio = require('cheerio');

// 解析 assets/items 目录中的所有图片名称
const dir = path.join(__dirname, 'assets', 'items');
const files = fs.readdirSync(dir);

const get_name_quality = async (itemID) => {
    const response = await fetch(`https://isaac.huijiwiki.com/api.php?action=parse&format=json&text=%7B%7B%23invoke%3AIsaacGsearch%7C_find_and_get_result%7C%26a%22filter%22%3A%26a%22Type%22%3A%26a%22%24regex%22%3A%22%E9%81%93%E5%85%B7%22%2C%22%24options%22%3A%22i%22%26b%26b%2C%22pagesize%22%3A16%2C%22current_page%22%3A1%2C%22keyword%22%3A%22${itemID}%22%2C%22version%22%3A%22%E5%BF%8F%E6%82%94%22%2C%22type%22%3A%22large%22%26b%7D%7D&utf8=1&prop=text&contentmodel=wikitext&maxage=6000&smaxage=6000`, {
        headers: {
            "accept": "application/json, text/javascript, */*; q=0.01",
            "sec-ch-ua": "\"Not(A:Brand\";v=\"99\", \"Microsoft Edge\";v=\"133\", \"Chromium\";v=\"133\"",
            "sec-ch-ua-mobile": "?0",
            "sec-ch-ua-platform": "\"Linux\"",
            "x-requested-with": "XMLHttpRequest",
            "Referer": "https://isaac.huijiwiki.com/wiki/%E9%81%93%E5%85%B7",
            "Referrer-Policy": "strict-origin-when-cross-origin"
        },
        method: "GET"
    });
    const data = await response.json();
    const htmlContent = data.parse.text['*'];
    const $ = cheerio.load(htmlContent);

    // console.log(htmlContent);

    // 提取道具品质
    const quality = $('span[id="333"]');
    const qualityText = quality.length ? quality.text() : 0;

    // 提取道具名称
    let itemNameText = "未找到道具名称";
    $('.i-gs-icon-name a').each((index, element) => {
        const text = $(element).text().trim();
        if (text) {
            itemNameText = text;
            return false; // 退出循环
        }
    });

    return {
        quality: qualityText,
        itemName: itemNameText
    };
};

const extractItemID = (filename) => {
    const match = filename.match(/collectibles_(\d+)_/);
    return match ? match[1] : null;
};

const main = async () => {
    const items = [];
    for (const file of files) {
        const itemID = "c" + extractItemID(file);
        if (itemID) {
            const { quality, itemName } = await get_name_quality(itemID);
            console.log(`道具 ${itemID} 的品质是 ${quality}，名称是 ${itemName}`);
            items.push({ id: itemID, name: itemName, quality });
        }
    }

    // 将所有解析的数据存放到一个文件中
    fs.writeFileSync('items.json', JSON.stringify(items, null, 2));
    console.log('数据已保存到 items.json 文件中');
};

const test = async () => {
    const itemID = "c20";
    const { quality, itemName } = await get_name_quality(itemID);
    console.log(`道具 ${itemID} 的品质是 ${quality}，名称是 ${itemName}`);
}

main().catch(console.error);
// test().catch(console.error);

