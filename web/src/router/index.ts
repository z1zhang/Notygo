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
          meta: { title: 'Dashboard', icon: 'Odometer' },
        },
        {
          path: 'sources',
          name: 'Sources',
          component: () => import('../views/Sources.vue'),
          meta: { title: 'Sources', icon: 'Connection' },
        },
        {
          path: 'mappings',
          name: 'Mappings',
          component: () => import('../views/Mappings.vue'),
          meta: { title: 'Mappings', icon: 'Switch' },
        },
        {
          path: 'rules',
          name: 'Rules',
          component: () => import('../views/Rules.vue'),
          meta: { title: 'Rules', icon: 'SetUp' },
        },
        {
          path: 'templates',
          name: 'Templates',
          component: () => import('../views/Templates.vue'),
          meta: { title: 'Templates', icon: 'Document' },
        },
        {
          path: 'destinations',
          name: 'Destinations',
          component: () => import('../views/Destinations.vue'),
          meta: { title: 'Destinations', icon: 'Promotion' },
        },
        {
          path: 'events',
          name: 'Events',
          component: () => import('../views/Events.vue'),
          meta: { title: 'Events', icon: 'Bell' },
        },
        {
          path: 'deliveries',
          name: 'Deliveries',
          component: () => import('../views/Deliveries.vue'),
          meta: { title: 'Deliveries', icon: 'Van' },
        },
        {
          path: 'audit',
          name: 'Audit',
          component: () => import('../views/Audit.vue'),
          meta: { title: 'Audit Logs', icon: 'List' },
        },
        {
          path: 'settings',
          name: 'Settings',
          component: () => import('../views/Settings.vue'),
          meta: { title: 'Settings', icon: 'Setting' },
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
