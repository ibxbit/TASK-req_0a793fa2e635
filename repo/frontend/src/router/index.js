import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuth, checkSession } from '../composables/useAuth.js'
import { ROLES } from '../composables/useRbac.js'

import PublicSearchPage from '../pages/SearchPage.vue'
import ReviewSubmitPage from '../pages/ReviewSubmitPage.vue'
import ComplaintSubmitPage from '../pages/ComplaintSubmitPage.vue'
import MemberComplaintsPage from '../pages/MemberComplaintsPage.vue'
import ConsoleLayout from '../console/ConsoleLayout.vue'
import DashboardPage     from '../console/pages/DashboardPage.vue'
import ContentPage       from '../console/pages/ContentPage.vue'
import PricingPage       from '../console/pages/PricingPage.vue'
import PricingMgmtPage   from '../console/pages/PricingMgmtPage.vue'
import ComplaintsPage    from '../console/pages/ComplaintsPage.vue'
import CrawlPage         from '../console/pages/CrawlPage.vue'
import ApprovalsPage     from '../console/pages/ApprovalsPage.vue'
import RevisionsPage     from '../console/pages/RevisionsPage.vue'
import AuditPage         from '../console/pages/AuditPage.vue'
import SettingsPage      from '../console/pages/SettingsPage.vue'
import MonitoringPage    from '../console/pages/MonitoringPage.vue'
import ContentPackPage   from '../console/pages/ContentPackPage.vue'

// ALL_ROLES is used to gate the console itself — any authenticated staff
// role may land on /console (they get a dashboard). Individual pages then
// gate themselves via `meta.roles` with tighter scopes.
//
// The `member` role is intentionally excluded from the console — members
// get the public search page, the offline download flow, and their own
// review/complaint flows, nothing else.
const STAFF_ROLES = [ROLES.ADMIN, ROLES.EDITOR, ROLES.REVIEWER, ROLES.MKT, ROLES.CRAWLER]
const ALL_AUTHED  = [...STAFF_ROLES, ROLES.MEMBER]

const routes = [
  { path: '/',  name: 'search', component: PublicSearchPage },
  {
    path: '/download',
    name: 'download',
    component: ContentPackPage,
    meta: { requiresAuth: true, roles: ALL_AUTHED },
  },
  {
    path: '/reviews/new',
    name: 'reviews.new',
    component: ReviewSubmitPage,
    meta: { requiresAuth: true, roles: ALL_AUTHED },
  },
  {
    path: '/complaints/new',
    name: 'complaints.new',
    component: ComplaintSubmitPage,
    meta: { requiresAuth: true, roles: ALL_AUTHED },
  },
  {
    path: '/my-complaints',
    name: 'my-complaints',
    component: MemberComplaintsPage,
    meta: { requiresAuth: true, roles: ALL_AUTHED },
  },
  {
    path: '/console',
    component: ConsoleLayout,
    meta: { requiresAuth: true, roles: STAFF_ROLES },
    children: [
      { path: '',             redirect: { name: 'console.dashboard' } },
      { path: 'dashboard',    name: 'console.dashboard',    component: DashboardPage,    meta: { roles: STAFF_ROLES } },
      { path: 'content',      name: 'console.content',      component: ContentPage,      meta: { roles: [ROLES.ADMIN, ROLES.EDITOR] } },
      { path: 'pricing',      name: 'console.pricing',      component: PricingPage,      meta: { roles: [ROLES.ADMIN, ROLES.MKT] } },
      { path: 'pricing-mgmt', name: 'console.pricing_mgmt', component: PricingMgmtPage,  meta: { roles: [ROLES.ADMIN, ROLES.MKT] } },
      { path: 'complaints',   name: 'console.complaints',   component: ComplaintsPage,   meta: { roles: [ROLES.ADMIN, ROLES.REVIEWER] } },
      { path: 'crawl',        name: 'console.crawl',        component: CrawlPage,        meta: { roles: [ROLES.ADMIN, ROLES.CRAWLER] } },
      { path: 'approvals',    name: 'console.approvals',    component: ApprovalsPage,    meta: { roles: [ROLES.ADMIN] } },
      { path: 'revisions',    name: 'console.revisions',    component: RevisionsPage,    meta: { roles: [ROLES.ADMIN] } },
      { path: 'audit',        name: 'console.audit',        component: AuditPage,        meta: { roles: [ROLES.ADMIN] } },
      { path: 'monitoring',   name: 'console.monitoring',   component: MonitoringPage,   meta: { roles: [ROLES.ADMIN] } },
      { path: 'settings',     name: 'console.settings',     component: SettingsPage,     meta: { roles: [ROLES.ADMIN] } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const { ready, isAuthenticated, user } = useAuth()
  if (!ready.value) await checkSession()

  if (to.meta.requiresAuth && !isAuthenticated.value) {
    return { name: 'search', query: { next: to.fullPath } }
  }
  if (isAuthenticated.value && to.meta.roles) {
    const role = user.value?.role
    if (!to.meta.roles.includes(role)) {
      // Members bounce to the public search page — they have no console.
      if (role === ROLES.MEMBER) {
        return { name: 'search' }
      }
      return { name: 'console.dashboard' }
    }
  }
})

export default router
