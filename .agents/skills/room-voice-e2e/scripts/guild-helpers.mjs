// Guild API + UI 操作辅助
// 用于 E2E 测试中的 Guild 创建/加入/成员管理

async function getAuthToken(page) {
  return await page.evaluate(() => {
    return localStorage.getItem('token') || '';
  });
}

async function createGuild(page, name, { description, isPublic } = {}) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({
        name: opts.name,
        description: opts.description || '',
        is_public: opts.isPublic || false,
      }),
    });
    return r.json();
  }, { token, name, description, isPublic });
  if (res.code !== 0) throw new Error(`createGuild failed: ${res.msg}`);
  return res.data;
}

async function joinGuild(page, inviteCode) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/join', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ invite_code: opts.inviteCode }),
    });
    return r.json();
  }, { token, inviteCode });
  if (res.code !== 0) throw new Error(`joinGuild failed: ${res.msg}`);
  return res.data;
}

async function getGuildMembers(page, guildUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/members', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ guild_uuid: opts.guildUUID }),
    });
    return r.json();
  }, { token, guildUUID });
  return res.data?.members || [];
}

async function kickGuildMember(page, guildUUID, userUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/kick', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ guild_uuid: opts.guildUUID, user_uuid: opts.userUUID }),
    });
    return r.json();
  }, { token, guildUUID, userUUID });
  if (res.code !== 0) throw new Error(`kickGuildMember failed: ${res.msg}`);
}

async function leaveGuild(page, guildUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/leave', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ uuid: opts.guildUUID }),
    });
    return r.json();
  }, { token, guildUUID });
  if (res.code !== 0) throw new Error(`leaveGuild failed: ${res.msg}`);
}

async function deleteGuild(page, guildUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ uuid: opts.guildUUID }),
    });
    return r.json();
  }, { token, guildUUID });
  if (res.code !== 0) throw new Error(`deleteGuild failed: ${res.msg}`);
}

module.exports = {
  getAuthToken,
  createGuild,
  joinGuild,
  getGuildMembers,
  kickGuildMember,
  leaveGuild,
  deleteGuild,
};
