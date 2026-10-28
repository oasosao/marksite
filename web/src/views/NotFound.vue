<script setup lang="ts">
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<template>
  <div class="error-wrap">
    <div class="error-card" style="text-align: center">
      <h1 class="error-title">404</h1>
      <p class="error-sub">页面不存在</p>
      <el-button type="primary" @click="router.push('/dashboard')">返回仪表盘</el-button>
    </div>
  </div>
</template>
