import { createRouter, createWebHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('../views/Login.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      component: MainLayout,
      redirect: '/dashboard',
      meta: { requiresAuth: true },
      children: [
        {
          path: 'dashboard',
          name: 'Dashboard',
          component: () => import('../views/Dashboard.vue'),
          meta: { titleKey: 'nav.dashboard', icon: 'Odometer' },
        },
        {
          path: 'sources',
          name: 'Sources',
          component: () => import('../views/Sources.vue'),
          meta: { titleKey: 'nav.sources', icon: 'Connection' },
        },
        {
          path: 'mappings',
          name: 'Mappings',
          component: () => import('../views/Mappings.vue'),
          meta: { titleKey: 'nav.mappings', icon: 'Switch' },
        },
        {
          path: 'rules',
          name: 'Rules',
          component: () => import('../views/Rules.vue'),
          meta: { titleKey: 'nav.rules', icon: 'SetUp' },
        },
        {
          path: 'templates',
          name: 'Templates',
          component: () => import('../views/Templates.vue'),
          meta: { titleKey: 'nav.templates', icon: 'Document' },
        },
        {
          path: 'destinations',
          name: 'Destinations',
          component: () => import('../views/Destinations.vue'),
          meta: { titleKey: 'nav.destinations', icon: 'Promotion' },
        },
        {
          path: 'events',
          name: 'Events',
          component: () => import('../views/Events.vue'),
          meta: { titleKey: 'nav.eventLogs', icon: 'Bell' },
        },
        {
          path: 'deliveries',
          name: 'Deliveries',
          component: () => import('../views/Deliveries.vue'),
          meta: { titleKey: 'nav.deliveryLogs', icon: 'Van' },
        },
        {
          path: 'audit',
          name: 'Audit',
          component: () => import('../views/Audit.vue'),
          meta: { titleKey: 'nav.auditLogs', icon: 'List' },
        },
        {
          path: 'playground',
          name: 'Playground',
          component: () => import('../views/Playground.vue'),
          meta: { titleKey: 'nav.playground', icon: 'MagicStick' },
        },
        {
          path: 'settings',
          name: 'Settings',
          component: () => import('../views/Settings.vue'),
          meta: { titleKey: 'nav.settings', icon: 'Setting' },
        },
      ],
    },
  ],
})

// Navigation guard
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth !== false && !token) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
  } else if (to.name === 'Login' && token) {
    next({ name: 'Dashboard' })
  } else {
    next()
  }
})

export default router
