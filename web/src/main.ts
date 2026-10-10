import { createApp } from 'vue'
import App from './App.vue'
import { connectWS, refreshJobs } from './api'
import './style.css'

createApp(App).mount('#app')
connectWS()
refreshJobs()
