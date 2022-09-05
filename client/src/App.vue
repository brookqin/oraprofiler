<template>
  <a-layout has-sider>
    <a-layout-sider id="tss" width="40%" :style="{ height: '100vh', position: 'fixed', left: 0, top: 0, bottom: 0 }">
        <a-table
          :columns="columns"
          :data-source="tableData"
          :pagination="false"
          :row-selection="rowSelection"
          :custom-row="customRow"
          rowKey="id"
          sticky
          size="small" >
          <template #title>
            <a-row type="flex" :gutter="10" style="padding: 10px;">
              <a-col flex="auto">
                <div ref="select">
                <a-select
                  v-model:value="selectedSession"
                  :dropdownMatchSelectWidth="false"
                  :options="sessions.map(s => ({ label: `${s.sid} | ${s.osuser} | ${s.program} | ${s.service} | ${s.terminal} | ${s.schema}`, value: `${s.sid}|${s.serial}|${s.service}` }))"
                  :style="{ width: selectWidth }"
                >
                </a-select>
                </div>
              </a-col>
              <a-col flex="100px">
                <a-button danger :disabled="wsOpened == false" @click="onTraceClick" :loading="isTracing == 1">{{ traceStatusText }}</a-button>
              </a-col>
              <a-col flex="100px">
                <a-button @click="() => tableData = []">CLEAR</a-button>
              </a-col>
            </a-row>
          </template>
        </a-table>
        <a-back-top :target="backToTop" :style="{ right: 'auto', left: '50px' }" />
    </a-layout-sider>
    <a-layout :style="{ marginLeft: '40%' }">
      <a-layout-content style="background-color: gray; padding: 20px; overflow-y: auto;">
          <preview-code :code="selectedTrace?.sql" />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { notification, Button } from 'ant-design-vue'
import 'ant-design-vue/lib/notification/style/index.css'
import { h, ref, reactive, computed, onMounted } from 'vue'
import PreviewCode from './components/PreviewCode.vue'

interface Session {
  sid: number,
  serial: number,
  osuser: string,
  terminal: string,
  program: string,
  service: string,
  schema: string
}

interface Trace {
  id: string
  sql: string
  tim: number
}

const wsOpened = ref(false)
const sessions = ref<Session[]>([])
const selectedSession = ref<string | null>('')
const selectedTrace = ref<Trace | null>(null)

const state = reactive({
  selectedRowKeys: [] as string[]
})

const selectRow = (record: Trace) => {
  state.selectedRowKeys = [record.id]
}
const rowSelection = computed(() => {
  return {
    type: 'radio',
    selectedRowKeys: state.selectedRowKeys,
    onChange: (selectedRowKeys: []) => {
      console.log(selectedRowKeys)
      state.selectedRowKeys = selectedRowKeys
    }
  }
})
const customRow = (record: Trace) => {
  return {
    onClick: () => {
      selectedTrace.value = record
      selectRow(record)
    }
  }
}

const columns = [
  {
    dataIndex: 'id',
    title: '#ID',
    width: 160
  },
  {
    dataIndex: 'sql',
    title: 'SQL',
    ellipsis: true
  },
  {
    dataIndex: 'tim',
    title: 'TIME',
    width: 100
  }
]
const tableData = ref<Trace[]>([])

const select = ref<HTMLDivElement | null>(null)
const selectWidth = ref<string>('100%')
onMounted(() => {
  if (select.value) {
    //select.value.style = `width: ${select.value.clientWidth}px`
    selectWidth.value = select.value.clientWidth - 10 + 'px'
  }
})

const isTracing = ref(0)
const traceStatusText = ref('Trace')

const ws = new WebSocket(`ws://${location.host}/ws`)
ws.onopen = () => {
  console.log('connected')
  wsOpened.value = true;
}
ws.onclose = () => {
  console.log('disconnected')
  wsOpened.value = false;
  const _key = `key${Date.now()}`
  notification['warning']({
    message: 'Warning',
    description: 'Disconnected from server',
    duration: 0,
    btn: () =>
          h(
            Button,
            {
              type: 'primary',
              size: 'small',
              onClick: () => {
                notification.close(_key)
                window.location.reload()
              }
            },
            { default: () => 'RELOAD' },
          ),
    key: _key
  })
}
ws.onerror = (err) => {
  console.log(err)
}

ws.onmessage = (event) => {
  const data = JSON.parse(event.data)
  if (data.command === 'ping') {
    ws.send('pong')
  }
  else if (data.command === 'sessions') {
    sessions.value = data.data
  }
  else if (data.command === 'trace') {
    if (data.data === 'started') {
      isTracing.value = 2
      traceStatusText.value = 'STOP'
    }
    else if (data.data === 'stopped') {
      isTracing.value = 0
      traceStatusText.value = 'TRACE'
    } else if (typeof data.data === 'object') {
      tableData.value.push(data.data)
    }
  }
  else if (data.command === 'error') {
    const _key = `key${Date.now()}`
    notification['error']({
      message: 'Error',
      description: data.data,
      duration: 0,
      btn: () =>
            h(
              Button,
              {
                type: 'primary',
                size: 'small',
                onClick: () => {
                  notification.close(_key)
                }
              },
              { default: () => 'CLOSE' },
            ),
      key: _key
    })
  }
}

const onTraceClick = () => {
  if (isTracing.value === 0) {
    if (!selectedSession.value) return
    isTracing.value = 1
    traceStatusText.value = ''
    ws.send(`trace|${selectedSession.value}`)
  } else if (isTracing.value === 2) {
    isTracing.value = 1
    traceStatusText.value = ''
    ws.send('untrace')
  }
}

const backToTop = () => {
  return document.querySelector('#tss .ant-layout-sider-children')
}
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  color: #e0e0e0;
  margin: 0;
  padding: 0;
  height: 100%;
}

.ant-layout {
  height: 100%;
}

#tss .ant-layout-sider-children {
  overflow: auto;
}

</style>
