<template>
  <el-container class="main-layout">
    <el-aside :width="isCollapsed ? '64px' : '220px'" class="sidebar">
      <div class="sidebar-header">
        <h1 v-show="!isCollapsed" class="app-title">zPush</h1>
        <span v-show="isCollapsed" class="app-title-collapsed">zP</span>
      </div>
      <el-menu
        :default-active="activeMenu"
        :collapse="isCollapsed"
        :collapse-transition="false"
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409EFF"
        router
        class="sidebar-menu"
      >
        <el-menu-item index="/dashboard">
          <el-icon><Odometer /></el-icon>
          <template #title>{{ $t('nav.dashboard') }}</template>
        </el-menu-item>
        <el-menu-item index="/sources">
          <el-icon><Connection /></el-icon>
          <template #title>{{ $t('nav.sources') }}</template>
        </el-menu-item>
        <el-menu-item index="/mappings">
          <el-icon><Switch /></el-icon>
          <template #title>{{ $t('nav.mappings') }}</template>
        </el-menu-item>
        <el-menu-item index="/rules">
          <el-icon><SetUp /></el-icon>
          <template #title>{{ $t('nav.rules') }}</template>
        </el-menu-item>
        <el-menu-item index="/templates">
          <el-icon><Document /></el-icon>
          <template #title>{{ $t('nav.templates') }}</template>
        </el-menu-item>
        <el-menu-item index="/destinations">
          <el-icon><Promotion /></el-icon>
          <template #title>{{ $t('nav.destinations') }}</template>
        </el-menu-item>
        <el-sub-menu index="logs-group">
          <template #title>
            <el-icon><Notebook /></el-icon>
            <span>{{ $t('nav.logs') }}</span>
          </template>
          <el-menu-item index="/events">
            <el-icon><Bell /></el-icon>
            <template #title>{{ $t('nav.eventLogs') }}</template>
          </el-menu-item>
          <el-menu-item index="/deliveries">
            <el-icon><Van /></el-icon>
            <template #title>{{ $t('nav.deliveryLogs') }}</template>
          </el-menu-item>
          <el-menu-item index="/audit">
            <el-icon><List /></el-icon>
            <template #title>{{ $t('nav.auditLogs') }}</template>
          </el-menu-item>
        </el-sub-menu>
        <el-menu-item index="/playground">
          <el-icon><MagicStick /></el-icon>
          <template #title>{{ $t('nav.playground') }}</template>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <template #title>{{ $t('nav.settings') }}</template>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container class="main-container">
      <el-header class="top-bar">
        <div class="top-bar-left">
          <el-icon class="collapse-btn" @click="isCollapsed = !isCollapsed">
            <Fold v-if="!isCollapsed" />
            <Expand v-else />
          </el-icon>
          <span class="page-title">{{ currentTitle }}</span>
        </div>
        <div class="top-bar-right">
          <el-dropdown @command="handleLocaleChange" class="locale-dropdown">
            <span class="locale-switch">
              <span>{{ locale === 'zh' ? '中文' : 'EN' }}</span>
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="zh" :class="{ 'is-active': locale === 'zh' }">中文</el-dropdown-item>
                <el-dropdown-item command="en" :class="{ 'is-active': locale === 'en' }">English</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-icon><User /></el-icon>
              <span class="username">Admin</span>
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  {{ $t('nav.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { setLocale, type Locales } from '../locales'

const route = useRoute()
const authStore = useAuthStore()
const { t, locale } = useI18n()
const isCollapsed = ref(false)

const activeMenu = computed(() => route.path)
const currentTitle = computed(() => {
  const titleKey = route.meta?.titleKey as string
  return titleKey ? t(titleKey) : 'zPush'
})

function handleCommand(command: string) {
  if (command === 'logout') {
    authStore.logout()
  }
}

function handleLocaleChange(lang: string) {
  setLocale(lang as Locales)
}
</script>

<style lang="scss" scoped>
.main-layout {
  height: 100vh;
}

.sidebar {
  background-color: #304156;
  overflow: hidden;
  transition: width 0.3s;
  box-shadow: 2px 0 6px rgba(0, 21, 41, 0.35);
}

.sidebar-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #263445;
}

.app-title {
  color: #fff;
  font-size: 20px;
  font-weight: 700;
  margin: 0;
  letter-spacing: 2px;
}

.app-title-collapsed {
  color: #fff;
  font-size: 18px;
  font-weight: 700;
}

.sidebar-menu {
  border-right: none;
  height: calc(100vh - 60px);
  overflow-y: auto;

  &::-webkit-scrollbar {
    width: 0;
  }
}

.main-container {
  background-color: #f0f2f5;
}

.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: #fff;
  box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08);
  padding: 0 20px;
  height: 60px;
  z-index: 1;
}

.top-bar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.collapse-btn {
  font-size: 20px;
  cursor: pointer;
  color: #606266;
  transition: color 0.2s;

  &:hover {
    color: #409EFF;
  }
}

.page-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.top-bar-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.locale-dropdown {
  cursor: pointer;
}

.locale-switch {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  color: #606266;
  font-size: 14px;

  &:hover {
    color: #409EFF;
  }
}

.user-info {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  color: #606266;
  font-size: 14px;

  &:hover {
    color: #409EFF;
  }
}

.username {
  font-weight: 500;
}

.main-content {
  padding: 20px;
  overflow-y: auto;
  background-color: #f0f2f5;
}
</style>
