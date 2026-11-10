<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="visible" class="confirm-dialog" @click.self="onDismiss">
        <div class="content">
          <div v-if="title" class="title">{{ title }}</div>
          <div v-if="message" class="message">{{ message }}</div>
          <div class="footer">
            <button class="cancel" type="button" @click="onCancel">{{ cancelText }}</button>
            <button class="ok" type="button" @click="onOk">{{ okText }}</button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
// 确认弹窗组件，移植自 douyin 前端，用于需要二次确认的操作。
defineOptions({ name: 'ConfirmDialog' })

const props = withDefaults(
  defineProps<{
    visible?: boolean
    title?: string
    message?: string
    okText?: string
    cancelText?: string
  }>(),
  {
    visible: false,
    title: '',
    message: '',
    okText: '确定',
    cancelText: '取消',
  },
)

const emit = defineEmits<{
  'update:visible': [boolean]
  ok: []
  cancel: []
  dismiss: []
}>()

function close() {
  emit('update:visible', false)
}

function onOk() {
  close()
  emit('ok')
}

function onCancel() {
  close()
  emit('cancel')
}

function onDismiss() {
  close()
  emit('dismiss')
}
</script>

<style scoped>
.confirm-dialog {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.7);
}
.content {
  width: min(76%, 340px);
  background: #fff;
  border-radius: 10px;
  color: #202c28;
  text-align: center;
  overflow: hidden;
}
.title {
  padding: 22px 20px 4px;
  font-size: 16px;
  font-weight: 600;
}
.message {
  padding: 0 20px 20px;
  font-size: 13px;
  color: var(--second-text-color);
}
.footer {
  display: flex;
  border-top: 1px solid #eee;
}
.footer button {
  flex: 1;
  height: 46px;
  background: #fff;
  color: #202c28;
  font-size: 15px;
}
.footer .cancel {
  border-right: 1px solid #eee;
  color: var(--second-text-color);
}
.footer .ok {
  font-weight: 600;
  color: var(--primary-btn-color);
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
