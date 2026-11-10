<template>
  <Loading v-if="!auth.ready" full />
  <router-view v-else />
</template>

<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import Loading from '@/components/Loading.vue'

const auth = useAuthStore()
</script>
