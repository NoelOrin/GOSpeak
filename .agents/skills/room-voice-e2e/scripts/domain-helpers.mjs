// Domain API + UI 操作辅助
// 用于 E2E 测试中的 Domain 创建/加入/成员管理
// 注意：GOSpeak 当前架构以 Domain（语音域/租户）为顶层归属容器，
// 旧版 Guild 术语与 /api/v1/guild/* 接口已统一为 Domain。

async function getAuthToken(page) {
  return await page.evaluate(() => {
    return localStorage.getItem('token') || '';
  });
}

async function createDomain(page, name, { description, isPublic } = {}) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/domain/create', {
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
  if (res.code !== 0) throw new Error(`createDomain failed: ${res.msg}`);
  return res.data;
}

async function joinDomain(page, inviteCode) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/domain/join', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ invite_code: opts.inviteCode }),
    });
    return r.json();
  }, { token, inviteCode });
  if (res.code !== 0) throw new Error(`joinDomain failed: ${res.msg}`);
  return res.data;
}

async function getDomainMembers(page, domainUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/domain/members', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ domain_uuid: opts.domainUUID }),
    });
    return r.json();
  }, { token, domainUUID });
  return res.data?.members || [];
}

async function kickDomainMember(page, domainUUID, userUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/domain/kick', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ domain_uuid: opts.domainUUID, user_uuid: opts.userUUID }),
    });
    return r.json();
  }, { token, domainUUID, userUUID });
  if (res.code !== 0) throw new Error(`kickDomainMember failed: ${res.msg}`);
}

async function leaveDomain(page, domainUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/domain/leave', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ domain_uuid: opts.domainUUID }),
    });
    return r.json();
  }, { token, domainUUID });
  if (res.code !== 0) throw new Error(`leaveDomain failed: ${res.msg}`);
}

async function deleteDomain(page, domainUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/domain/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ domain_uuid: opts.domainUUID }),
    });
    return r.json();
  }, { token, domainUUID });
  if (res.code !== 0) throw new Error(`deleteDomain failed: ${res.msg}`);
}

module.exports = {
  getAuthToken,
  createDomain,
  joinDomain,
  getDomainMembers,
  kickDomainMember,
  leaveDomain,
  deleteDomain,
};
