// 测试数据清理
// 注意：GOSpeak 当前以 Domain（语音域/租户）为顶层归属容器，
// 旧版 Guild 术语与 /api/v1/guild/* 接口已统一为 Domain。

async function getAuthToken(page) {
  return await page.evaluate(() => localStorage.getItem('token') || '');
}

async function cleanupDomain(page, domainUUID) {
  const token = await getAuthToken(page);
  await page.evaluate(async (opts) => {
    await fetch('/api/v1/domain/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ domain_uuid: opts.domainUUID }),
    });
  }, { token, domainUUID });
}

async function cleanupAllDomains(page) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (token) => {
    const r = await fetch('/api/v1/domain/my-domains', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({}),
    });
    return r.json();
  }, token);

  const domains = res.data || [];
  for (const d of domains) {
    const uuid = d.uuid;
    if (!uuid) continue;
    await page.evaluate(async (opts) => {
      await fetch('/api/v1/domain/leave', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
        body: JSON.stringify({ domain_uuid: opts.uuid }),
      });
    }, { token, uuid });
  }
}

async function cleanupTestUser(page, username) {
  const token = await getAuthToken(page);
  await page.evaluate(async (opts) => {
    await fetch('/api/v1/user/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ name: opts.username }),
    });
  }, { token, username });
}

module.exports = { getAuthToken, cleanupDomain, cleanupAllDomains, cleanupTestUser };
