<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { withBase } from 'vitepress'

const REPO = 'NoelOrin/GOSpeak'
const RELEASES_URL = `https://github.com/${REPO}/releases`
const LATEST_RELEASE_URL = `${RELEASES_URL}/latest`
const RELEASES_LIST_API = `https://api.github.com/repos/${REPO}/releases?per_page=30`

interface AssetEntry {
  url: string
  size: number
}

interface AssetMap {
  darwinArm?: AssetEntry
  darwinIntel?: AssetEntry
  windows?: AssetEntry
  linuxDeb?: AssetEntry
  linuxTar?: AssetEntry
}

const tag = ref('')
const publishedAt = ref('')
const assets = ref<AssetMap | null>(null)
const failed = ref(false)

const isMac = ref(false)
const isWindows = ref(false)
const isLinux = ref(false)
const macArch = ref<'arm' | 'x86' | null>(null)

function detectPlatform() {
  const ua = navigator.userAgent
  const platform = navigator.platform ?? ''
  isMac.value = /Mac|iPhone|iPad/i.test(ua) || /Mac/i.test(platform)
  isWindows.value = /Win/i.test(ua) || /Win/i.test(platform)
  isLinux.value = !isMac.value && !isWindows.value && /Linux|X11/i.test(ua)
  const uad = (navigator as { userAgentData?: { getHighEntropyValues?: (hints: string[]) => Promise<{ architecture?: string }> } }).userAgentData
  if (!isMac.value || !uad?.getHighEntropyValues) return
  uad.getHighEntropyValues(['architecture'])
    .then(({ architecture }) => {
      if (architecture) macArch.value = /arm|aarch/i.test(architecture) ? 'arm' : 'x86'
    })
    .catch(() => {})
}

function formatSize(size?: number) {
  if (!size) return ''
  const mb = size / 1024 / 1024
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  return `${Math.round(size / 1024)} KB`
}

function button(label: string, ext: string, entry?: AssetEntry) {
  return {
    label,
    ext,
    href: entry?.url ?? LATEST_RELEASE_URL,
    size: formatSize(entry?.size),
    available: Boolean(entry),
  }
}

const rows = computed(() => {
  const a = assets.value
  return [
    {
      os: 'macOS',
      subtitle: isMac.value && macArch.value === 'x86' ? 'Intel 芯片推荐' : 'Apple Silicon 或 Intel',
      recommended: isMac.value,
      buttons: [
        button('Apple Silicon', '.dmg', a?.darwinArm),
        button('Intel 芯片', '.dmg', a?.darwinIntel),
      ],
    },
    {
      os: 'Windows',
      subtitle: 'x64 安装包',
      recommended: isWindows.value,
      buttons: [button('安装包', '.exe', a?.windows)],
    },
    {
      os: 'Linux',
      subtitle: 'x64 · deb 或便携包',
      recommended: isLinux.value,
      buttons: [
        button('deb 包', '.deb', a?.linuxDeb),
        button('便携包', '.tar.gz', a?.linuxTar),
      ],
    },
  ]
})

const dateText = computed(() => {
  if (!publishedAt.value) return ''
  return new Date(publishedAt.value).toLocaleDateString('zh-CN')
})

const guideUrl = withBase('/guide/desktop')

function collectDesktopAssets(release: { assets?: { name?: string; browser_download_url?: string; size?: number }[] }): AssetMap | null {
  const map: AssetMap = {}
  for (const asset of release.assets ?? []) {
    const name = asset.name ?? ''
    if (!name.startsWith('gospeak-desktop-')) continue
    if (!asset.browser_download_url) continue
    const entry = { url: asset.browser_download_url, size: asset.size ?? 0 }
    if (name.includes('darwin-arm64')) map.darwinArm = entry
    else if (name.includes('darwin-amd64')) map.darwinIntel = entry
    else if (name.includes('windows-amd64')) map.windows = entry
    else if (name.includes('linux-amd64.deb')) map.linuxDeb = entry
    else if (name.includes('linux-amd64.tar.gz')) map.linuxTar = entry
  }
  return map.darwinArm || map.darwinIntel || map.windows || map.linuxDeb || map.linuxTar ? map : null
}

async function fetchLatestRelease() {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 8000)
  try {
    const res = await fetch(RELEASES_LIST_API, {
      headers: { Accept: 'application/vnd.github+json' },
      signal: controller.signal,
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const releases = await res.json()
    for (const release of releases ?? []) {
      if (release.draft || release.prerelease) continue
      const map = collectDesktopAssets(release)
      if (!map) continue
      tag.value = release.tag_name
      publishedAt.value = release.published_at ?? ''
      assets.value = map
      return
    }
    failed.value = true
  } catch {
    failed.value = true
  } finally {
    clearTimeout(timer)
  }
}

onMounted(() => {
  detectPlatform()
  fetchLatestRelease()
})
</script>

<template>
  <section id="download" class="client-download">
    <div class="head">
      <h2 class="title">下载客户端</h2>
      <p class="release" :class="{ failed }">
        <template v-if="failed">
          未能获取最新版本，请<a :href="LATEST_RELEASE_URL" target="_blank" rel="noreferrer">前往 Releases 页面</a>下载。
        </template>
        <template v-else-if="!assets">正在获取最新版本…</template>
        <template v-else>
          最新版本
          <a class="tag" :href="LATEST_RELEASE_URL" target="_blank" rel="noreferrer">{{ tag }}</a>
          <template v-if="dateText"> · 发布于 {{ dateText }}</template>
        </template>
      </p>
    </div>

    <div class="grid">
      <article
        v-for="row in rows"
        :key="row.os"
        class="card"
        :class="{ recommended: row.recommended }"
      >
        <header class="card-head">
          <h3 class="os">{{ row.os }}</h3>
          <span v-if="row.recommended" class="badge">当前系统</span>
        </header>
        <p class="subtitle">{{ row.subtitle }}</p>
        <div class="actions">
          <a
            v-for="btn in row.buttons"
            :key="btn.label"
            class="dl"
            :class="{ pending: !btn.available }"
            :href="btn.href"
            target="_blank"
            rel="noreferrer"
          >
            <span class="label">{{ btn.label }}</span>
            <span class="meta">{{ btn.ext }}<template v-if="btn.size"> · {{ btn.size }}</template></span>
          </a>
        </div>
      </article>
    </div>

    <ul class="notes">
      <li>
        安装包未签名：macOS 首次打开需在「系统设置 → 隐私与安全性」中点击「仍要打开」，提示已损坏时执行
        <code>xattr -cr /Applications/GOSpeak.app</code>；Windows 选择「更多信息 → 仍要运行」。
      </li>
      <li>
        首次启动需填写服务器地址，见<a :href="guideUrl">桌面客户端指南</a>；纯浏览器访问无需下载。
      </li>
      <li>
        查找历史版本：<a :href="RELEASES_URL" target="_blank" rel="noreferrer">全部 Releases →</a>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.client-download {
  margin: 56px 0 24px;
}

.title {
  margin: 0 0 8px;
  font-size: 26px;
  font-weight: 600;
  line-height: 1.3;
}

.release {
  margin: 0;
  font-size: 14px;
  color: var(--vp-c-text-2);
}

.release .tag {
  font-weight: 600;
  color: var(--vp-c-brand-1);
}

.release.failed {
  color: var(--vp-c-text-1);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
  margin-top: 24px;
}

.card {
  display: flex;
  flex-direction: column;
  padding: 18px 20px;
  background: linear-gradient(180deg, var(--vp-c-bg-soft), var(--vp-c-bg));
  border: 1px solid var(--vp-c-divider);
  border-radius: 14px;
  transition: transform 0.25s ease, border-color 0.25s ease, box-shadow 0.25s ease;
}

.card:hover {
  transform: translateY(-2px);
  box-shadow: 0 14px 36px -14px rgba(0, 0, 0, 0.2);
}

.dark .card:hover {
  box-shadow: 0 14px 36px -14px rgba(0, 0, 0, 0.55);
}

.card.recommended {
  border-color: var(--vp-c-brand-1);
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.os {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 10px;
  font-size: 12px;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  border-radius: 999px;
}

.badge::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.subtitle {
  margin: 4px 0 14px;
  font-size: 13px;
  color: var(--vp-c-text-2);
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.dl {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 14px;
  font-size: 14px;
  font-weight: 500;
  color: var(--vp-button-brand-text);
  background-color: var(--vp-button-brand-bg);
  background-image: linear-gradient(180deg, rgba(255, 255, 255, 0.16), rgba(255, 255, 255, 0) 55%);
  border-radius: 10px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.12);
  transition: transform 0.2s ease, box-shadow 0.2s ease, background-color 0.25s ease, color 0.25s ease;
}

.dl:hover {
  color: var(--vp-button-brand-hover-text);
  background-color: var(--vp-button-brand-hover-bg);
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.2);
}

.dl.pending {
  color: var(--vp-c-text-1);
  background-color: var(--vp-c-bg-3);
  background-image: none;
  box-shadow: none;
}

.meta {
  font-size: 12px;
  font-weight: 400;
  opacity: 0.85;
}

.notes {
  margin: 24px 0 0;
  padding-left: 20px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--vp-c-text-2);
}

.notes code {
  font-size: 12px;
}
</style>
