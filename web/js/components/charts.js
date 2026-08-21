window.App = window.App || {};

Chart.defaults.font.family = "'Inter', system-ui, sans-serif"
Chart.defaults.color = '#64748b'

// Shared y-axis tick formatter: plain numbers below 1000, "k" above it — avoids Chart.js's default
// auto-scaling producing fractional-thousand ticks like "0.0001k" when every value is 0.
function formatAxisValue(v) {
  const n = Math.abs(Number(v))
  return n >= 1000 ? `${(n / 1000).toLocaleString('ru-RU', { maximumFractionDigits: 1 })}k` : n.toLocaleString('ru-RU')
}

App.DonutChart = {
  props: { labels: Array, values: Array, colors: Array },
  template: `<canvas ref="canvas"></canvas>`,
  data() {
    return { chart: null }
  },
  mounted() {
    // markRaw: Chart.js instances are deeply circular (canvas ↔ chart ↔ layout boxes) and mutate
    // their own internals directly. Left in Vue's data(), the instance gets wrapped in a reactive
    // Proxy, which breaks that direct mutation (Chart.js's layout code error setting properties
    // like `fullSize` on `undefined`) and can recurse into a stack overflow. markRaw keeps it plain.
    this.chart = Vue.markRaw(new Chart(this.$refs.canvas, {
      type: 'doughnut',
      data: this.buildData(),
      options: {
        responsive: true,
        maintainAspectRatio: false,
        cutout: '68%',
        plugins: {
          legend: { display: false },
          tooltip: { backgroundColor: '#0f172a', padding: 10, cornerRadius: 8 },
        },
      },
    }))
  },
  beforeUnmount() {
    this.chart?.destroy()
  },
  computed: {
    // labels/values/colors always change together on a data refresh — watching them individually
    // fires refresh() (and chart.update()) once per prop in the same tick, and the second call can
    // hit Chart.js mid-redraw from the first and throw, leaving the canvas stuck on stale data. One
    // watcher over a combined signature collapses that into a single update.
    signature() {
      return JSON.stringify([this.labels, this.values, this.colors])
    },
  },
  watch: {
    signature() {
      this.refresh()
    },
  },
  methods: {
    buildData() {
      return {
        labels: this.labels,
        datasets: [{ data: this.values, backgroundColor: this.colors, borderColor: '#ffffff', borderWidth: 2, hoverOffset: 4 }],
      }
    },
    refresh() {
      if (!this.chart) return
      this.chart.data = this.buildData()
      this.chart.update()
    },
  },
}

App.BarChart = {
  props: { labels: Array, income: Array, expense: Array },
  template: `<canvas ref="canvas"></canvas>`,
  data() {
    return { chart: null }
  },
  mounted() {
    // markRaw: Chart.js instances are deeply circular (canvas ↔ chart ↔ layout boxes) and mutate
    // their own internals directly. Left in Vue's data(), the instance gets wrapped in a reactive
    // Proxy, which breaks that direct mutation (Chart.js's layout code error setting properties
    // like `fullSize` on `undefined`) and can recurse into a stack overflow. markRaw keeps it plain.
    this.chart = Vue.markRaw(new Chart(this.$refs.canvas, {
      type: 'bar',
      data: this.buildData(),
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: '#0f172a',
            padding: 10,
            cornerRadius: 8,
            callbacks: {
              label: (ctx) => `${ctx.dataset.label}: ${Math.round(Math.abs(ctx.parsed.y)).toLocaleString('ru-RU')} ₽`,
            },
          },
        },
        scales: {
          x: { grid: { display: false }, ticks: { color: '#94a3b8' } },
          y: {
            beginAtZero: true,
            grid: { color: '#f1f5f9' },
            ticks: { color: '#94a3b8', precision: 0, callback: formatAxisValue },
          },
        },
      },
    }))
  },
  beforeUnmount() {
    this.chart?.destroy()
  },
  computed: {
    // See the identical note in App.DonutChart — one watcher over a combined signature avoids
    // firing chart.update() once per changed prop in the same tick.
    signature() {
      return JSON.stringify([this.labels, this.income, this.expense])
    },
  },
  watch: {
    signature() {
      this.refresh()
    },
  },
  methods: {
    buildData() {
      return {
        labels: this.labels,
        datasets: [
          { label: 'Доходы', data: this.income, backgroundColor: '#86efac', borderRadius: 4, maxBarThickness: 22 },
          { label: 'Расходы', data: this.expense.map((v) => -v), backgroundColor: '#fca5a5', borderRadius: 4, maxBarThickness: 22 },
        ],
      }
    },
    refresh() {
      if (!this.chart) return
      this.chart.data = this.buildData()
      this.chart.update()
    },
  },
}

App.LineChart = {
  props: { labels: Array, values: Array },
  template: `<canvas ref="canvas"></canvas>`,
  data() {
    return { chart: null }
  },
  mounted() {
    // markRaw: Chart.js instances are deeply circular (canvas ↔ chart ↔ layout boxes) and mutate
    // their own internals directly. Left in Vue's data(), the instance gets wrapped in a reactive
    // Proxy, which breaks that direct mutation (Chart.js's layout code error setting properties
    // like `fullSize` on `undefined`) and can recurse into a stack overflow. markRaw keeps it plain.
    this.chart = Vue.markRaw(new Chart(this.$refs.canvas, {
      type: 'line',
      data: this.buildData(),
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: '#0f172a',
            padding: 10,
            cornerRadius: 8,
            callbacks: { label: (ctx) => `${Math.round(ctx.parsed.y).toLocaleString('ru-RU')} ₽` },
          },
        },
        scales: {
          x: { grid: { display: false }, ticks: { color: '#94a3b8', maxTicksLimit: 8 } },
          y: {
            beginAtZero: true,
            grid: { color: '#f1f5f9' },
            ticks: { color: '#94a3b8', precision: 0, callback: formatAxisValue },
          },
        },
      },
    }))
  },
  beforeUnmount() {
    this.chart?.destroy()
  },
  computed: {
    // See the identical note in App.DonutChart — one watcher over a combined signature avoids
    // firing chart.update() once per changed prop in the same tick.
    signature() {
      return JSON.stringify([this.labels, this.values])
    },
  },
  watch: {
    signature() {
      this.refresh()
    },
  },
  methods: {
    buildData() {
      return {
        labels: this.labels,
        datasets: [
          {
            data: this.values,
            borderColor: '#3b82f6',
            backgroundColor: (ctx) => {
              const { ctx: c, chartArea } = ctx.chart
              if (!chartArea) return 'rgba(59,130,246,0.08)'
              const gradient = c.createLinearGradient(0, chartArea.top, 0, chartArea.bottom)
              gradient.addColorStop(0, 'rgba(59,130,246,0.25)')
              gradient.addColorStop(1, 'rgba(59,130,246,0.02)')
              return gradient
            },
            fill: true,
            tension: 0.35,
            pointRadius: 0,
            pointHoverRadius: 4,
            borderWidth: 2,
          },
        ],
      }
    },
    refresh() {
      if (!this.chart) return
      this.chart.data = this.buildData()
      this.chart.update()
    },
  },
}
