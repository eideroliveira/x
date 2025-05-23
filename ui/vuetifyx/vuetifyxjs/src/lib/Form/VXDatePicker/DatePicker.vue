<template>
  <div class="vx-datepicker-wrap">
    <vx-label
      v-if="label"
      class="mb-2"
      :tooltip="tips"
      :label-for="name"
      :required-symbol="required"
      >{{ label }}</vx-label
    >

    <vx-field
      v-model="inputValue"
      :placeholder="placeholder"
      :focused="isFocus"
      ref="inputRef"
      @blur="onInputBlur"
      @mouseover="isHovering = true"
      @mouseout="isHovering = false"
      @keydown.enter="onInputBlur(inputValue, true)"
      v-bind="filteredAttrs"
      :style="minWidth"
    >
      <!-- calendar icon -->
      <template v-if="!hideAppendInner" #append-inner
        ><v-icon
          :icon="showClearIcon ? 'mdi-close-circle' : 'mdi-calendar-range-outline'"
          @click.stop="onClickAppendInner"
      /></template>

      <!-- drop down -->
      <v-overlay
        v-model="showMenu"
        open-on-click
        :close-on-content-click="false"
        activator="parent"
        :scrim="false"
        :open-delay="0"
        :close-delay="0"
        min-width="292"
        location-strategy="connected"
      >
        <date-picker-base
          class="elevation-5 d-inline-block bg-background rounded-lg overflow-hidden"
          :model-value="datePickerValue"
          :type="type"
          @update:modelValue="emitDatePickerValue"
          :format-str="formatStr"
          :datePickerProps="datePickerProps"
        />
      </v-overlay>

      <input readonly class="input-cover" :value="inputValue" :placeholder="placeholder" />
    </vx-field>
  </div>
</template>

<script setup lang="ts">
import { defineEmits, computed, PropType, watchEffect, watch, ref } from 'vue'
import { useFilteredAttrs } from '@/lib/composables/useFilteredAttrs'
import datePickerBase from './DatePickerBase.vue'
import { useDatePicker, datePickerType } from '@/lib/composables/useDatePicker'
import dayjs from 'dayjs'
const { filteredAttrs } = useFilteredAttrs()

const props = defineProps({
  modelValue: [String, Number, Date],
  tips: String,
  name: String,
  required: Boolean,
  label: String,
  placeholder: String,
  type: {
    type: String as PropType<datePickerType>,
    default: 'datepicker'
  },
  clearable: Boolean,
  format: {
    type: String,
    default: ''
  },
  hideAppendInner: Boolean,
  datePickerProps: Object
})
const inputValue = ref()
const inputRef = ref()
const datePickerValue = ref()
const isHovering = ref(false)
const isFocus = computed(() => showMenu.value)
const emit = defineEmits(['update:modelValue', 'blur'])
const { showMenu, formatStr, emitDatePickerValue } = useDatePicker(props, emit)
const timeStr = ref('00:00:00')

const showClearIcon = computed(
  () => (isHovering.value || showMenu.value) && inputValue.value && props.clearable
)
const minWidth = computed(() => ({
  minWidth: props.type === 'datepicker' ? '140px' : '190px'
}))

watch(
  () => showMenu.value,
  (oldVal, newVal) => {
    // this state is when finished select data and dropdown is closed
    if (!oldVal && newVal) {
      emitDatePickerValue(datePickerValue.value, { extraEmitEvents: ['blur'] })
    }
  }
)

// Add watch for time selection with no date
watch(
  () => props.modelValue,
  (value) => {
    if (value && props.type === 'datetimepicker') {
      // When value is changed and it has a time component but no date
      const val = dayjs(value)
      if (val.hour() !== 0 || val.minute() !== 0 || val.second() !== 0) {
        // Has time component
        if (val.year() === 1970 && val.month() === 0 && val.date() === 1) {
          // Likely only time was set (Unix epoch date)
          const currentDate = new Date()
          const newDate = dayjs(currentDate)
            .hour(val.hour())
            .minute(val.minute())
            .second(val.second())

          emitDatePickerValue(newDate.valueOf())
        }
      }
    }
  }
)

watchEffect(() => {
  convertValueForInputAndDatePicker({ value: props.modelValue, shouldEmit: true })
})

// Add this watch effect to handle time-only selection
watch(
  () => datePickerValue.value,
  (value) => {
    if (!value && props.type === 'datetimepicker') {
      // If only time is provided but no date, use current date
      const currentDate = new Date()
      datePickerValue.value = currentDate.valueOf()
      // Update inputValue with formatted date+time
      if (formatStr) {
        inputValue.value = dayjs(currentDate).format(formatStr)
      }
    }
  }
)

function onInputBlur(obj: FocusEvent | string, closeMenu: boolean = false) {
  // fix blur event is more quick than modelValue change event
  // dropdown visible condition is conflict with this event
  if (closeMenu) {
    showMenu.value = false
    inputRef.value.blur()
  }

  let value

  if (obj instanceof FocusEvent) {
    const target = obj.target as HTMLInputElement
    value = target.value
  } else {
    value = obj
  }

  if (props.datePickerProps) {
    const maxTimestamp = props.datePickerProps.max ? dayjs(props.datePickerProps.max).valueOf() : 0
    const minTimestamp = props.datePickerProps.min ? dayjs(props.datePickerProps.min).valueOf() : 0
    const current = dayjs(value).valueOf()
    if (current > maxTimestamp) {
      value = maxTimestamp
    } else if (current < minTimestamp) {
      value = minTimestamp
    }
  }

  // the first time select date will trigger blur event
  if (!value) return

  convertValueForInputAndDatePicker({ value, shouldEmit: true, extraEmitEvents: ['blur'] })
}

function convertValueForInputAndDatePicker({
  value,
  shouldEmit,
  extraEmitEvents
}: {
  value: string | number | undefined | Date
  shouldEmit?: boolean
  extraEmitEvents?: string[]
}) {
  //case: no init value
  if (!value) {
    inputValue.value = ''
    datePickerValue.value = ''
  } else {
    inputValue.value = formatStr ? dayjs(value).format(formatStr) : value
    datePickerValue.value = value ? dayjs(value).valueOf() : ''
  }
  shouldEmit && emitDatePickerValue(datePickerValue.value, { extraEmitEvents })
}

function onClickAppendInner() {
  if (showClearIcon.value) {
    emitDatePickerValue('')
    showMenu.value = false
  } else {
    showMenu.value = true
  }
}
</script>

<style lang="scss" scoped>
.v-menu {
  &:deep(.v-overlay__content) {
    border-radius: 8px !important;
  }
}

.vx-datepicker-wrap {
  &:deep(.v-input) {
    .input-cover {
      position: absolute;
      width: 100%;
      height: 100%;
      z-index: 1;
      pointer-events: none;
    }

    & input:not(.input-cover) {
      display: none;
    }

    .v-field {
      cursor: pointer;
    }
  }
}
</style>
