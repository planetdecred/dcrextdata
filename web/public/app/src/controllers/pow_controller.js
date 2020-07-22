import { Controller } from 'stimulus'
import axios from 'axios'
import {
  hide,
  show,
  legendFormatter,
  setActiveOptionBtn,
  showLoading,
  hideLoading,
  selectedOption,
  insertOrUpdateQueryParam, updateQueryParam, updateZoomSelector, trimUrl, csv
} from '../utils'
import Zoom from '../helpers/zoom_helper'
import { animationFrame } from '../helpers/animation_helper'
import TurboQuery from '../helpers/turbolinks_helper'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'powFilterWrapper', 'selectedFilter', 'powTable', 'numPageWrapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton', 'viewOptionControl',
      'powRowTemplate', 'currentPage', 'selectedNum', 'powTableWrapper', 'messageView',
      'chartSourceWrapper', 'pool', 'chartWrapper', 'chartDataTypeSelector', 'dataType', 'labels',
      'chartsView', 'viewOption', 'pageSizeWrapper', 'poolDiv', 'loadingData',
      'zoomSelector', 'zoomOption', 'interval', 'graphIntervalWrapper'
    ]
  }

  initialize () {
    this.query = new TurboQuery()
    this.settings = TurboQuery.nullTemplate(['chart', 'zoom', 'scale', 'bin', 'axis', 'dataType'])

    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.query = new TurboQuery()
    this.settings = TurboQuery.nullTemplate(['chart', 'zoom', 'scale', 'bin', 'axis', 'dataType', 'page', 'view-option'])
    this.settings.chart = this.settings.chart || 'hashrate'

    this.zoomCallback = this._zoomCallback.bind(this)
    this.drawCallback = this._drawCallback.bind(this)

    this.dataType = this.dataTypeTarget.getAttribute('data-initial-value')

    // if no pool is selected, select the first on
    let noPoolSelected = true
    this.poolTargets.forEach(el => {
      if (el.checked) {
        noPoolSelected = false
      }
    })
    if (noPoolSelected) {
      this.poolTarget.checked = true
    }

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
    hide(this.chartSourceWrapperTarget)
    hide(this.zoomSelectorTarget)
    hide(this.graphIntervalWrapperTarget)
    show(this.powFilterWrapperTarget)
    hide(this.messageViewTarget)
    show(this.powTableWrapperTarget)
    show(this.numPageWrapperTarget)
    show(this.pageSizeWrapperTarget)
    hide(this.chartDataTypeSelectorTarget)
    this.nextPage = this.currentPage
    this.fetchData()
    insertOrUpdateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['view-option', 'page', 'records-per-page', 'filter'])
  }

  setChart () {
    this.selectedViewOption = 'chart'
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    hide(this.numPageWrapperTarget)
    hide(this.powFilterWrapperTarget)
    hide(this.powTableWrapperTarget)
    hide(this.messageViewTarget)
    show(this.graphIntervalWrapperTarget)
    show(this.chartSourceWrapperTarget)
    show(this.zoomSelectorTarget)
    show(this.chartWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    show(this.chartDataTypeSelectorTarget)
    this.fetchDataAndPlotGraph()
    updateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['view-option', 'pools', 'data-type'])
    // reset this table properties as they are removed from the url
    this.currentPage = 1
  }

  poolCheckChanged () {
    let selectedPools = []
    this.poolTargets.forEach(el => {
      if (el.checked) {
        selectedPools.push(el.value)
      }
    })
    this.fetchDataAndPlotGraph()
    insertOrUpdateQueryParam('pools', selectedPools.join('|'), '')
  }

  selectedFilterChanged () {
    this.currentPage = 1
    this.fetchData()
    let defaultFilter
    if (this.selectedFilterTarget.options.length > 0) {
      defaultFilter = this.selectedFilterTarget.options[0].value
    }
    insertOrUpdateQueryParam('filter', this.selectedFilterTarget.value, defaultFilter)
  }

  loadPreviousPage () {
    this.currentPage -= 1
    this.fetchData()
    insertOrUpdateQueryParam('page', this.currentPage, 1)
  }

  loadNextPage () {
    this.currentPage += 1
    this.fetchData()
    insertOrUpdateQueryParam('page', this.currentPage, 1)
  }

  numberOfRowsChanged () {
    this.currentPage = 1
    this.fetchData()
    insertOrUpdateQueryParam('page', this.currentPage, 1)
    insertOrUpdateQueryParam('records-per-page', this.selectedNumTarget.value, 20)
  }

  fetchData () {
    const selectedFilter = this.selectedFilterTarget.value
    const numberOfRows = this.selectedNumTarget.value

    let elementsToToggle = [this.powTableWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this
    axios.get(`/filteredpow?page=${_this.currentPage}&filter=${selectedFilter}&records-per-page=${numberOfRows}&view-option=${_this.selectedViewOption}`)
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
          hide(_this.powTableTarget)
          hide(_this.pageSizeWrapperTarget)
          _this.totalPageCountTarget.textContent = 0
          _this.currentPageTarget.textContent = 0
        } else {
          show(_this.powTableTarget)
          show(_this.pageSizeWrapperTarget)
          hide(_this.messageViewTarget)

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

          _this.displayPoW(result.powData)
        }
      }).catch(function (e) {
        console.log(e)
      })
  }

  displayPoW (pows) {
    const _this = this
    this.powTableTarget.innerHTML = ''

    pows.forEach(pow => {
      const powRow = document.importNode(_this.powRowTemplateTarget.content, true)
      const fields = powRow.querySelectorAll('td')

      fields[0].innerText = pow.source
      fields[1].innerText = pow.pool_hashrate_th
      fields[2].innerHTML = pow.workers
      fields[4].innerHTML = pow.time

      _this.powTableTarget.appendChild(powRow)
    })
  }

  setDataType (event) {
    this.dataType = event.currentTarget.getAttribute('data-option')
    setActiveOptionBtn(this.dataType, this.dataTypeTargets)

    // this.btcIndex = this.poolTargets.findIndex(el => el.value === 'btc')
    this.f2poolIndex = this.poolTargets.findIndex(el => el.value === 'f2pool')
    if (this.dataType === 'workers') {
      // hide(this.poolDivTargets[this.btcIndex])
      hide(this.poolDivTargets[this.f2poolIndex])
    } else {
      // show(this.poolDivTargets[this.btcIndex])
      show(this.poolDivTargets[this.f2poolIndex])
    }

    this.fetchDataAndPlotGraph()
    insertOrUpdateQueryParam('data-type', this.dataType, 'pool_hashrate')
  }

  fetchDataAndPlotGraph () {
    hide(this.messageViewTarget)
    this.selectedPools = []
    this.poolTargets.forEach(el => {
      if (el.checked) {
        this.selectedPools.push(el.value)
      }
    })

    let elementsToToggle = [this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this

    axios.get(`/api/charts/pow/${this.dataType}?extras=${this.selectedPools.join('|')}&bin=${this.selectedInterval()}`).then(function (response) {
      let result = response.data
      if (result.error) {
        _this.messageViewTarget.innerHTML = `<p class="text-danger">${result.error}</p>`
        show(_this.messageViewTarget)
        hide(_this.loadingDataTarget)
        return
      }

      if (result.x.length === 0) {
        _this.messageViewTarget.innerHTML = '<p class="text-danger">No record found</p>'
        show(_this.messageViewTarget)
        hide(_this.loadingDataTarget)
        return
      }
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      _this.plotGraph(result)
    }).catch(function (e) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      console.log(e)
    })
  }

  selectedInterval () { return selectedOption(this.intervalTargets) }

  setInterval (e) {
    const option = e.currentTarget.dataset.option
    setActiveOptionBtn(option, this.intervalTargets)
    this.fetchDataAndPlotGraph()
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
  plotGraph (data) {
    const _this = this
    let dataTypeLabel = 'Pool Hashrate (Th/s)'
    if (_this.dataType === 'workers') {
      dataTypeLabel = 'Workers'
    }
    let minDate, maxDate
    data.x.forEach(unixTime => {
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
      ylabel: dataTypeLabel,
      xlabel: 'Date',
      labels: ['Date', ...this.selectedPools],
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

    _this.chartsView = new Dygraph(_this.chartsViewTarget, csv(data, this.selectedPools.length), options)
    _this.validateZoom()

    updateZoomSelector(_this.zoomOptionTargets, minDate, maxDate)
    show(this.zoomSelectorTarget)
  }
}
