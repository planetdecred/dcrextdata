import { Controller } from 'stimulus'
import axios from 'axios'
import {
  hide,
  show,
  legendFormatter,
  setActiveOptionBtn,
  showLoading,
  hideLoading,
  options,
  selectedOption, insertOrUpdateQueryParam, updateQueryParam, updateZoomSelector, trimUrl, csv, notifyFailure
} from '../utils'
import TurboQuery from '../helpers/turbolinks_helper'
import Zoom from '../helpers/zoom_helper'
import { animationFrame } from '../helpers/animation_helper'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'selectedFilter', 'vspTicksTable', 'numPageWrapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton', 'messageView',
      'vspRowTemplate', 'currentPage', 'selectedNum', 'vspTableWrapper',
      'graphTypeWrapper', 'dataType', 'pageSizeWrapper', 'viewOptionControl',
      'vspSelectorWrapper', 'chartSourceWrapper', 'allChartSource', 'chartSource',
      'chartWrapper', 'labels', 'chartsView', 'viewOption', 'loadingData',
      'zoomSelector', 'zoomOption', 'interval', 'graphIntervalWrapper'
    ]
  }

  initialize () {
    this.query = new TurboQuery()
    this.settings = TurboQuery.nullTemplate(['chart', 'zoom', 'scale', 'bin', 'axis', 'dataType'])
    this.query.update(this.settings)

    if (this.settings.zoom) {
      setActiveOptionBtn(this.settings.zoom, this.zoomOptionTargets)
    }
    if (this.settings.bin) {
      setActiveOptionBtn(this.settings.bin, this.intervalTargets)
    }

    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.query = new TurboQuery()
    this.settings = TurboQuery.nullTemplate(['chart', 'zoom', 'scale', 'bin', 'axis', 'dataType', 'page', 'view-option'])
    this.settings.chart = this.settings.chart || 'mempool'

    this.zoomCallback = this._zoomCallback.bind(this)
    this.drawCallback = this._drawCallback.bind(this)

    this.vsps = []
    this.chartSourceTargets.forEach(chartSource => {
      if (chartSource.checked) {
        this.vsps.push(chartSource.value)
      }
    })

    this.dataType = this.dataTypeTarget.value = this.dataTypeTarget.getAttribute('data-initial-value')

    // if no vsp is selected, select the first one
    let noVspSelected = true
    let allVspSelected = true
    this.chartSourceTargets.forEach(el => {
      if (el.checked) {
        noVspSelected = false
      } else {
        allVspSelected = false
      }
    })
    if (noVspSelected) {
      this.chartSourceTarget.checked = true
    }

    this.allChartSourceTarget.checked = allVspSelected

    this.selectedViewOption = this.viewOptionControlTarget.getAttribute('data-initial-value')
    if (this.selectedViewOption === 'chart') {
      this.setChart()
    } else {
      this.setTable()
    }
  }

  setTable () {
    this.selectedViewOption = 'table'
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    hide(this.chartWrapperTarget)
    hide(this.graphTypeWrapperTarget)
    hide(this.chartSourceWrapperTarget)
    hide(this.zoomSelectorTarget)
    hide(this.graphIntervalWrapperTarget)
    show(this.vspTableWrapperTarget)
    hide(this.messageViewTarget)
    show(this.numPageWrapperTarget)
    show(this.pageSizeWrapperTarget)
    show(this.vspSelectorWrapperTarget)
    this.nextPage = this.currentPage
    this.fetchData()
    insertOrUpdateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['view-option', 'page', 'records-per-page', 'filter'])
  }

  setChart () {
    this.selectedViewOption = 'chart'
    hide(this.numPageWrapperTarget)
    hide(this.vspTableWrapperTarget)
    hide(this.messageViewTarget)
    hide(this.vspSelectorWrapperTarget)
    show(this.graphTypeWrapperTarget)
    show(this.chartWrapperTarget)
    show(this.chartSourceWrapperTarget)
    show(this.graphIntervalWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    updateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['data-type', 'view-option', 'zoom', 'bin'])
    // reset this table properties as they are removed from the url
    this.nextPage = 1
    this.selectedNumTarget.value = 20
    this.fetchDataAndPlotGraph()
  }

  selectedFilterChanged () {
    if (this.selectedViewOption === 'table') {
      this.nextPage = 1
      this.fetchData()
    } else {
      if (this.selectedFilterTarget.selectedIndex === 0) {
        this.selectedFilterTarget.selectedIndex = 1
      }
      this.fetchDataAndPlotGraph()
    }
    let defaultFilter
    if (this.selectedFilterTarget.options.length > 0) {
      defaultFilter = this.selectedFilterTarget.options[0].value
    }
    insertOrUpdateQueryParam('filter', this.selectedFilterTarget.value, defaultFilter)
  }

  loadPreviousPage () {
    this.nextPage = this.currentPage - 1
    this.fetchData()
    insertOrUpdateQueryParam('page', this.currentPage - 1, 1)
  }

  loadNextPage () {
    this.nextPage = this.currentPage + 1
    this.fetchData()
    insertOrUpdateQueryParam('page', this.currentPage + 1, 1)
  }

  numberOfRowsChanged () {
    this.nextPage = 1
    this.fetchData()
    insertOrUpdateQueryParam('records-per-page', parseInt(this.selectedNumTarget.value), 20)
  }

  fetchData () {
    const selectedFilter = this.selectedFilterTarget.value
    const numberOfRows = this.selectedNumTarget.value

    let elementsToToggle = [this.vspTableWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this
    axios.get(`/vsps?page=${this.nextPage}&filter=${selectedFilter}&records-per-page=${numberOfRows}&view-option=${_this.selectedViewOption}`)
      .then(function (response) {
        hideLoading(_this.loadingDataTarget, elementsToToggle)
        let result = response.data
        if (result.message) {
          let messageHTML = ''
          messageHTML += `<div class="alert alert-primary">
                         <strong>${result.message}</strong>
                    </div>`

          _this.messageViewTarget.innerHTML = messageHTML
          show(_this.messageViewTarget)
          hide(_this.vspTicksTableTarget)
          hide(_this.pageSizeWrapperTarget)
        } else {
          hide(_this.messageViewTarget)
          show(_this.vspTicksTableTarget)
          show(_this.pageSizeWrapperTarget)
          _this.currentPage = result.currentPage
          if (_this.currentPage <= 1) {
            hide(_this.previousPageButtonTarget)
          } else {
            show(_this.previousPageButtonTarget)
          }

          if (_this.currentPage >= result.totalPages) {
            hide(_this.nextPageButtonTarget)
          } else {
            show(_this.nextPageButtonTarget)
          }
          _this.totalPageCountTarget.textContent = result.totalPages
          _this.currentPageTarget.textContent = result.currentPage

          _this.displayVSPs(result.vspData)
        }
      }).catch(function (e) {
        _this.drawInitialGraph()
      })
  }

  displayVSPs (vsps) {
    const _this = this
    this.vspTicksTableTarget.innerHTML = ''

    vsps.forEach(vsp => {
      const vspRow = document.importNode(_this.vspRowTemplateTarget.content, true)
      const fields = vspRow.querySelectorAll('td')

      fields[0].innerText = vsp.vsp
      fields[1].innerText = vsp.immature
      fields[2].innerText = vsp.live
      fields[3].innerHTML = vsp.voted
      fields[4].innerHTML = vsp.missed
      fields[5].innerHTML = vsp.pool_fees
      fields[6].innerText = vsp.proportion_live
      fields[7].innerHTML = vsp.proportion_missed
      fields[8].innerHTML = vsp.user_count
      fields[9].innerHTML = vsp.users_active
      fields[10].innerHTML = vsp.time

      _this.vspTicksTableTarget.appendChild(vspRow)
    })
  }

  chartSourceCheckChanged (e) {
    // allow a maximum of 5 sources to avoid server overload
    let count = 0
    this.chartSourceTargets.forEach(el => {
      if (el.checked) {
        count++
      }
    })
    if (count > 5) {
      notifyFailure('You cannot compare more than 5 sources')
      e.currentTarget.checked = false
      return
    }
    this.fetchDataAndPlotGraph()
  }

  vspCheckboxCheckChanged (event) {
    const checked = event.currentTarget.checked
    this.chartSourceTargets.forEach(el => {
      el.checked = checked
    })
    this.fetchDataAndPlotGraph()
  }

  dataTypeChanged () {
    this.dataType = this.dataTypeTarget.value
    this.fetchDataAndPlotGraph()
    insertOrUpdateQueryParam('data-type', this.dataType, 'Immature')
  }

  fetchDataAndPlotGraph () {
    this.vsps = []
    this.chartSourceTargets.forEach(chartSource => {
      if (chartSource.checked) {
        this.vsps.push(chartSource.value)
      }
    })

    let elementsToToggle = [this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    let _this = this
    const queryString = `extras=${this.vsps.join('|')}&bin=${this.selectedInterval()}`
    axios.get(`/api/charts/vsp/${this.dataType}?${queryString}`).then(function (response) {
      let result = response.data
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      if (result.error) {
        _this.drawInitialGraph()
        return
      }
      _this.plotGraph(result)
    }).catch(function (e) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      _this.drawInitialGraph()
    })
  }

  selectedInterval () { return selectedOption(this.intervalTargets) }

  setInterval (e) {
    const option = e.currentTarget.dataset.option
    setActiveOptionBtn(option, this.intervalTargets)
    this.fetchDataAndPlotGraph()
    insertOrUpdateQueryParam('bin', option, 'day')
  }

  selectedZoom () { return selectedOption(this.zoomOptionTargets) }

  setZoom (e) {
    var target = e.srcElement || e.target
    var option
    if (!target) {
      let ex = this.chartsView.xAxisExtremes()
      option = Zoom.mapKey(e, ex, 1)
    } else {
      option = target.dataset.option
    }
    setActiveOptionBtn(option, this.zoomOptionTargets)
    if (!target) return // Exit if running for the first time
    insertOrUpdateQueryParam('zoom', option, 'all')
    this.validateZoom()
  }

  async validateZoom () {
    await animationFrame()
    await animationFrame()
    let oldLimits = this.limits || this.chartsView.xAxisExtremes()
    this.limits = this.chartsView.xAxisExtremes()
    var selected = this.selectedZoom()
    if (selected) {
      this.lastZoom = Zoom.validate(selected, this.limits, 1, 1)
    } else {
      this.lastZoom = Zoom.project(this.settings.zoom, oldLimits, this.limits)
    }
    if (this.lastZoom) {
      this.chartsView.updateOptions({
        dateWindow: [this.lastZoom.start, this.lastZoom.end]
      })
    }
    if (selected !== this.settings.zoom) {
      this._zoomCallback(this.lastZoom.start, this.lastZoom.end)
    }
    await animationFrame()
    this.chartsView.updateOptions({
      zoomCallback: this.zoomCallback,
      drawCallback: this.drawCallback
    })
  }

  _zoomCallback (start, end) {
    this.lastZoom = Zoom.object(start, end)
    this.settings.zoom = Zoom.encode(this.lastZoom)
    // this.query.replace(this.settings)
    let ex = this.chartsView.xAxisExtremes()
    let option = Zoom.mapKey(this.settings.zoom, ex, 1)
    setActiveOptionBtn(option, this.zoomOptionTargets)
  }

  _drawCallback (graph, first) {
    if (first) return
    var start, end
    [start, end] = this.chartsView.xAxisRange()
    if (start === end) return
    if (this.lastZoom.start === start) return // only handle slide event.
    this._zoomCallback(start, end)
  }

  // vsp chart
  plotGraph (dataSet) {
    const _this = this
    _this.yLabel = this.dataType.split('_').join(' ')
    if ((_this.yLabel.toLowerCase() === 'proportion live' || _this.yLabel.toLowerCase() === 'proportion missed')) {
      _this.yLabel += ' (%)'
    }
    if (_this.yLabel === '') {
      _this.yLabel = 'n/a'
    }
    let minDate, maxDate

    dataSet.x.forEach(unixTime => {
      let date = new Date(unixTime * 1000)
      if (minDate === undefined || date < minDate) {
        minDate = date
      }

      if (maxDate === undefined || date > maxDate) {
        maxDate = date
      }
    })

    let options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: legendFormatter,
      labelsDiv: _this.labelsTarget,
      ylabel: _this.yLabel,
      xlabel: 'Date',
      labels: ['Date', ...this.vsps],
      labelsUTC: true,
      labelsKMB: true,
      connectSeparatedPoints: true,
      showRangeSelector: true,
      axes: {
        x: {
          drawGrid: false
        }
      }
    }
    _this.chartsView = new Dygraph(
      _this.chartsViewTarget,
      csv(dataSet, this.vsps.length),
      options
    )

    updateZoomSelector(_this.zoomOptionTargets, minDate, maxDate)
    show(this.zoomSelectorTarget)
  }

  drawInitialGraph () {
    var extra = {
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: this.yLabel,
      xlabel: 'Date',
      labelsUTC: true,
      labelsKMB: true,
      axes: {
        x: {
          drawGrid: false
        }
      }
    }

    this.chartsView = new Dygraph(
      this.chartsViewTarget,
      [[0, 0]],
      { ...options, ...extra }
    )
  }
}
