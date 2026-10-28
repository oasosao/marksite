<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { configApi } from '../api'
import type { Asset, SiteConfig } from '../types'
import { emit } from '../utils/bus'
import AssetPickerDialog from '../components/AssetPickerDialog.vue'

const form = ref<SiteConfig>({
  id: 0,
  site_name: '',
  site_desc: '',
  site_url: '',
  logo: '',
  favicon: '',
  copyright: '',
  page_size: 10,
  path_mode: 'fixed',
  theme: '',
  rss_enabled: 1,
  sitemap_enabled: 1,
  robots: '',
  custom_head: '',
  custom_footer: '',
  updated_at: '',
})
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const activeGroups = ref(['basic', 'appearance'])
const picker = ref<{ visible: boolean; target: 'logo' | 'favicon' }>({ visible: false, target: 'logo' })

async function load() {
  loading.value = true
  try {
    form.value = await configApi.getSite()
    loaded.value = true
  } finally {
    loading.value = false
  }
}

function openPicker(target: 'logo' | 'favicon') {
  picker.value = { visible: true, target }
}

function onPicked(asset: Asset | Asset[]) {
  const a = Array.isArray(asset) ? asset[0] : asset
  if (!a) return
  if (picker.value.target === 'logo') form.value.logo = a.path
  else form.value.favicon = a.path
}

async function save() {
  saving.value = true
  try {
    form.value = await configApi.updateSite(form.value)
    ElMessage.success('已保存，修改后需重新生成站点生效')
    emit('site-refresh')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="page-header">
      <div>
        <h2 class="page-title">网站生成设置</h2>
        <p class="page-subtitle">站点信息与静态生成参数</p>
      </div>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </div>

    <div v-if="loaded">
      <el-collapse v-model="activeGroups">
        <el-collapse-item title="基本信息" name="basic">
          <div class="card" style="margin-top: 8px">
            <el-form label-width="110px">
              <el-form-item label="站点名称" required>
                <el-input v-model="form.site_name" maxlength="100" show-word-limit />
              </el-form-item>
              <el-form-item label="站点描述">
                <el-input v-model="form.site_desc" type="textarea" :rows="2" maxlength="500" show-word-limit />
              </el-form-item>
              <el-form-item label="站点 URL" required>
                <el-input v-model="form.site_url" placeholder="https://example.com" />
                <div class="muted" style="width: 100%; margin-top: 4px">用于 RSS、Sitemap 中的绝对地址，不带末尾斜杠</div>
              </el-form-item>
              <el-form-item label="版权信息">
                <el-input v-model="form.copyright" placeholder="© 2026 Example" maxlength="500" />
              </el-form-item>
            </el-form>
          </div>
        </el-collapse-item>

        <el-collapse-item title="外观" name="appearance">
          <div class="card" style="margin-top: 8px">
            <el-form label-width="110px">
              <el-form-item label="当前主题">
                <span class="muted">
                  {{ form.theme }}（如需切换主题请前往
                  <router-link to="/themes">主题管理</router-link>）
                </span>
              </el-form-item>
              <el-form-item label="Logo">
                <div style="display: flex; align-items: center; gap: 8px; width: 100%">
                  <el-input v-model="form.logo" placeholder="资源 path（可选）" readonly />
                  <el-button @click="openPicker('logo')">选择</el-button>
                  <img v-if="form.logo" :src="`/media/${form.logo}`" alt="logo" style="height: 40px; border-radius: 6px" />
                </div>
              </el-form-item>
              <el-form-item label="Favicon">
                <div style="display: flex; align-items: center; gap: 8px; width: 100%">
                  <el-input v-model="form.favicon" placeholder="资源 path（可选）" readonly />
                  <el-button @click="openPicker('favicon')">选择</el-button>
                  <img v-if="form.favicon" :src="`/media/${form.favicon}`" alt="favicon" style="height: 32px" />
                </div>
              </el-form-item>
            </el-form>
          </div>
        </el-collapse-item>

        <el-collapse-item title="SEO 与 RSS" name="seo">
          <div class="card" style="margin-top: 8px">
            <el-form label-width="110px">
              <el-form-item label="RSS">
                <el-switch v-model="form.rss_enabled" :active-value="1" :inactive-value="0" active-text="启用 feed.xml" />
              </el-form-item>
              <el-form-item label="Sitemap">
                <el-switch v-model="form.sitemap_enabled" :active-value="1" :inactive-value="0" active-text="启用 sitemap.xml" />
              </el-form-item>
              <el-form-item label="robots.txt">
                <el-input v-model="form.robots" type="textarea" :rows="4" maxlength="5000" show-word-limit />
              </el-form-item>
              <el-form-item label="自定义 head">
                <el-input v-model="form.custom_head" type="textarea" :rows="4" maxlength="20000" placeholder="将插入到页面 <head> 内" />
              </el-form-item>
              <el-form-item label="自定义 footer">
                <el-input v-model="form.custom_footer" type="textarea" :rows="4" maxlength="20000" placeholder="将插入到页面底部" />
              </el-form-item>
            </el-form>
          </div>
        </el-collapse-item>

        <el-collapse-item title="内容与路径" name="content">
          <div class="card" style="margin-top: 8px">
            <el-form label-width="110px">
              <el-form-item label="每页数量">
                <el-input-number v-model="form.page_size" :min="1" :max="100" controls-position="right" />
                <span class="muted" style="margin-left: 10px">首页与归档分页大小</span>
              </el-form-item>
              <el-form-item label="URL 规则">
                <span class="muted" style="line-height: 1.7">
                  固定规则：文章与页面 /&#123;分类slug&#125;/&#123;slug&#125;.html；分类列表 /&#123;分类slug&#125;/index.html；根级与独立页面 /&#123;slug&#125;.html；标签页 /tags/&#123;slug&#125;.html
                </span>
              </el-form-item>
            </el-form>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <AssetPickerDialog v-model="picker.visible" @select="onPicked" />
  </div>
</template>
