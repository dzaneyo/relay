<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  createRecord,
  createRoute,
  deleteRecord,
  deleteRoute,
  getRecord,
  listRecords,
  listRoutes,
  listTags,
  updateRecord,
  updateRoute,
} from './api'
import type {
  AuthType,
  DBType,
  RecordCategory,
  RecordInput,
  RecordSummary,
  SSHRoute,
} from './types'

type Tab = 'records' | 'routes'

interface RecordForm {
  name: string
  alias: string
  category: RecordCategory
  notes: string
  favorite: boolean
  tags: string
  username: string
  password: string
  authType: AuthType
  keyPath: string
  host: string
  port: number
  routeId: string
  dbType: DBType
  databaseName: string
}

interface RouteForm {
  name: string
  description: string
  hostIds: string[]
}

const tab = ref<Tab>('records')
const records = ref<RecordSummary[]>([])
const routes = ref<SSHRoute[]>([])
const hostOptions = ref<RecordSummary[]>([])
const search = ref('')
const categoryFilter = ref<RecordCategory | ''>('')
const selectedTags = ref<string[]>([])
const availableTags = ref<string[]>([])
const selectedRecordId = ref('')
const selectedRouteId = ref('')
const recordDrawerOpen = ref(false)
const routeDrawerOpen = ref(false)
const hasExistingPassword = ref(false)
const originalHasExistingPassword = ref(false)
const originalRecordCategory = ref<RecordCategory>('NOTE')
const loading = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const error = ref('')
const success = ref('')

const recordForm = reactive<RecordForm>(emptyRecordForm())
const recordDrafts = reactive<Partial<Record<RecordCategory, RecordForm>>>({})
const routeForm = reactive<RouteForm>({ name: '', description: '', hostIds: [] })

const editingRecord = computed(() => selectedRecordId.value !== '')
const editingRoute = computed(() => selectedRouteId.value !== '')
const availableHopHosts = computed(() =>
  hostOptions.value.filter((host) => !routeForm.hostIds.includes(host.id)),
)

function emptyRecordForm(category: RecordCategory = 'NOTE'): RecordForm {
  return {
    name: '',
    alias: '',
    category,
    notes: '',
    favorite: false,
    tags: '',
    username: '',
    password: '',
    authType: category === 'HOST' ? 'NONE' : 'PASSWORD',
    keyPath: '',
    host: '',
    port: category === 'DATABASE' ? 3306 : 22,
    routeId: '',
    dbType: 'MYSQL',
    databaseName: '',
  }
}

function messageOf(value: unknown): string {
  return value instanceof Error ? value.message : String(value)
}

function clearMessages() {
  error.value = ''
  success.value = ''
}

async function loadRecordList() {
  loading.value = true
  clearMessages()
  try {
    records.value = await listRecords(search.value, categoryFilter.value, selectedTags.value)
  } catch (value) {
    error.value = messageOf(value)
  } finally {
    loading.value = false
  }
}

async function refreshTags() {
  availableTags.value = await listTags()
}

function toggleTagFilter(tag: string) {
  selectedTags.value = selectedTags.value.includes(tag)
    ? selectedTags.value.filter((item) => item !== tag)
    : [...selectedTags.value, tag]
  void loadRecordList()
}

function clearRecordDrafts() {
  delete recordDrafts.NOTE
  delete recordDrafts.HOST
  delete recordDrafts.DATABASE
}

function snapshotRecordForm(): RecordForm {
  return { ...recordForm }
}

async function refreshRoutes() {
  routes.value = await listRoutes()
}

async function refreshHostOptions() {
  const all = await listRecords()
  const hosts = all.filter((record) => record.category === 'HOST')
  const details = await Promise.all(hosts.map((host) => getRecord(host.id)))
  hostOptions.value = hosts.filter((_, index) => !details[index].ssh?.routeId)
}

async function loadRouteData() {
  loading.value = true
  clearMessages()
  try {
    await Promise.all([refreshRoutes(), refreshHostOptions()])
  } catch (value) {
    error.value = messageOf(value)
  } finally {
    loading.value = false
  }
}

async function switchTab(next: Tab) {
  if (tab.value === next) return
  if (saving.value || detailLoading.value) return
  recordDrawerOpen.value = false
  routeDrawerOpen.value = false
  tab.value = next
  clearMessages()
  if (next === 'routes') await loadRouteData()
  else await loadRecordList()
}

function newRecord(category: RecordCategory = 'NOTE') {
  selectedRecordId.value = ''
  hasExistingPassword.value = false
  originalHasExistingPassword.value = false
  originalRecordCategory.value = category
  clearRecordDrafts()
  Object.assign(recordForm, emptyRecordForm(category))
  recordDrafts[category] = snapshotRecordForm()
  clearMessages()
  recordDrawerOpen.value = true
}

function closeRecordDrawer() {
  if (saving.value || detailLoading.value) return
  recordDrawerOpen.value = false
}

function changeCategory(category: RecordCategory) {
  if (recordForm.category === category) return
  recordDrafts[recordForm.category] = snapshotRecordForm()
  const common = {
    name: recordForm.name,
    notes: recordForm.notes,
    favorite: recordForm.favorite,
    tags: recordForm.tags,
  }
  Object.assign(recordForm, recordDrafts[category] ?? { ...emptyRecordForm(category), ...common })
  hasExistingPassword.value = editingRecord.value
    && category === originalRecordCategory.value
    && originalHasExistingPassword.value
}

async function selectRecord(id: string) {
  selectedRecordId.value = id
  hasExistingPassword.value = false
  detailLoading.value = true
  recordDrawerOpen.value = true
  clearMessages()
  try {
    const detail = await getRecord(id)
    const credential = detail.credential
    originalRecordCategory.value = detail.record.category
    originalHasExistingPassword.value = credential?.authType === 'PASSWORD'
    hasExistingPassword.value = originalHasExistingPassword.value
    Object.assign(recordForm, emptyRecordForm(detail.record.category), {
      name: detail.record.name,
      alias: detail.record.alias,
      notes: detail.record.notes,
      favorite: detail.record.favorite,
      tags: (detail.record.tags ?? []).join(', '),
      username: credential?.username ?? '',
      password: '',
      authType: credential?.authType ?? 'NONE',
      keyPath: credential?.keyPath ?? '',
      host: detail.ssh?.host ?? detail.database?.host ?? '',
      port: detail.ssh?.port ?? detail.database?.port ?? 0,
      routeId: detail.ssh?.routeId ?? '',
      dbType: detail.database?.dbType ?? 'MYSQL',
      databaseName: detail.database?.databaseName ?? '',
    })
    clearRecordDrafts()
    recordDrafts[detail.record.category] = snapshotRecordForm()
  } catch (value) {
    error.value = messageOf(value)
  } finally {
    detailLoading.value = false
  }
}

function changeAuthType(authType: AuthType) {
  recordForm.authType = authType
  recordForm.password = ''
  recordForm.keyPath = ''
}

function databaseDefaultPort(dbType: DBType): number {
  if (dbType === 'POSTGRESQL') return 5432
  if (dbType === 'DORIS') return 9030
  return 3306
}

function changeDatabaseType(dbType: DBType) {
  const oldDefault = databaseDefaultPort(recordForm.dbType)
  if (!recordForm.port || recordForm.port === oldDefault) {
    recordForm.port = databaseDefaultPort(dbType)
  }
  recordForm.dbType = dbType
}

function recordPayload(): RecordInput {
  const base: RecordInput = {
    name: recordForm.name,
    alias: recordForm.category === 'NOTE' ? '' : recordForm.alias,
    category: recordForm.category,
    notes: recordForm.notes,
    favorite: recordForm.favorite,
    tags: recordForm.tags.split(',').map((tag) => tag.trim()).filter(Boolean),
  }
  if (recordForm.category === 'HOST') {
    base.credential = {
      username: recordForm.username,
      authType: recordForm.authType,
      secretValue: recordForm.authType === 'PASSWORD' ? recordForm.password : '',
      keyPath: recordForm.authType === 'SSH_KEY' ? recordForm.keyPath : '',
    }
    base.ssh = {
      host: recordForm.host,
      port: Number(recordForm.port),
      routeId: recordForm.routeId,
    }
  } else if (recordForm.category === 'DATABASE') {
    base.credential = {
      username: recordForm.username,
      authType: 'PASSWORD',
      secretValue: recordForm.password,
    }
    base.database = {
      dbType: recordForm.dbType,
      host: recordForm.host,
      port: Number(recordForm.port),
      databaseName: recordForm.databaseName,
    }
  }
  return base
}

async function saveRecord() {
  if (editingRecord.value && recordForm.category !== originalRecordCategory.value
    && !window.confirm(`Convert ${originalRecordCategory.value} to ${recordForm.category}? The previous connection configuration will be removed.`)) return
  saving.value = true
  clearMessages()
  try {
    const detail = editingRecord.value
      ? await updateRecord(selectedRecordId.value, recordPayload())
      : await createRecord(recordPayload())
    selectedRecordId.value = detail.record.id
    hasExistingPassword.value = detail.credential?.authType === 'PASSWORD'
    originalHasExistingPassword.value = hasExistingPassword.value
    originalRecordCategory.value = detail.record.category
    recordForm.password = ''
    clearRecordDrafts()
    recordDrafts[recordForm.category] = snapshotRecordForm()
    await Promise.all([loadRecordList(), refreshRoutes(), refreshTags()])
    success.value = 'Record saved.'
  } catch (value) {
    error.value = messageOf(value)
  } finally {
    saving.value = false
  }
}

async function removeRecord() {
  if (!selectedRecordId.value || !window.confirm(`Delete “${recordForm.name}”?`)) return
  saving.value = true
  clearMessages()
  try {
    await deleteRecord(selectedRecordId.value)
    selectedRecordId.value = ''
    hasExistingPassword.value = false
    Object.assign(recordForm, emptyRecordForm())
    recordDrawerOpen.value = false
    await loadRecordList()
    success.value = 'Record deleted.'
  } catch (value) {
    error.value = messageOf(value)
  } finally {
    saving.value = false
  }
}

async function copyRecordNotes(record: RecordSummary, event: MouseEvent) {
  event.stopPropagation()
  const content = record.notes.trim()
  if (!content) return
  clearMessages()
  try {
    if (!navigator.clipboard) throw new Error('Clipboard access is unavailable.')
    await navigator.clipboard.writeText(content)
    success.value = 'Copied.'
  } catch (value) {
    error.value = messageOf(value)
  }
}

function newRoute() {
  selectedRouteId.value = ''
  Object.assign(routeForm, { name: '', description: '', hostIds: [] })
  clearMessages()
  routeDrawerOpen.value = true
}

function closeRouteDrawer() {
  if (saving.value) return
  routeDrawerOpen.value = false
}

function selectRoute(route: SSHRoute) {
  selectedRouteId.value = route.id
  Object.assign(routeForm, {
    name: route.name,
    description: route.description,
    hostIds: [...route.hops].sort((a, b) => a.seq - b.seq).map((hop) => hop.hostRecordId),
  })
  clearMessages()
  routeDrawerOpen.value = true
}

function hostName(id: string): string {
  const host = hostOptions.value.find((item) => item.id === id)
  return host ? `${host.name} (${host.alias})` : id
}

function addHop(event: Event) {
  const select = event.target as HTMLSelectElement
  if (select.value && !routeForm.hostIds.includes(select.value)) routeForm.hostIds.push(select.value)
  select.value = ''
}

function moveHop(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= routeForm.hostIds.length) return
  const next = [...routeForm.hostIds]
  ;[next[index], next[target]] = [next[target], next[index]]
  routeForm.hostIds = next
}

function removeHop(index: number) {
  routeForm.hostIds.splice(index, 1)
}

async function saveRoute() {
  saving.value = true
  clearMessages()
  const payload = {
    name: routeForm.name,
    description: routeForm.description,
    hops: routeForm.hostIds.map((hostRecordId, index) => ({ seq: index + 1, hostRecordId })),
  }
  try {
    const saved = editingRoute.value
      ? await updateRoute(selectedRouteId.value, payload)
      : await createRoute(payload)
    await loadRouteData()
    selectRoute(routes.value.find((route) => route.id === saved.id) ?? saved)
    success.value = 'Route saved.'
  } catch (value) {
    error.value = messageOf(value)
  } finally {
    saving.value = false
  }
}

async function removeRoute() {
  if (!selectedRouteId.value || !window.confirm(`Delete route “${routeForm.name}”?`)) return
  saving.value = true
  clearMessages()
  try {
    await deleteRoute(selectedRouteId.value)
    selectedRouteId.value = ''
    Object.assign(routeForm, { name: '', description: '', hostIds: [] })
    routeDrawerOpen.value = false
    await loadRouteData()
    success.value = 'Route deleted.'
  } catch (value) {
    error.value = messageOf(value)
  } finally {
    saving.value = false
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (recordDrawerOpen.value) closeRecordDrawer()
  else if (routeDrawerOpen.value) closeRouteDrawer()
}

onMounted(async () => {
  window.addEventListener('keydown', handleKeydown)
  await Promise.all([loadRecordList(), refreshRoutes(), refreshTags()])
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <main class="app-shell">
    <header class="topbar">
      <div class="brand">
        <span class="brand-mark">R</span>
        <div><h1>relay</h1><p>Local connection manager</p></div>
      </div>
      <nav class="tabs" aria-label="Sections">
        <button :class="{ active: tab === 'records' }" type="button" @click="switchTab('records')">Records</button>
        <button :class="{ active: tab === 'routes' }" type="button" @click="switchTab('routes')">Routes</button>
      </nav>
    </header>

    <div v-if="error" class="notice error" role="alert">{{ error }}</div>
    <div v-else-if="success" class="notice success" role="status">{{ success }}</div>

    <section v-if="tab === 'records'" class="list-page">
      <div class="page-heading">
        <div><span class="eyebrow">Library</span><h2>Records</h2></div>
        <button class="primary" type="button" @click="newRecord()">New record</button>
      </div>
      <form class="search search-wide" role="search" @submit.prevent="loadRecordList">
        <input v-model="search" type="search" placeholder="Search name, alias, notes" aria-label="Search records" />
        <select v-model="categoryFilter" aria-label="Filter by record type" @change="loadRecordList">
          <option value="">All types</option>
          <option value="HOST">HOST</option>
          <option value="DATABASE">DATABASE</option>
          <option value="NOTE">NOTE</option>
        </select>
        <button type="submit">Search</button>
      </form>
      <div v-if="availableTags.length" class="tag-filters" aria-label="Filter by tags">
        <button v-for="tag in availableTags" :key="tag" type="button" :class="{ active: selectedTags.includes(tag) }" :aria-pressed="selectedTags.includes(tag)" @click="toggleTagFilter(tag)">{{ tag }}</button>
      </div>
      <div class="list" :aria-busy="loading">
        <p v-if="loading" class="empty">Loading records…</p>
        <p v-else-if="records.length === 0" class="empty bordered">No records found.<br />Create one to get started.</p>
        <div
          v-for="record in records"
          v-else
          :key="record.id"
          class="list-item"
          :class="{ selected: selectedRecordId === record.id && recordDrawerOpen }"
          role="button"
          tabindex="0"
          :aria-label="`Open ${record.name}`"
          @click="selectRecord(record.id)"
          @keydown.enter.self.prevent="selectRecord(record.id)"
          @keydown.space.self.prevent="selectRecord(record.id)"
        >
          <div class="list-main">
            <div class="list-title-row">
              <span class="list-title"><span v-if="record.favorite" class="favorite" title="Favorite">★</span>{{ record.name }}</span>
              <span class="badge">{{ record.category }}</span>
              <code v-if="record.category !== 'NOTE'" class="list-alias">{{ record.alias }}</code>
              <span v-for="tag in record.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
              <span v-if="record.tags.length > 3" class="tag more">+{{ record.tags.length - 3 }}</span>
            </div>
            <p class="list-summary" :aria-hidden="!record.notes.trim() && record.category !== 'NOTE'">{{ record.notes.trim() || (record.category === 'NOTE' ? 'No notes' : '\u00a0') }}</p>
          </div>
          <button
            v-if="record.category === 'NOTE' || record.category === 'HOST'"
            class="copy-button"
            type="button"
            :disabled="!record.notes.trim()"
            :aria-label="`Copy ${record.category === 'NOTE' ? 'note content' : 'host notes'} for ${record.name}`"
            @click="copyRecordNotes(record, $event)"
          >Copy</button>
        </div>
      </div>
    </section>

    <section v-else class="list-page">
      <div class="page-heading">
        <div><span class="eyebrow">SSH</span><h2>Routes</h2><p class="page-description">Reusable multi-hop connection paths.</p></div>
        <button class="primary" type="button" @click="newRoute">New route</button>
      </div>
      <div class="list" :aria-busy="loading">
        <p v-if="loading" class="empty">Loading routes…</p>
        <p v-else-if="routes.length === 0" class="empty bordered">No routes yet.<br />Add a HOST first, then create a route.</p>
        <div
          v-for="route in routes"
          v-else
          :key="route.id"
          class="list-item route-item"
          :class="{ selected: selectedRouteId === route.id && routeDrawerOpen }"
          role="button"
          tabindex="0"
          :aria-label="`Open route ${route.name}`"
          @click="selectRoute(route)"
          @keydown.enter.self.prevent="selectRoute(route)"
          @keydown.space.self.prevent="selectRoute(route)"
        >
          <div class="list-main">
            <div class="list-title-row"><span class="list-title">{{ route.name }}</span><span class="badge">ROUTE</span></div>
            <p class="list-summary">{{ route.description.trim() || 'No description' }}</p>
          </div>
          <span class="route-count">{{ route.hops.length }} {{ route.hops.length === 1 ? 'hop' : 'hops' }}</span>
        </div>
      </div>
    </section>

    <div v-if="recordDrawerOpen" class="drawer-overlay" @click.self="closeRecordDrawer">
      <section class="drawer" role="dialog" aria-modal="true" :aria-label="editingRecord ? 'Edit record' : 'New record'">
        <div class="drawer-heading">
          <div><span class="eyebrow">{{ editingRecord ? 'Edit record' : 'New record' }}</span><h2>{{ recordForm.name || 'Untitled record' }}</h2></div>
          <button class="secondary close-button" type="button" :disabled="saving || detailLoading" @click="closeRecordDrawer">Close</button>
        </div>
        <div class="drawer-body">
          <p v-if="detailLoading" class="empty">Loading details…</p>
          <form v-else class="form" @submit.prevent="saveRecord">
            <fieldset :disabled="saving">
              <legend>Record type</legend>
              <div class="segmented">
                <button v-for="category in (['NOTE', 'HOST', 'DATABASE'] as RecordCategory[])" :key="category" type="button" :class="{ active: recordForm.category === category }" @click="changeCategory(category)">{{ category }}</button>
              </div>
            </fieldset>

            <div class="form-grid" :class="{ single: recordForm.category === 'NOTE' }">
              <label><span>{{ recordForm.category === 'NOTE' ? 'Title' : 'Name' }}</span><input v-model="recordForm.name" required autocomplete="off" :placeholder="recordForm.category === 'NOTE' ? 'Deployment notes' : 'Production database'" /></label>
              <label v-if="recordForm.category !== 'NOTE'"><span>Alias</span><input v-model="recordForm.alias" required autocomplete="off" pattern="[a-zA-Z0-9][a-zA-Z0-9._-]*" placeholder="prod-db" /></label>
            </div>
            <label><span>{{ recordForm.category === 'NOTE' ? 'Content' : 'Notes' }} <small>Optional</small></span><textarea v-model="recordForm.notes" :class="{ 'note-content': recordForm.category === 'NOTE' }" :rows="recordForm.category === 'NOTE' ? 10 : 3" :placeholder="recordForm.category === 'NOTE' ? 'Write your note here…' : 'Environment, owner, or other context'"></textarea></label>
            <label><span>Tags <small>Comma separated</small></span><input v-model="recordForm.tags" autocomplete="off" placeholder="production, Doris, data-platform" /></label>
            <label class="checkbox"><input v-model="recordForm.favorite" type="checkbox" /><span>Mark as favorite</span></label>

            <div v-if="recordForm.category !== 'NOTE'" class="divider"><span>{{ recordForm.category === 'HOST' ? 'SSH connection' : 'Database connection' }}</span></div>

            <template v-if="recordForm.category === 'HOST'">
              <div class="form-grid compact">
                <label class="grow"><span>Host</span><input v-model="recordForm.host" required placeholder="10.0.0.20" /></label>
                <label class="port"><span>Port</span><input v-model.number="recordForm.port" required type="number" min="1" max="65535" /></label>
              </div>
              <div class="form-grid">
                <label><span>Authentication</span><select :value="recordForm.authType" @change="changeAuthType(($event.target as HTMLSelectElement).value as AuthType)"><option value="NONE">None</option><option value="PASSWORD">Password</option><option value="SSH_KEY">SSH key</option></select></label>
                <label><span>Route <small>Optional</small></span><select v-model="recordForm.routeId"><option value="">Direct connection</option><option v-for="route in routes" :key="route.id" :value="route.id">{{ route.name }}</option></select></label>
              </div>
              <p v-if="routes.length === 0" class="hint route-empty">No routes yet. <button type="button" class="inline-link" @click="switchTab('routes')">Create one in Routes</button>.</p>
            </template>

            <template v-if="recordForm.category === 'DATABASE'">
              <div class="form-grid">
                <label><span>Database type</span><select :value="recordForm.dbType" @change="changeDatabaseType(($event.target as HTMLSelectElement).value as DBType)"><option value="MYSQL">MySQL</option><option value="POSTGRESQL">PostgreSQL</option><option value="DORIS">Doris</option></select></label>
                <label><span>Database name <small>Optional</small></span><input v-model="recordForm.databaseName" placeholder="app_production" /></label>
              </div>
              <div class="form-grid compact">
                <label class="grow"><span>Host</span><input v-model="recordForm.host" required placeholder="db.internal" /></label>
                <label class="port"><span>Port</span><input v-model.number="recordForm.port" required type="number" min="1" max="65535" /></label>
              </div>
            </template>

            <div v-if="recordForm.category !== 'NOTE' && (recordForm.category !== 'HOST' || recordForm.authType !== 'NONE')" class="form-grid">
              <label><span>Username</span><input v-model="recordForm.username" required autocomplete="username" placeholder="username" /></label>
              <label v-if="recordForm.category !== 'HOST' || recordForm.authType === 'PASSWORD'"><span>Password <small v-if="editingRecord && hasExistingPassword">Leave blank to keep current</small></span><input v-model="recordForm.password" :required="!editingRecord || !hasExistingPassword" type="password" autocomplete="new-password" placeholder="••••••••" /></label>
              <label v-else><span>Private key path</span><input v-model="recordForm.keyPath" required placeholder="~/.ssh/id_ed25519" /></label>
            </div>

            <div class="actions">
              <button v-if="editingRecord" class="danger-outline" type="button" :disabled="saving" @click="removeRecord">Delete record</button>
              <span v-else></span>
              <button class="primary" type="submit" :disabled="saving">{{ saving ? 'Saving…' : 'Save record' }}</button>
            </div>
          </form>
        </div>
      </section>
    </div>

    <div v-if="routeDrawerOpen" class="drawer-overlay" @click.self="closeRouteDrawer">
      <section class="drawer" role="dialog" aria-modal="true" :aria-label="editingRoute ? 'Edit route' : 'New route'">
        <div class="drawer-heading">
          <div><span class="eyebrow">{{ editingRoute ? 'Edit route' : 'New route' }}</span><h2>{{ routeForm.name || 'Untitled route' }}</h2></div>
          <button class="secondary close-button" type="button" :disabled="saving" @click="closeRouteDrawer">Close</button>
        </div>
        <div class="drawer-body">
          <form class="form" @submit.prevent="saveRoute">
            <div class="form-grid">
              <label><span>Name</span><input v-model="routeForm.name" required placeholder="Production route" /></label>
              <label><span>Description <small>Optional</small></span><input v-model="routeForm.description" placeholder="Access through production bastions" /></label>
            </div>
            <div class="divider"><span>Hops in connection order</span></div>
            <label><span>Add HOST</span><select :disabled="availableHopHosts.length === 0" value="" @change="addHop"><option value="">{{ availableHopHosts.length ? 'Choose a direct HOST…' : 'No eligible HOST available' }}</option><option v-for="host in availableHopHosts" :key="host.id" :value="host.id">{{ host.name }} ({{ host.alias }})</option></select></label>
            <p class="hint">Only HOST records without their own route can be used as hops.</p>
            <ol v-if="routeForm.hostIds.length" class="hops">
              <li v-for="(hostId, index) in routeForm.hostIds" :key="hostId">
                <span class="hop-number">{{ index + 1 }}</span><span class="hop-name">{{ hostName(hostId) }}</span>
                <div class="hop-actions"><button type="button" :disabled="index === 0" :aria-label="`Move ${hostName(hostId)} up`" @click="moveHop(index, -1)">↑</button><button type="button" :disabled="index === routeForm.hostIds.length - 1" :aria-label="`Move ${hostName(hostId)} down`" @click="moveHop(index, 1)">↓</button><button type="button" :aria-label="`Remove ${hostName(hostId)}`" @click="removeHop(index)">×</button></div>
              </li>
            </ol>
            <p v-else class="empty bordered">Add at least one HOST hop.</p>
            <div class="actions">
              <button v-if="editingRoute" class="danger-outline" type="button" :disabled="saving" @click="removeRoute">Delete route</button><span v-else></span>
              <button class="primary" type="submit" :disabled="saving || routeForm.hostIds.length === 0">{{ saving ? 'Saving…' : 'Save route' }}</button>
            </div>
          </form>
        </div>
      </section>
    </div>
  </main>
</template>