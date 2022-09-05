<script setup lang="ts">
import { ref, toRefs, watch } from 'vue'
import { format, FormatOptions } from 'sql-formatter'
import Prism from 'prismjs'
import 'prismjs/components/prism-sql'
import 'prismjs/themes/prism.css'
import 'prismjs/themes/prism-okaidia.css'

const sqlFormatOptions: FormatOptions = {
  language: 'plsql',
  keywordCase: 'upper',
  tabWidth: 2,
  useTabs: false,
  indentStyle: 'standard',
  logicalOperatorNewline: 'before',
  tabulateAlias: false,
  commaPosition: 'before',
  expressionWidth: 50,
  linesBetweenQueries: 1,
  denseOperators: false,
  newlineBeforeSemicolon: false
}

const props = defineProps({
  code: {
    type: String,
    default: ''
  },
  type: {
    type: String,
    default: 'plsql'
  }
})

const code = toRefs(props).code
const type = props.type
const html = ref('')

watch(code, (first, second) => {
  html.value = Prism.highlight(format(code.value, sqlFormatOptions), Prism.languages[type], type)
})

</script>

<template>
  <pre><code :class="'language-'+ type" v-html="html"></code></pre>
</template>
