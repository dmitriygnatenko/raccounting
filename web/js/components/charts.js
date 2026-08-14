window.App = window.App || {};

Chart.defaults.font.family = "'Inter', system-ui, sans-serif"
Chart.defaults.color = '#64748b'

App.DonutChart = {
  props: { labels: Array, values: Array, colors: Array },
  template: `<canvas ref="canvas"></canvas>`,
  data() {
    return { chart: null }
  },
  mounted() {
    this.chart = new Chart(this.$refs.canvas, {
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
    })
  },
  beforeUnmount() {
    this.chart?.destroy()
  },
  watch: {
    values() {
      this.refresh()
    },
    labels() {
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
    this.chart = new Chart(this.$refs.canvas, {
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
            grid: { color: '#f1f5f9' },
            ticks: { color: '#94a3b8', callback: (v) => `${Math.abs(Number(v)) / 1000}k` },
          },
        },
      },
    })
  },
  beforeUnmount() {
    this.chart?.destroy()
  },
  watch: {
    income() {
      this.refresh()
    },
    expense() {
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
    this.chart = new Chart(this.$refs.canvas, {
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
          y: { grid: { color: '#f1f5f9' }, ticks: { color: '#94a3b8', callback: (v) => `${Number(v) / 1000}k` } },
        },
      },
    })
  },
  beforeUnmount() {
    this.chart?.destroy()
  },
  watch: {
    values() {
      this.refresh()
    },
    labels() {
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
