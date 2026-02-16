// 启动时记录
console.log('Cookie Sniffer service worker 已启动');

// 服务配置
const SERVICES = {
  douyin: {
    name: 'douyin',
    displayName: '抖音',
    domains: ['douyin.com'],
    cookieDomain: '.douyin.com'
  },
  tiktok: {
    name: 'tiktok',
    displayName: 'TikTok',
    domains: ['tiktok.com'],
    cookieDomain: '.tiktok.com'
  },
  bilibili: {
    name: 'bilibili',
    displayName: 'B站',
    domains: ['bilibili.com'],
    cookieDomain: '.bilibili.com'
  }
};

// 获取服务名称
function getServiceFromUrl(url) {
  for (const [key, service] of Object.entries(SERVICES)) {
    if (service.domains.some(domain => url.includes(domain))) {
      return service;
    }
  }
  return null;
}

// 检查是否在5分钟内抓取过
async function shouldSkipCapture(serviceName) {
  return new Promise((resolve) => {
    chrome.storage.local.get([`lastCapture_${serviceName}`], function(result) {
      const lastTime = result[`lastCapture_${serviceName}`];
      if (!lastTime) {
        resolve(false);
        return;
      }
      
      const now = Date.now();
      const fiveMinutes = 5 * 60 * 1000;
      const shouldSkip = (now - lastTime) < fiveMinutes;
      
      if (shouldSkip) {
        console.log(`${serviceName}: 5分钟内已抓取过，跳过`);
      }
      resolve(shouldSkip);
    });
  });
}

// 检查Cookie是否有变化
async function isCookieChanged(serviceName, newCookie) {
  return new Promise((resolve) => {
    chrome.storage.local.get([`cookieData_${serviceName}`], function(result) {
      const existingData = result[`cookieData_${serviceName}`];
      if (!existingData || existingData.cookie !== newCookie) {
        resolve(true);
      } else {
        console.log(`${serviceName}: Cookie内容无变化，跳过`);
        resolve(false);
      }
    });
  });
}

// 保存Cookie数据
async function saveCookieData(serviceName, url, cookie, source = 'headers') {
  const cookieData = {
    service: serviceName,
    url: url,
    timestamp: Date.now(),
    lastUpdate: new Date().toISOString(),
    cookie: cookie,
    source: source
  };
  
  // 保存服务数据
  chrome.storage.local.set({
    [`cookieData_${serviceName}`]: cookieData,
    [`lastCapture_${serviceName}`]: Date.now()
  });
  
  // 触发Webhook回调
  await sendWebhook(serviceName, cookie);
  
  console.log(`${serviceName}: Cookie已保存`);
}

// Webhook回调
async function sendWebhook(serviceName, cookie) {
  chrome.storage.local.get(['webhookUrl'], function(result) {
    const webhookUrl = result.webhookUrl;
    if (webhookUrl && webhookUrl.trim()) {
      const baseUrl = webhookUrl.replace(/\/$/, "");
      const targetUrl = `${baseUrl}/platforms/${serviceName}/cookie`;

      const payload = {
        cookie: cookie,
        timestamp: new Date().toISOString()
      };

      fetch(targetUrl, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload)
      }).then(response => {
        console.log(`Webhook回调成功: ${serviceName}`, response.status);
      }).catch(error => {
        console.error(`Webhook回调失败: ${serviceName}`, error);
      });
    }
  });
}

chrome.webRequest.onBeforeSendHeaders.addListener(
    async function(details) {
      const service = getServiceFromUrl(details.url);
      if (!service) return;
      
      console.log(`请求拦截: ${service.displayName}`, details.url, details.method);
      
      if (details.method === "POST" || details.method === "GET") {
        // 检查5分钟限制
        if (await shouldSkipCapture(service.name)) {
          return;
        }
        
        let cookieFound = false;
        
        // 尝试从请求头获取Cookie
        if (details.requestHeaders) {
          for (let header of details.requestHeaders) {
            if (header.name.toLowerCase() === "cookie") {
              console.log(`从请求头捕获到Cookie: ${service.displayName}`);
              
              // 检查Cookie是否有变化
              if (await isCookieChanged(service.name, header.value)) {
                await saveCookieData(service.name, details.url, header.value, 'headers');
              }
              
              cookieFound = true;
              break;
            }
          }
        }
        
        // 如果请求头没有Cookie，使用cookies API备用方案
        if (!cookieFound) {
          chrome.cookies.getAll({domain: service.cookieDomain}, async function(cookies) {
            if (cookies && cookies.length > 0) {
              console.log(`通过cookies API获取到: ${service.displayName}`, cookies.length, '个cookie');
              const cookieString = cookies.map(c => `${c.name}=${c.value}`).join('; ');
              
              // 检查Cookie是否有变化
              if (await isCookieChanged(service.name, cookieString)) {
                await saveCookieData(service.name, details.url, cookieString, 'cookies_api');
              }
            }
          });
        }
      }
    },
    { urls: [
        "https://*.douyin.com/*", "https://douyin.com/*",
        "https://*.tiktok.com/*", "https://tiktok.com/*",
        "https://*.bilibili.com/*", "https://bilibili.com/*"
    ] },
    ["requestHeaders", "extraHeaders"]
  );

// 添加存储变化监听
chrome.storage.onChanged.addListener((changes, areaName) => {
  if (areaName === 'local') {
    // 监听服务数据变化
    Object.keys(changes).forEach(key => {
      if (key.startsWith('cookieData_')) {
        const serviceName = key.replace('cookieData_', '');
        const serviceConfig = SERVICES[serviceName];
        if (serviceConfig && changes[key].newValue) {
          console.log(`${serviceConfig.displayName} Cookie数据已更新`);
        }
      }
    });
  }
});

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'testWebhook') {
    const serviceName = 'bilibili'; // 测试默认使用 B站
    const testCookie = 'test_cookie_value_' + Date.now();

    console.log("收到测试请求，准备发送 Webhook...");

    chrome.storage.local.get(['webhookUrl'], function(result) {
      const webhookUrl = result.webhookUrl;
      if (!webhookUrl) {
        sendResponse({ success: false, error: '未配置 Webhook 地址' });
        return;
      }

      // 构造测试 Payload
      const baseUrl = webhookUrl.replace(/\/$/, "");
      const targetUrl = `${baseUrl}/platforms/${serviceName}/cookie`;

      const payload = {
        cookie: testCookie,
        timestamp: new Date().toISOString(),
        test: true, // 标记为测试请求
        message: "Extension Connection Test"
      };

      fetch(targetUrl, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      .then(response => {
        if (response.ok) {
          console.log("测试请求发送成功");
          sendResponse({ success: true });
        } else {
          console.error("测试请求服务器返回错误:", response.status);
          sendResponse({ success: false, error: `Server Error: ${response.status}` });
        }
      })
      .catch(error => {
        console.error("测试请求发送失败:", error);
        sendResponse({ success: false, error: error.message });
      });
    });

    return true; // 保持消息通道开启以进行异步响应
  }
});