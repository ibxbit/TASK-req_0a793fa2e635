import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router/index.js'
import { startQueueProcessor } from './offline/queue.js'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')

startQueueProcessor()
