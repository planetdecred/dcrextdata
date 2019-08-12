import { Controller } from 'stimulus'
import axios from 'axios'
import { hide, show, legendFormatter, setActiveOptionBtn, options, appName, showLoading, hideLoading } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'selectedFilter', 'exchangeTable', 'selectedCurrencyPair', 'numPageWrapper', 'intervalsWapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton', 'selectedTicks', 'selectedInterval', 'loadingData',
      'exRowTemplate', 'currentPage', 'selectedNum', 'exchangeTableWrapper', 'tickWapper', 'viewOptionControl',
      'chartWrapper', 'labels', 'chartsView', 'selectedViewOption', 'hideOption', 'sourceWrapper', 'chartSelector',
      'pageSizeWrapper', 'chartSource', 'currencyPairHideOption', 'messageView', 'hideIntervalOption', 'viewOption'
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

  setTable () {
    this.selectedViewOption = 'table'
    hide(this.messageViewTarget)
    hide(this.tickWapperTarget)
    show(this.hideOptionTarget)
    show(this.pageSizeWrapperTarget)
    hide(this.chartWrapperTarget)
    show(this.selectedIntervalTarget.options[0])
    show(this.currencyPairHideOptionTarget)
    show(this.exchangeTableWrapperTarget)
    show(this.numPageWrapperTarget)
    this.resetCommonFilter()
    this.selectedExchange = this.selectedFilterTarget.value
    this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value
    this.numberOfRows = this.selectedNumTarget.value
    this.selectedInterval = this.selectedIntervalTarget.value
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    this.nextPage = this.currentPage
    this.fetchExchange(this.selectedViewOption)
  }

  setChart () {
    this.selectedViewOption = 'chart'
    hide(this.messageViewTarget)
    var intervals = this.selectedIntervalTarget.options
    show(this.chartWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    show(this.tickWapperTarget)
    hide(this.hideOptionTarget)
    hide(intervals[0])
    hide(this.currencyPairHideOptionTarget)
    hide(this.numPageWrapperTarget)
    hide(this.exchangeTableWrapperTarget)
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    this.resetCommonFilter()
    if (this.selectedCurrencyPair === '' || this.selectedCurrencyPair === 'All') {
      this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value = this.selectedCurrencyPairTarget.options[1].text
      this.selectedCurrencyPairTarget.text = this.selectedCurrencyPair
    }
    if (this.selectedExchange === '' || this.selectedExchange === 'All') {
      this.selectedExchange = this.selectedFilterTarget.value = this.selectedFilterTarget.options[1].text
      this.selectedFilterTarget.text = this.selectedExchange
    }
    this.fetchExchange(this.selectedViewOption)
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
  }

  selectedTicksChanged () {
    this.selectedTick = this.selectedTicksTarget.value
    this.fetchExchange(this.selectedViewOption)
  }

  selectedFilterChanged () {
    this.nextPage = 1
    this.selectedExchange = this.selectedFilterTarget.value
    this.fetchExchange(this.selectedViewOption)
  }

  loadPreviousPage () {
    this.nextPage = this.currentPage - 1
    this.fetchExchange(this.selectedViewOption)
  }

  loadNextPage () {
    this.nextPage = this.currentPage + 1
    this.fetchExchange(this.selectedViewOption)
  }

  selectedCurrencyPairChanged () {
    this.nextPage = 1
    this.selectedCurrencyPair = this.selectedCurrencyPairTarget.value
    this.fetchExchange(this.selectedViewOption)
  }

  NumberOfRowsChanged () {
    this.nextPage = 1
    this.numberOfRows = this.selectedNumTarget.value
    this.fetchExchange(this.selectedViewOption)
  }

  fetchExchange (display) {
    const _this = this

    let elementsToToggle = [this.exchangeTableWrapperTarget, this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    var url
    if (display === 'table') {
      url = `/exchangedata?page=${_this.nextPage}&selected-exchange=${_this.selectedExchange}&records-per-page=${_this.numberOfRows}&selected-currency-pair=${_this.selectedCurrencyPair}&selected-interval=${_this.selectedInterval}&view-option=${_this.selectedViewOption}`
    } else {
      const queryString = `selected-tick=${_this.selectedTick}&selected-currency-pair=${_this.selectedCurrencyPair}&selected-interval=${_this.selectedInterval}&selected-exchange=${_this.selectedExchange}&view-option=${_this.selectedViewOption}`
      window.history.pushState(window.history.state, appName, `/exchanges?${queryString}`)
      url = `/exchangechart?${queryString}`
    }

    axios.get(url)
      .then(function (response) {
        console.log(response.data)
        let result = response.data
        if (display === 'table') {
          hideLoading(_this.loadingDataTarget, [_this.exchangeTableWrapperTarget])
          if (result.message) {
            let messageHTML = ''
            messageHTML += `<div class="alert alert-primary align-text-center">
                           <strong>${result.message}</strong>
                      </div>`

            _this.messageViewTarget.innerHTML = messageHTML
            show(_this.messageViewTarget)
            hide(_this.exchangeTableWrapperTarget)
            hide(_this.pageSizeWrapperTarget)
            _this.totalPageCountTarget.textContent = 0
            _this.currentPageTarget.textContent = 0
            window.history.pushState(window.history.state, appName, `/exchanges?page=${_this.nextPage}&selected-exchange=${_this.selectedExchange}&records-per-page=${_this.numberOfRows}&selected-currency-pair=${_this.selectedCurrencyPair}&selected-interval=${_this.selectedInterval}&view-option=${_this.selectedViewOption}`)
          } else {
            window.history.pushState(window.history.state, appName, `/exchanges?page=${result.currentPage}&selected-exchange=${_this.selectedExchange}&records-per-page=${result.selectedNum}&selected-currency-pair=${result.selectedCurrencyPair}&selected-interval=${result.selectedInterval}&view-option=${result.selectedViewOption}`)
            hide(_this.messageViewTarget)
            show(_this.exchangeTableWrapperTarget)
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
          hideLoading(_this.loadingDataTarget, [_this.chartWrapperTarget])
          if (result.chartData) {
            _this.plotGraph(result.chartData)
          } else {
            _this.drawInitialGraph()
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
      fields[8].innerHTML = ex.currency_pair

      _this.exchangeTableTarget.appendChild(exRow)
    })
  }

  // exchange chart
  plotGraph (exs) {
    var data = []
    var dataSet = []

    const _this = this
    exs.forEach(ex => {
      data.push(new Date(ex.time))
      data.push(ex.filter)

      dataSet.push(data)
      data = []
    })

    _this.labels = ['Date', _this.selectedFilter]
    let colors = ['#007bff']

    var extra = {
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: 'Price',
      xlabel: 'Date',
      labels: _this.labels,
      colors: colors,
      digitsAfterDecimal: 8
    }

    _this.chartsView = new Dygraph(
      _this.chartsViewTarget,
      dataSet, { ...options, ...extra }
    )
  }

  drawInitialGraph () {
    var extra = {
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: 'Price',
      xlabel: 'Date',
      labels: this.labels,
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
