import { Controller } from 'stimulus'
import axios from 'axios'
import {
  hide,
  show,
  legendFormatter,
  setActiveOptionBtn,
  options,
  showLoading,
  hideLoading,
  selectedOption, updateQueryParam, insertOrUpdateQueryParam, updateZoomSelector, trimUrl, zipXYZData
} from '../utils'
import Zoom from '../helpers/zoom_helper'
import { animationFrame } from '../helpers/animation_helper'
import TurboQuery from '../helpers/turbolinks_helper'

const Dygraph = require('../../../dist/js/dygraphs.min.js')
var chartData

export default class extends Controller {
  static get targets () {
    return [
      'selectedFilter', 'exchangeTable', 'selectedCurrencyPair', 'numPageWrapper', 'intervalsWapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton', 'selectedTicks', 'selectedInterval', 'loadingData',
      'exRowTemplate', 'currentPage', 'selectedNum', 'exchangeTableWrapper', 'tickWapper', 'viewOptionControl',
      'chartWrapper', 'labels', 'chartsView', 'selectedViewOption', 'hideOption', 'sourceWrapper', 'chartSelector',
      'pageSizeWrapper', 'chartSource', 'messageView', 'hideIntervalOption', 'viewOption',
      'zoomSelector', 'zoomOption', 'drawGridWrapper', 'drawGrid'
    ]
  }

  initialize () {
    this.selectedFilter = this.selectedFilterTarget.value
    this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value
    this.numberOfRows = this.selectedNumTarget.value
    this.selectedInterval = this.selectedIntervalTarget.value

    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.query = new TurboQuery()
    this.settings = TurboQuery.nullTemplate(['chart', 'zoom', 'scale', 'bin', 'axis', 'dataType', 'page', 'view-option'])
    this.query.update(this.settings)
    if (this.settings.zoom) {
      setActiveOptionBtn(this.settings.zoom, this.zoomOptionTargets)
    }
    this.settings.chart = this.settings.chart || 'mempool'

    this.zoomCallback = this._zoomCallback.bind(this)
    this.drawCallback = this._drawCallback.bind(this)

    this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value = this.selectedCurrencyPairTarget.getAttribute('data-initial-value')
    this.selectedInterval = this.selectedIntervalTarget.value = this.selectedIntervalTarget.getAttribute('data-initial-value')
    this.selectedExchange = this.selectedFilterTarget.value = this.selectedFilterTarget.getAttribute('data-initial-value')
    this.selectedTick = this.selectedTicksTarget.value = this.selectedTicksTarget.getAttribute('data-initial-value')

    this.selectedViewOption = this.viewOptionControlTarget.getAttribute('data-initial-value')
    if (this.selectedViewOption === 'chart') {
      this.setChart()
    } else {
      this.setTable()
    }
  }

  async setTable () {
    this.selectedViewOption = 'table'
    hide(this.messageViewTarget)
    hide(this.tickWapperTarget)
    hide(this.drawGridWrapperTarget)
    show(this.hideOptionTarget)
    show(this.pageSizeWrapperTarget)
    hide(this.chartWrapperTarget)
    hide(this.zoomSelectorTarget)
    show(this.exchangeTableWrapperTarget)
    show(this.numPageWrapperTarget)
    this.resetCommonFilter()
    this.selectedExchange = this.selectedFilterTarget.value
    this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value
    this.numberOfRows = this.selectedNumTarget.value
    this.selectedInterval = this.selectedIntervalTarget.value
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    this.nextPage = this.currentPage
    await this.loadCurrencyPairs()
    await this.loadIntervals()
    this.fetchExchange(this.selectedViewOption)
    insertOrUpdateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['page', 'records-per-page', 'view-option', 'selected-currency-pair', 'selected-exchange', 'selected-interval'])
  }

  async setChart () {
    this.selectedViewOption = 'chart'
    hide(this.messageViewTarget)
    show(this.chartWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    show(this.tickWapperTarget)
    show(this.zoomSelectorTarget)
    show(this.drawGridWrapperTarget)
    hide(this.hideOptionTarget)
    hide(this.messageViewTarget)
    hide(this.numPageWrapperTarget)
    hide(this.exchangeTableWrapperTarget)
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    this.resetCommonFilter()
    if (this.selectedCurrencyPair === '' || this.selectedCurrencyPair === 'All') {
      this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value = this.selectedCurrencyPairTarget.options[1].text
      this.selectedCurrencyPairTarget.text = this.selectedCurrencyPair
    } else {
      this.selectedCurrencyPairTarget.value = this.selectedCurrencyPair
    }
    if (this.selectedExchange === '' || this.selectedExchange === 'All') {
      this.selectedExchange = this.selectedFilterTarget.value = this.selectedFilterTarget.options[1].text
      this.selectedFilterTarget.text = this.selectedExchange
    }
    await this.loadCurrencyPairs()
    await this.loadIntervals()
    this.fetchExchange(this.selectedViewOption)
    updateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['selected-tick', 'view-option', 'selected-currency-pair', 'selected-exchange', 'selected-interval', 'zoom'])
    // reset this table properties as they are removed from the url
    this.currentPage = 1
    this.selectedNumTarget.value = 20
  }

  resetCommonFilter () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value = this.selectedCurrencyPairTarget.getAttribute('data-initial-value')
    this.selectedInterval = this.selectedIntervalTarget.value = this.selectedIntervalTarget.getAttribute('data-initial-value')
    this.selectedExchange = this.selectedFilterTarget.value = this.selectedFilterTarget.getAttribute('data-initial-value')
    this.selectedTick = this.selectedTicksTarget.value = this.selectedTicksTarget.getAttribute('data-initial-value')
  }

  selectedIntervalChanged () {
    this.nextPage = 1
    this.selectedInterval = this.selectedIntervalTarget.value
    this.fetchExchange(this.selectedViewOption)
    let defaultInterval
    if (this.selectedIntervalTarget.options.length > 0) {
      defaultInterval = this.selectedIntervalTarget.options[0].value
    }
    insertOrUpdateQueryParam('selected-interval', this.selectedInterval, defaultInterval)
    insertOrUpdateQueryParam('page', this.nextPage, 1)
  }

  selectedTicksChanged () {
    this.selectedTick = this.selectedTicksTarget.value
    this.fetchExchange(this.selectedViewOption)
    let defaultTick
    if (this.selectedTicksTarget.options.length > 0) {
      defaultTick = this.selectedTicksTarget.options[0].value
    }
    insertOrUpdateQueryParam('selected-tick', this.selectedTick, defaultTick)
  }

  async selectedFilterChanged () {
    this.nextPage = 1
    this.selectedExchange = this.selectedFilterTarget.value
    await this.loadCurrencyPairs()
    await this.loadIntervals()
    this.fetchExchange(this.selectedViewOption)
    let defaultFilter
    if (this.selectedFilterTarget.options.length > 0) {
      defaultFilter = this.selectedFilterTarget.options[0].value
    }
    insertOrUpdateQueryParam('selected-exchange', this.selectedExchange, defaultFilter)
  }

  loadPreviousPage () {
    this.nextPage = this.currentPage - 1
    if (this.nextPage < 1) {
      this.nextPage = 1
    }
    this.fetchExchange(this.selectedViewOption)
    insertOrUpdateQueryParam('page', this.nextPage, 1)
  }

  loadNextPage () {
    this.nextPage = this.currentPage + 1
    this.fetchExchange(this.selectedViewOption)
    insertOrUpdateQueryParam('page', this.currentPage + 1, 1)
  }

  async selectedCurrencyPairChanged () {
    this.nextPage = 1
    this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value
    await this.loadIntervals()
    this.fetchExchange(this.selectedViewOption)
    let defaultPair
    if (this.selectedCurrencyPairTarget.options.length > 0) {
      defaultPair = this.selectedCurrencyPairTarget.options[0].value
    }
    insertOrUpdateQueryParam('selected-currency-pair', this.selectedCurrencyPair, defaultPair)
  }

  async loadIntervals () {
    var response = await axios.get(`/api/exchanges/intervals?currency-pair=${this.selectedCurrencyPair}&exchange=${this.selectedExchange}`)
    if (response.data.error) {
      window.alert(response.data.error)
      return
    }
    let selectedDropped = true
    this.selectedIntervalTarget.innerHTML = ''
    let options = ''
    response.data.forEach(p => {
      if (p.value === parseInt(this.selectedInterval)) {
        selectedDropped = false
      }
      options += `<option value="${p.value}">${p.label}</option>`
    })
    this.selectedIntervalTarget.innerHTML = options
    if (selectedDropped && response.data.length > 1) {
      this.selectedInterval = this.selectedIntervalTarget.options[1].value
    }
    this.selectedIntervalTarget.value = this.selectedInterval
    if (this.selectedViewOption === 'chart') {
      hide(this.selectedIntervalTarget.options[0])
    }
  }

  async loadCurrencyPairs () {
    var response = await axios.get(`/api/exchanges/currency-pairs?exchange=${this.selectedExchange}`)
    if (response.data.error) {
      window.alert(response.data.error)
      return
    }
    let selectedDropped = true
    this.selectedCurrencyPairTarget.innerHTML = ''
    let options = ''
    response.data.forEach(p => {
      if (p.value === this.selectedCurrencyPair) {
        selectedDropped = false
      }
      options += `<option value="${p}">${p}</option>`
    })
    this.selectedCurrencyPairTarget.innerHTML = options
    if (selectedDropped && response.data.length > 1) {
      this.selectedCurrencyPair = this.selectedCurrencyPairTarget.options[1].value
    }
    this.selectedCurrencyPairTarget.value = this.selectedCurrencyPair
    if (this.selectedViewOption === 'chart' || response.data.length <= 2) {
      hide(this.selectedCurrencyPairTarget.options[0])
    }
  }

  numberOfRowsChanged () {
    this.nextPage = 1
    this.numberOfRows = this.selectedNumTarget.value
    this.fetchExchange(this.selectedViewOption)
    insertOrUpdateQueryParam('records-per-page', this.numberOfRows, 20)
  }

  fetchExchange (display) {
    const _this = this

    let elementsToToggle = [this.exchangeTableWrapperTarget, this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    var url
    if (display === 'table') {
      url = `/exchangedata?page=${_this.nextPage}&selected-exchange=${_this.selectedExchange}&records-per-page=${_this.numberOfRows}&selected-currency-pair=${_this.selectedCurrencyPair}&selected-interval=${_this.selectedInterval}&view-option=${_this.selectedViewOption}`
    } else {
      const queryString = `selected-currency-pair=${_this.selectedCurrencyPair}&selected-interval=${_this.selectedInterval}&selected-exchange=${_this.selectedExchange}`
      url = `/api/charts/exchange/${_this.selectedTick}?${queryString}`
    }

    axios.get(url)
      .then(function (response) {
        let result = response.data
        if (display === 'table') {
          hideLoading(_this.loadingDataTarget, [_this.exchangeTableWrapperTarget])
          if (result.message) {
            let messageHTML = ''
            messageHTML += `<div class="alert alert-primary">
                           <strong>${result.message}</strong>
                      </div>`

            _this.messageViewTarget.innerHTML = messageHTML
            show(_this.messageViewTarget)
            hide(_this.exchangeTableTarget)
            hide(_this.pageSizeWrapperTarget)
            _this.totalPageCountTarget.textContent = 0
            _this.currentPageTarget.textContent = 0
            _this.selectedFilterTarget.value = _this.selectedFilterTarget.getAttribute('data-initial-value')
          } else {
            hide(_this.messageViewTarget)
            show(_this.exchangeTableTarget)
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

            _this.selectedIntervalTarget.value = result.selectedInterval
            _this.selectedFilterTarget.value = _this.selectedExchange
            _this.selectedNumTarget.value = result.selectedNum
            _this.selectedCurrencyPairTarget.value = result.selectedCurrencyPair
            _this.totalPageCountTarget.textContent = result.totalPages
            _this.currentPageTarget.textContent = result.currentPage
            _this.displayExchange(result)
          }
        } else {
          if (!result.x) {
            hideLoading(_this.loadingDataTarget, [_this.chartWrapperTarget])
            _this.drawInitialGraph()
          } else {
            hideLoading(_this.loadingDataTarget, [_this.chartWrapperTarget])
            chartData = result
            _this.plotGraph(result)
          }
        }
      }).catch(function (e) {
        console.log(e)
      })
  }

  displayExchange (exs) {
    hide(this.messageViewTarget)
    show(this.exchangeTableWrapperTarget)
    const _this = this
    this.exchangeTableTarget.innerHTML = ''

    exs.exData.forEach(ex => {
      const exRow = document.importNode(_this.exRowTemplateTarget.content, true)
      const fields = exRow.querySelectorAll('td')

      fields[0].innerHTML = ex.time
      fields[1].innerText = ex.exchange_name
      fields[2].innerText = ex.high
      fields[3].innerText = ex.low
      fields[4].innerHTML = ex.open
      fields[5].innerHTML = ex.close
      fields[6].innerHTML = ex.volume
      fields[7].innerText = ex.interval
      if (ex.currency_pair === 'BTC/DCR') {
        fields[8].innerHTML = 'DCR/BTC'
      } else if (ex.currency_pair === 'USD/BTC') {
        fields[8].innerHTML = 'BTC/USD'
      }

      _this.exchangeTableTarget.appendChild(exRow)
    })
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

  drawGridCheckChanged () {
    this.plotGraph()
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

  // exchange chart
  plotGraph () {
    const _this = this
    let minDate, maxDate
    chartData.x.forEach(unixTime => {
      let date = new Date(unixTime * 1000)
      if (minDate === undefined || date < minDate) {
        minDate = date
      }

      if (maxDate === undefined || date > maxDate) {
        maxDate = date
      }
    })

    _this.labels = ['Date', _this.selectedExchange]
    let colors = ['#007bff']

    var extra = {
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: 'Price',
      xlabel: 'Date',
      labels: _this.labels,
      colors: colors,
      digitsAfterDecimal: 8,
      drawGrid: this.drawGridTarget.checked
    }

    const data = zipXYZData(chartData)

    _this.chartsView = new Dygraph(
      _this.chartsViewTarget,
      data, { ...options, ...extra }
    )

    _this.validateZoom()

    if (updateZoomSelector(_this.zoomOptionTargets, minDate, maxDate)) {
      show(this.zoomSelectorTarget)
    } else {
      hide(this.zoomSelectorTarget)
    }
  }

  drawInitialGraph () {
    var extra = {
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: 'Price',
      xlabel: 'Date',
      labels: ['Date', this.selectedExchange],
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
