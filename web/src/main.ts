import { createApp } from 'vue'
// Element Plus 组件样式由 unplugin-vue-components 按需注入；
// ElMessage / ElMessageBox / v-loading 为显式调用或指令，需在此手动引入样式
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'
import 'element-plus/es/components/loading/style/css'
import App from './App.vue'
import router from './router'
import './styles/main.css'

createApp(App).use(router).mount('#app')
