import dompurify from 'dompurify'

const Dygraph = require('../../dist/js/dygraphs.min.js')

export const hide = (el) => {
  el.classList.add('d-none')
  el.classList.add('d-hide')
}

export const show = (el) => {
  el.classList.remove('d-none')
  el.classList.remove('d-hide')
}

export const showLoading = (loadingTarget, elementsToHide) => {
  show(loadingTarget)
  if (!elementsToHide || !elementsToHide.length) return
  elementsToHide.forEach(element => {
    hide(element)
  })
}

export const hideLoading = (loadingTarget, elementsToShow) => {
  hide(loadingTarget)
  if (!elementsToShow || !elementsToShow.length) return
  elementsToShow.forEach(element => {
    show(element)
  })
}

export const isHidden = (el) => {
  return el.classList.contains('d-none')
}

export const isLoading = (el) => {
  return el.classList.add('loading')
}

export function legendFormatter (data) {
  let html = ''
  if (data.x == null) {
    let dashLabels = data.series.reduce((nodes, series) => {
      return `${nodes} <div class="pr-2">${series.dashHTML} ${series.labelHTML}</div>`
    }, '')
    html = `<div class="d-flex flex-wrap justify-content-center align-items-center">
              <div class="pr-3">${this.getLabels()[0]}: N/A</div>
              <div class="d-flex flex-wrap">${dashLabels}</div>
            </div>`
  } else {
    data.series.sort((a, b) => a.y > b.y ? -1 : 1)
    let extraHTML = ''
    // The circulation chart has an additional legend entry showing percent
    // difference.
    if (data.series.length === 2 && data.series[1].label.toLowerCase() === 'coin supply') {
      let predicted = data.series[0].y
      let actual = data.series[1].y
      let change = (((actual - predicted) / predicted) * 100).toFixed(2)
      extraHTML = `<div class="pr-2">&nbsp;&nbsp;Change: ${change} %</div>`
    }

    let yVals = data.series.reduce((nodes, series) => {
      if (!series.isVisible) return nodes
      let yVal = series.yHTML
      yVal = series.y
      // propotion missed/live has % sign
      if ((series.label.toLowerCase() === 'proportion live (%)' || series.label.toLowerCase() === 'proportion missed (%)')) {
        yVal += '%'
      }

      return `${nodes} <div class="pr-2">${series.dashHTML} ${series.labelHTML}: ${yVal}</div>`
    }, '')

    html = `<div class="d-flex flex-wrap justify-content-center align-items-center">
                <div class="pr-3">${this.getLabels()[0]}: ${data.xHTML}</div>
                <div class="d-flex flex-wrap"> ${yVals}</div>
            </div>${extraHTML}`
  }

  dompurify.sanitize(html)
  return html
}

export function barChartPlotter (e) {
  const ctx = e.drawingContext
  const points = e.points
  const yBottom = e.dygraph.toDomYCoord(0)

  ctx.fillStyle = darkenColor(e.color)

  // Find the minimum separation between x-values.
  // This determines the bar width.
  let minSep = Infinity
  for (let i = 1; i < points.length; i++) {
    const sep = points[i].canvasx - points[i - 1].canvasx
    if (sep < minSep) minSep = sep
  }
  const barWidth = Math.floor(2.0 / 3 * minSep)

  // Do the actual plotting.
  for (let i = 0; i < points.length; i++) {
    const p = points[i]
    const centerx = p.canvasx

    ctx.fillRect(centerx - barWidth / 2, p.canvasy, barWidth, yBottom - p.canvasy)
    ctx.strokeRect(centerx - barWidth / 2, p.canvasy, barWidth, yBottom - p.canvasy)
  }
}

function darkenColor (colorStr) {
  // Defined in dygraph-utils.js
  var color = Dygraph.toRGB_(colorStr)
  color.r = Math.floor((255 + color.r) / 2)
  color.g = Math.floor((255 + color.g) / 2)
  color.b = Math.floor((255 + color.b) / 2)
  return 'rgb(' + color.r + ',' + color.g + ',' + color.b + ')'
}

export var options = {
  axes: { y: { axisLabelWidth: 100 } },
  axisLabelFontSize: 12,
  retainDateWindow: false,
  showRangeSelector: true,
  rangeSelectorHeight: 40,
  drawPoints: true,
  pointSize: 0.25,
  legend: 'always',
  labelsSeparateLines: true,
  highlightCircleSize: 4,
  yLabelWidth: 20,
  drawAxesAtZero: true
}

export function getRandomColor () {
  const letters = '0123456789ABCDEF'
  let color = '#'
  for (let i = 0; i < 6; i++) {
    color += letters[Math.floor(Math.random() * 16)]
  }
  return color
}

export function setActiveOptionBtn (opt, optTargets) {
  optTargets.forEach(li => {
    if (li.dataset.option === opt) {
      li.classList.add('active')
    } else {
      li.classList.remove('active')
    }
  })
}
