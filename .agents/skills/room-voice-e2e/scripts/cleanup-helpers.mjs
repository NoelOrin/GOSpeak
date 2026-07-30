// 测试数据清理

async function getAuthToken(page) {
  return await page.evaluate(() => localStorage.getItem('token') || '');
}

async function cleanupGuild(page, guildUUID) {
  const token = await getAuthToken(page);
  await page.evaluate(async (opts) => {
    await fetch('/api/v1/guild/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ uuid: opts.guildUUID }),
    });
  }, { token, guildUUID });
}

async function cleanupAllGuilds(page) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (token) => {
    const r = await fetch('/api/v1/guild/my-guilds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({}),
    });
    return r.json();
  }, token);

  const uuids = res.data?.guild_uuids || [];
  for (const uuid of uuids) {
    await page.evaluate(async (opts) => {
      await fetch('/api/v1/guild/leave', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
        body: JSON.stringify({ uuid: opts.uuid }),
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

module.exports = { getAuthToken, cleanupGuild, cleanupAllGuilds, cleanupTestUser };
