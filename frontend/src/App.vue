<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'

const apiBase = (import.meta.env.VITE_API_BASE ?? '').trim()
const apiPrefix = '/v1/echolog'

const site = ref({
  name: 'EchoLog',
  description: '',
  nav: [],
  icpNumber: '',
  icpLinkUrl: '',
})
const posts = ref([])
const currentPost = ref(null)
const loading = ref(true)
const error = ref('')

const route = ref({
  pathname: window.location.pathname,
  search: window.location.search,
})

const secretInput = ref('')
const authLoading = ref(false)
const authError = ref('')
const sessionChecked = ref(false)
const isAuthenticated = ref(false)
const adminLoading = ref(false)
const adminError = ref('')
const settingsSaving = ref(false)
const postSaving = ref(false)
const deleteLoading = ref(false)
const selectedPostId = ref('')
const editorTextarea = ref(null)

const settingsForm = reactive({
  name: '',
  description: '',
  icpNumber: '',
  icpLinkUrl: '',
  nav: [],
})

const postForm = reactive(blankPost())
const adminPosts = ref([])

const isAdminRoute = computed(() => route.value.pathname.startsWith('/thesky9531'))
const selectedSlug = computed(() => new URLSearchParams(route.value.search).get('post') ?? '')
const adminSubPath = computed(() => route.value.pathname.replace(/^\/thesky9531/, '') || '/')
const adminPage = computed(() => {
  const subPath = adminSubPath.value

  if (subPath === '/' || subPath === '/settings') return 'settings'
  if (subPath === '/posts') return 'posts'
  if (subPath === '/posts/new') return 'post-editor'
  if (subPath.startsWith('/posts/edit/')) return 'post-editor'
  return 'settings'
})
const editingPostId = computed(() => {
  const match = adminSubPath.value.match(/^\/posts\/edit\/(.+)$/)
  return match ? decodeURIComponent(match[1]) : ''
})
const isEditorPage = computed(() => adminPage.value === 'post-editor')
const isNewPostPage = computed(() => adminSubPath.value === '/posts/new')
const navItems = computed(() => {
  const alwaysHome = { label: 'Home', href: '/' }
  const rest = Array.isArray(site.value.nav) ? site.value.nav : []
  return [alwaysHome, ...rest]
})
const adminPostOptions = computed(() =>
  adminPosts.value.map((post) => ({
    label: post.title || post.slug,
    value: post.slug,
  })),
)
const renderedCurrentPost = computed(() => renderMarkdown(currentPost.value?.content ?? ''))
const renderedPreview = computed(() => renderMarkdown(postForm.content))
const selectedAdminPost = computed(() =>
  adminPosts.value.find((post) => post.id === selectedPostId.value) ?? null,
)

watch(
  () => [isAdminRoute.value, adminSubPath.value, selectedSlug.value],
  async ([adminRoute]) => {
    if (adminRoute) {
      await loadAdminSession()
      return
    }

    await loadPublicData()
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('popstate', syncRoute)
})

onUnmounted(() => {
  window.removeEventListener('popstate', syncRoute)
})

function blankPost() {
  return {
    id: '',
    slug: '',
    title: '',
    published: false,
    excerpt: '',
    content: '',
    publishedAt: new Date().toISOString().slice(0, 10),
  }
}

function syncRoute() {
  route.value = {
    pathname: window.location.pathname,
    search: window.location.search,
  }
}

function navigateTo(path) {
  window.history.pushState({}, '', path)
  syncRoute()
}

function openAdminPath(path) {
  navigateTo(`/thesky9531${path}`)
}

function isExternalLink(href) {
  return /^https?:\/\//.test(href)
}

function handleLinkClick(href, event) {
  if (!href || isExternalLink(href)) {
    return
  }

  event.preventDefault()
  navigateTo(href)
}

function handleRenderedContentClick(event) {
  const anchor = event.target instanceof Element ? event.target.closest('a') : null
  const href = anchor?.getAttribute('href')

  if (!href || isExternalLink(href) || anchor?.getAttribute('target') === '_blank') {
    return
  }

  event.preventDefault()
  navigateTo(href)
}

async function request(path, options = {}) {
  const requestPath = `${apiPrefix}${path}`
  const requestURL = apiBase ? `${apiBase}${requestPath}` : requestPath
  const response = await fetch(requestURL, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers ?? {}),
    },
    ...options,
  })

  const contentType = response.headers.get('content-type') ?? ''
  const payload = contentType.includes('application/json') ? await response.json() : null

  if (!response.ok) {
    throw new Error(payload?.error ?? 'Request failed')
  }

  return payload
}

async function loadPublicData() {
  try {
    loading.value = true
    error.value = ''

    const [siteData, postsData] = await Promise.all([
      request('/site'),
      request('/posts'),
    ])

    site.value = {
      name: siteData.name ?? 'EchoLog',
      description: siteData.description ?? '',
      nav: siteData.nav ?? [],
      icpNumber: siteData.icpNumber ?? '',
      icpLinkUrl: siteData.icpLinkUrl ?? '',
    }
    posts.value = postsData.items ?? []

    if (selectedSlug.value) {
      currentPost.value = await request(`/posts/${encodeURIComponent(selectedSlug.value)}`)
    } else {
      currentPost.value = null
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load content'
  } finally {
    loading.value = false
  }
}

async function loadAdminSession() {
  try {
    authError.value = ''
    const session = await request('/auth/session')
    isAuthenticated.value = Boolean(session.authenticated)
    sessionChecked.value = true

    if (isAuthenticated.value) {
      await loadAdminData()
    }
  } catch (err) {
    sessionChecked.value = true
    authError.value = err instanceof Error ? err.message : 'Failed to check session'
  }
}

async function loadAdminData() {
  try {
    adminLoading.value = true
    adminError.value = ''

    const [settingsData, postsData] = await Promise.all([
      request('/manage/settings'),
      request('/manage/posts'),
    ])

    applySettingsForm(settingsData)
    adminPosts.value = postsData.items ?? []

    if (isNewPostPage.value) {
      resetPostForm(false)
    } else if (editingPostId.value) {
      const editingPost = adminPosts.value.find((post) => post.id === editingPostId.value)
      if (editingPost) {
        fillPostForm(editingPost)
      } else {
        resetPostForm(false)
      }
    } else if (!selectedPostId.value && adminPosts.value.length > 0) {
      fillPostForm(adminPosts.value[0])
    } else if (!selectedPostId.value) {
      resetPostForm(false)
    }
  } catch (err) {
    adminError.value = err instanceof Error ? err.message : 'Failed to load admin data'
  } finally {
    adminLoading.value = false
  }
}

function applySettingsForm(data) {
  settingsForm.name = data.name ?? ''
  settingsForm.description = data.description ?? ''
  settingsForm.icpNumber = data.icpNumber ?? ''
  settingsForm.icpLinkUrl = data.icpLinkUrl ?? ''
  settingsForm.nav = Array.isArray(data.nav)
    ? data.nav.map((item) => ({
        label: item.label ?? '',
        type: item.type === 'post' ? 'post' : 'url',
        value: item.value ?? '',
      }))
    : []
}

function fillPostForm(post) {
  selectedPostId.value = post.id
  postForm.id = post.id ?? ''
  postForm.slug = post.slug ?? ''
  postForm.title = post.title ?? ''
  postForm.published = Boolean(post.published)
  postForm.excerpt = post.excerpt ?? ''
  postForm.content = post.content ?? ''
  postForm.publishedAt = post.publishedAt ?? new Date().toISOString().slice(0, 10)
}

function resetPostForm(navigate = true) {
  if (navigate) {
    openAdminPath('/posts/new')
  }
  selectedPostId.value = ''
  Object.assign(postForm, blankPost())
}

function addNavItem() {
  settingsForm.nav.push({
    label: '',
    type: 'url',
    value: '',
  })
}

function removeNavItem(index) {
  settingsForm.nav.splice(index, 1)
}

async function login() {
  try {
    authLoading.value = true
    authError.value = ''

    await request('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ secret: secretInput.value }),
    })

    secretInput.value = ''
    isAuthenticated.value = true
    await loadAdminData()
  } catch (err) {
    authError.value = err instanceof Error ? err.message : 'Login failed'
  } finally {
    sessionChecked.value = true
    authLoading.value = false
  }
}

async function logout() {
  await request('/auth/logout', { method: 'POST' })
  isAuthenticated.value = false
  selectedPostId.value = ''
  adminPosts.value = []
  resetPostForm()
}

async function saveSettings() {
  try {
    settingsSaving.value = true
    adminError.value = ''

    const saved = await request('/manage/settings', {
      method: 'PUT',
      body: JSON.stringify({
        name: settingsForm.name,
        description: settingsForm.description,
        icpNumber: settingsForm.icpNumber,
        icpLinkUrl: settingsForm.icpLinkUrl,
        nav: settingsForm.nav,
      }),
    })

    applySettingsForm(saved)
  } catch (err) {
    adminError.value = err instanceof Error ? err.message : 'Failed to save settings'
  } finally {
    settingsSaving.value = false
  }
}

async function savePost() {
  try {
    postSaving.value = true
    adminError.value = ''
    const isCreating = !postForm.id

    const payload = {
      slug: postForm.slug,
      title: postForm.title,
      published: postForm.published,
      excerpt: postForm.excerpt,
      content: postForm.content,
      publishedAt: postForm.publishedAt,
    }

    const saved = postForm.id
      ? await request(`/manage/posts/${postForm.id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        })
      : await request('/manage/posts', {
          method: 'POST',
          body: JSON.stringify(payload),
        })

    await loadAdminData()
    fillPostForm(saved)
    if (isCreating) {
      openAdminPath('/posts')
    } else {
      openAdminPath(`/posts/edit/${encodeURIComponent(saved.id)}`)
    }
  } catch (err) {
    adminError.value = err instanceof Error ? err.message : 'Failed to save post'
  } finally {
    postSaving.value = false
  }
}

async function deletePost() {
  if (!postForm.id || !window.confirm(`Delete "${postForm.title || postForm.slug}"?`)) {
    return
  }

  try {
    deleteLoading.value = true
    adminError.value = ''

    await request(`/manage/posts/${postForm.id}`, {
      method: 'DELETE',
    })

    await loadAdminData()
    if (adminPosts.value.length > 0) {
      fillPostForm(adminPosts.value[0])
      openAdminPath('/posts')
    } else {
      resetPostForm(false)
      openAdminPath('/posts')
    }
  } catch (err) {
    adminError.value = err instanceof Error ? err.message : 'Failed to delete post'
  } finally {
    deleteLoading.value = false
  }
}

function formatDate(dateText) {
  const date = new Date(dateText)
  if (Number.isNaN(date.getTime())) {
    return dateText
  }

  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function escapeHTML(value) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function renderInlineMarkdown(value) {
  return escapeHTML(value)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
    .replace(/\[([^\]]+)\]\(((?:\/|\?)[^\s)]*)\)/g, '<a href="$2">$1</a>')
}

function renderMarkdown(source) {
  const lines = (source || '').replaceAll('\r\n', '\n').split('\n')
  const html = []
  let paragraph = []
  let listType = ''
  let codeFence = false
  let codeLines = []

  function flushParagraph() {
    if (!paragraph.length) return
    html.push(`<p>${renderInlineMarkdown(paragraph.join(' '))}</p>`)
    paragraph = []
  }

  function flushList() {
    if (!listType) return
    html.push(`</${listType}>`)
    listType = ''
  }

  function flushCodeFence() {
    if (!codeFence) return
    html.push(`<pre><code>${escapeHTML(codeLines.join('\n'))}</code></pre>`)
    codeFence = false
    codeLines = []
  }

  for (const rawLine of lines) {
    const line = rawLine.trimEnd()
    const trimmed = line.trim()

    if (trimmed.startsWith('```')) {
      flushParagraph()
      flushList()
      if (codeFence) {
        flushCodeFence()
      } else {
        codeFence = true
      }
      continue
    }

    if (codeFence) {
      codeLines.push(line)
      continue
    }

    if (!trimmed) {
      flushParagraph()
      flushList()
      continue
    }

    const headingMatch = trimmed.match(/^(#{1,6})\s+(.*)$/)
    if (headingMatch) {
      flushParagraph()
      flushList()
      const level = headingMatch[1].length
      html.push(`<h${level}>${renderInlineMarkdown(headingMatch[2])}</h${level}>`)
      continue
    }

    const quoteMatch = trimmed.match(/^>\s?(.*)$/)
    if (quoteMatch) {
      flushParagraph()
      flushList()
      html.push(`<blockquote><p>${renderInlineMarkdown(quoteMatch[1])}</p></blockquote>`)
      continue
    }

    const unorderedMatch = trimmed.match(/^[-*]\s+(.*)$/)
    if (unorderedMatch) {
      flushParagraph()
      if (listType !== 'ul') {
        flushList()
        listType = 'ul'
        html.push('<ul>')
      }
      html.push(`<li>${renderInlineMarkdown(unorderedMatch[1])}</li>`)
      continue
    }

    const orderedMatch = trimmed.match(/^\d+\.\s+(.*)$/)
    if (orderedMatch) {
      flushParagraph()
      if (listType !== 'ol') {
        flushList()
        listType = 'ol'
        html.push('<ol>')
      }
      html.push(`<li>${renderInlineMarkdown(orderedMatch[1])}</li>`)
      continue
    }

    flushList()
    paragraph.push(trimmed)
  }

  flushParagraph()
  flushList()
  flushCodeFence()

  return html.join('')
}

function insertAroundSelection(prefix, suffix = '', placeholder = 'text') {
  const textarea = editorTextarea.value
  if (!textarea) {
    postForm.content += `${prefix}${placeholder}${suffix}`
    return
  }

  const start = textarea.selectionStart
  const end = textarea.selectionEnd
  const selected = postForm.content.slice(start, end) || placeholder
  postForm.content =
    postForm.content.slice(0, start) +
    prefix +
    selected +
    suffix +
    postForm.content.slice(end)

  nextTick(() => {
    textarea.focus()
    const cursorStart = start + prefix.length
    const cursorEnd = cursorStart + selected.length
    textarea.setSelectionRange(cursorStart, cursorEnd)
  })
}

function insertBlock(block) {
  const textarea = editorTextarea.value
  if (!textarea) {
    postForm.content += `${postForm.content ? '\n\n' : ''}${block}`
    return
  }

  const start = textarea.selectionStart
  const insertion = `${start > 0 ? '\n\n' : ''}${block}`
  postForm.content =
    postForm.content.slice(0, start) +
    insertion +
    postForm.content.slice(textarea.selectionEnd)

  nextTick(() => {
    textarea.focus()
    const cursor = start + insertion.length
    textarea.setSelectionRange(cursor, cursor)
  })
}
</script>

<template>
  <div v-if="isAdminRoute" class="admin-layout">
    <header v-if="!isEditorPage" class="admin-topbar">
      <div>
        <p class="eyebrow">EchoLog 后台</p>
        <h1 class="admin-title">内容管理</h1>
      </div>
      <a href="/" class="ghost-link" @click="handleLinkClick('/', $event)">返回站点</a>
    </header>

    <main class="admin-main">
      <p v-if="!sessionChecked" class="state">正在检查登录状态...</p>

      <form v-else-if="!isAuthenticated" class="auth-card" @submit.prevent="login">
        <h2>后台登录</h2>
        <p class="panel-copy">请输入后端校验的管理秘钥后继续。</p>
        <label class="field">
          <span>管理秘钥</span>
          <input v-model="secretInput" type="password" placeholder="请输入管理秘钥" />
        </label>
        <p v-if="authError" class="state state-error">{{ authError }}</p>
        <button class="primary-button" type="submit" :disabled="authLoading">
          {{ authLoading ? '登录中...' : '进入后台' }}
        </button>
      </form>

      <p v-else-if="adminLoading" class="state">正在加载后台内容...</p>

      <section v-else-if="isEditorPage" class="panel admin-editor-workspace">
        <div class="editor-workspace-bar">
          <div>
            <p class="eyebrow">文章编辑</p>
            <h2>{{ isNewPostPage ? '新建文章' : postForm.title || '编辑文章' }}</h2>
          </div>
          <div class="button-row">
            <button class="ghost-button" type="button" @click="openAdminPath('/posts')">返回列表</button>
            <button class="primary-button" type="button" :disabled="postSaving" @click="savePost">
              {{ postSaving ? '保存中...' : postForm.id ? '保存' : '发布' }}
            </button>
            <button
              v-if="postForm.id"
              class="danger-button"
              type="button"
              :disabled="deleteLoading"
              @click="deletePost"
            >
              {{ deleteLoading ? '删除中...' : '删除' }}
            </button>
          </div>
        </div>

        <div class="editor-document-meta">
          <label class="field">
            <span>标题</span>
            <input v-model="postForm.title" type="text" placeholder="请输入文章标题" />
          </label>
          <label class="field">
            <span>Slug</span>
            <input v-model="postForm.slug" type="text" placeholder="article-slug" />
          </label>
          <label class="field">
            <span>发布日期</span>
            <input v-model="postForm.publishedAt" type="date" />
          </label>
          <label class="field">
            <span>发布状态</span>
            <select v-model="postForm.published">
              <option :value="true">已发布</option>
              <option :value="false">未发布</option>
            </select>
          </label>
        </div>

        <div class="editor-workspace-layout">
          <div class="editor-document">
            <div class="editor-toolbar">
              <button class="ghost-button" type="button" @click="insertBlock('# 标题')">标题</button>
              <button class="ghost-button" type="button" @click="insertAroundSelection('**', '**', '加粗文本')">加粗</button>
              <button class="ghost-button" type="button" @click="insertAroundSelection('*', '*', '斜体文本')">斜体</button>
              <button class="ghost-button" type="button" @click="insertAroundSelection('[', '](https://example.com)', '链接文字')">链接</button>
              <button class="ghost-button" type="button" @click="insertBlock('- 列表项')">列表</button>
              <button class="ghost-button" type="button" @click="insertBlock('> 引用')">引用</button>
              <button class="ghost-button" type="button" @click="insertBlock('```\\ncode\\n```')">代码块</button>
            </div>

            <textarea
              ref="editorTextarea"
              v-model="postForm.content"
              class="editor-textarea editor-textarea-immersive"
              rows="28"
              placeholder="开始用 Markdown 写文章..."
            />
          </div>

          <aside class="editor-preview-pane">
            <div class="panel-subhead">
              <div>
                <p class="eyebrow">实时预览</p>
                <h3>渲染效果</h3>
              </div>
            </div>
            <div class="markdown-body" v-html="renderedPreview" />
          </aside>
        </div>

        <p v-if="adminError" class="state state-error">{{ adminError }}</p>
      </section>

      <div v-else class="admin-grid">
        <aside class="panel admin-sidebar">
          <div class="panel-head">
            <div>
              <p class="eyebrow">导航</p>
              <h2>后台模块</h2>
            </div>
          </div>

          <div class="admin-module-list">
            <button
              type="button"
              class="admin-module-item"
              :class="{ active: adminPage === 'settings' }"
              @click="openAdminPath('/settings')"
            >
              <strong>通用设置</strong>
              <span>站点名称、备案号、导航等基础配置。</span>
            </button>
            <button
              type="button"
              class="admin-module-item"
              :class="{ active: adminPage === 'posts' || isEditorPage }"
              @click="openAdminPath('/posts')"
            >
              <strong>文章管理</strong>
              <span>列表和元信息在这里，正文进入详情页编辑。</span>
            </button>
            <div class="admin-module-placeholder">
              <p class="eyebrow">后续扩展</p>
              <p>以后继续增加模块时，左侧导航和右侧功能区结构都可以直接复用。</p>
            </div>
          </div>

          <div class="admin-sidebar-footer">
            <button class="ghost-button" type="button" @click="logout">退出登录</button>
          </div>
        </aside>

        <section v-if="adminPage === 'settings'" class="panel">
          <div class="panel-head">
            <div>
              <p class="eyebrow">通用设置</p>
              <h2>站点基础信息</h2>
            </div>
          </div>

          <label class="field">
            <span>站点名称</span>
            <input v-model="settingsForm.name" type="text" placeholder="EchoLog" />
          </label>

          <label class="field">
            <span>站点描述</span>
            <textarea
              v-model="settingsForm.description"
              rows="3"
              placeholder="填写首页展示的站点简介"
            />
          </label>

          <div class="field-row">
            <label class="field">
              <span>备案号</span>
              <input v-model="settingsForm.icpNumber" type="text" placeholder="请输入备案号" />
            </label>
            <label class="field">
              <span>备案链接</span>
              <input v-model="settingsForm.icpLinkUrl" type="url" placeholder="https://beian.miit.gov.cn/" />
            </label>
          </div>

          <div class="panel-subhead">
            <div>
              <p class="eyebrow">导航配置</p>
              <h3>头部导航栏</h3>
            </div>
            <button class="ghost-button" type="button" @click="addNavItem">新增导航</button>
          </div>

          <div v-if="settingsForm.nav.length === 0" class="state">当前还没有导航项。</div>
          <div v-else class="nav-editor">
            <div v-for="(item, index) in settingsForm.nav" :key="index" class="nav-editor-row">
              <label class="field">
                <span>名称</span>
                <input v-model="item.label" type="text" placeholder="导航名称" />
              </label>
              <label class="field">
                <span>类型</span>
                <select v-model="item.type">
                  <option value="url">外部链接或站内地址</option>
                  <option value="post">文章</option>
                </select>
              </label>
              <label class="field field-grow">
                <span>{{ item.type === 'post' ? '文章 slug' : 'URL 或路径' }}</span>
                <select v-if="item.type === 'post'" v-model="item.value">
                  <option disabled value="">请选择文章</option>
                  <option v-for="post in adminPostOptions" :key="post.value" :value="post.value">
                    {{ post.label }}
                  </option>
                </select>
                <input v-else v-model="item.value" type="text" placeholder="https://..." />
              </label>
              <button class="danger-button" type="button" @click="removeNavItem(index)">删除</button>
            </div>
          </div>

          <p v-if="adminError" class="state state-error">{{ adminError }}</p>
          <button class="primary-button" type="button" :disabled="settingsSaving" @click="saveSettings">
            {{ settingsSaving ? '保存中...' : '保存设置' }}
          </button>
        </section>

        <section v-else-if="adminPage === 'posts'" class="panel">
          <div class="panel-head">
            <div>
              <p class="eyebrow">文章管理</p>
              <h2>文章列表</h2>
            </div>
          </div>

          <div class="editor-layout dashboard-posts-layout">
            <aside class="post-list">
              <button type="button" class="post-list-create" @click="openAdminPath('/posts/new')">
                + 新建文章
              </button>
              <button
                v-for="post in adminPosts"
                :key="post.id"
                type="button"
                class="post-list-item"
                :class="{ active: selectedPostId === post.id }"
                @click="fillPostForm(post)"
              >
                <strong>{{ post.title }}</strong>
                <span class="post-meta-line">{{ post.slug }}</span>
                <span class="post-status" :class="{ published: post.published, draft: !post.published }">
                  {{ post.published ? '已发布' : '未发布' }}
                </span>
              </button>
            </aside>

            <div v-if="selectedAdminPost" class="editor-form">
              <div class="panel-subhead">
                <div>
                  <p class="eyebrow">文章信息</p>
                  <h3>{{ selectedAdminPost.title }}</h3>
                </div>
              </div>

              <div class="details-form">
                <label class="detail-row">
                  <span class="detail-label">标题</span>
                  <input v-model="postForm.title" class="compact-input" type="text" placeholder="请输入文章标题" />
                </label>
                <label class="detail-row">
                  <span class="detail-label">Slug</span>
                  <input v-model="postForm.slug" class="compact-input" type="text" placeholder="article-slug" />
                </label>
                <label class="detail-row">
                  <span class="detail-label">发布日期</span>
                  <input v-model="postForm.publishedAt" class="compact-input" type="date" />
                </label>
                <label class="detail-row">
                  <span class="detail-label">发布状态</span>
                  <select v-model="postForm.published" class="compact-input">
                    <option :value="true">已发布</option>
                    <option :value="false">未发布</option>
                  </select>
                </label>
                <label class="detail-row">
                  <span class="detail-label">摘要</span>
                  <textarea
                    v-model="postForm.excerpt"
                    class="compact-input compact-textarea"
                    rows="3"
                    placeholder="用于列表展示的简短摘要"
                  />
                </label>
              </div>

              <div class="button-row">
                <button class="ghost-button" type="button" @click="openAdminPath(`/posts/edit/${selectedAdminPost.id}`)">
                  内容编辑
                </button>
                <button class="primary-button" type="button" :disabled="postSaving" @click="savePost">
                  {{ postSaving ? '保存中...' : '保存信息' }}
                </button>
                <button
                  v-if="selectedAdminPost"
                  class="danger-button"
                  type="button"
                  :disabled="deleteLoading"
                  @click="deletePost"
                >
                  {{ deleteLoading ? '删除中...' : '删除文章' }}
                </button>
              </div>
            </div>

            <div v-else class="empty-module-state">
              <p class="eyebrow">文章</p>
              <h3>尚未选中文章</h3>
              <p>从左侧列表选择一篇文章，或创建一篇新的文章开始编辑。</p>
            </div>
          </div>

          <p v-if="adminError" class="state state-error">{{ adminError }}</p>
        </section>
      </div>
    </main>
  </div>

  <div v-else class="site-layout">
    <header class="site-topbar">
      <div class="brand-block">
        <p class="eyebrow">EchoLog</p>
        <h1 class="brand">{{ site.name }}</h1>
        <p v-if="site.description" class="site-description">{{ site.description }}</p>
      </div>
      <nav class="site-nav">
        <a
          v-for="item in navItems"
          :key="item.label"
          :href="item.href"
          class="nav-link"
          :target="isExternalLink(item.href) ? '_blank' : undefined"
          :rel="isExternalLink(item.href) ? 'noreferrer' : undefined"
          @click="handleLinkClick(item.href, $event)"
        >
          {{ item.label }}
        </a>
      </nav>
    </header>

    <main class="content">
      <p v-if="loading" class="state">Loading content...</p>
      <p v-else-if="error" class="state state-error">{{ error }}</p>

      <template v-else-if="currentPost">
        <article class="article-card">
          <a href="/" class="ghost-link" @click="handleLinkClick('/', $event)">Back to homepage</a>
          <h2 class="article-title">{{ currentPost.title }}</h2>
          <time class="post-date">{{ formatDate(currentPost.publishedAt) }}</time>
          <p v-if="currentPost.excerpt" class="article-excerpt">{{ currentPost.excerpt }}</p>
          <div class="article-content markdown-body" @click="handleRenderedContentClick" v-html="renderedCurrentPost" />
        </article>
      </template>

      <section v-else class="posts-grid">
        <article v-for="post in posts" :key="post.id" class="post-card">
          <p class="post-date">{{ formatDate(post.publishedAt) }}</p>
          <h2 class="post-title">{{ post.title }}</h2>
          <p class="post-excerpt">{{ post.excerpt }}</p>
          <a
            class="post-link"
            :href="`/?post=${post.slug}`"
            @click="handleLinkClick(`/?post=${post.slug}`, $event)"
          >
            Read post
          </a>
        </article>
      </section>
    </main>

    <footer v-if="site.icpNumber && site.icpLinkUrl" class="footer">
      <a :href="site.icpLinkUrl" class="icp-link" target="_blank" rel="noreferrer">
        {{ site.icpNumber }}
      </a>
    </footer>
  </div>
</template>
