import { Controller } from 'stimulus'
import axios from 'axios'
import { legendFormatter, barChartPlotter, hide, show, setActiveOptionBtn, options, showLoading, hideLoading } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'nextPageButton', 'previousPageButton', 'tableBody', 'rowTemplate',
      'totalPageCount', 'currentPage', 'btnWrapper', 'tableWrapper', 'chartsView',
      'chartWrapper', 'viewOption', 'labels', 'viewOptionControl', 'messageView',
      'chartDataTypeSelector', 'chartDataType', 'chartOptions', 'labels', 'selectedMempoolOpt',
      'selectedNumberOfRows', 'numPageWrapper', 'loadingData'
    ]
  }

  initialize () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.dataType = this.chartDataTypeTarget.getAttribute('data-initial-value')

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
    hide(this.messageViewTarget)
    hide(this.chartDataTypeSelectorTarget)
    show(this.tableWrapperTarget)
    show(this.numPageWrapperTarget)
    show(this.btnWrapperTarget)
    this.nextPage = this.currentPage
    this.fetchData(this.selectedViewOption)
  }

  setChart () {
    this.selectedViewOption = 'chart'
    hide(this.btnWrapperTarget)
    hide(this.tableWrapperTarget)
    hide(this.messageViewTarget)
    this.chartFilter = this.selectedMempoolOptTarget.value = this.selectedMempoolOptTarget.options[0].value
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    setActiveOptionBtn(this.dataType, this.chartDataTypeTargets)
    show(this.chartDataTypeSelectorTarget)
    hide(this.numPageWrapperTarget)
    show(this.chartWrapperTarget)
    this.fetchData(this.selectedViewOption)
  }

  mempoolOptionChanged () {
    this.chartFilter = this.selectedMempoolOptTarget.value
    this.fetchData(this.selectedViewOption)
  }

  setDataType (event) {
    this.dataType = event.currentTarget.getAttribute('data-option')
    setActiveOptionBtn(this.dataType, this.chartDataTypeTargets)
    this.fetchData('chart')
  }

  numberOfRowsChanged () {
    this.selectedNumberOfRowsberOfRows = this.selectedNumberOfRowsTarget.value
    this.fetchData(this.selectedViewOption)
  }

  loadPreviousPage () {
    this.nextPage = this.currentPage - 1
    this.fetchData(this.selectedViewOption)
  }

  loadNextPage () {
    this.nextPage = this.currentPage + 1
    this.fetchData(this.selectedViewOption)
  }

  fetchData (display) {
    let url
    let elementsToToggle = [this.tableWrapperTarget, this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    if (display === 'table') {
      this.selectedNumberOfRowsberOfRows = this.selectedNumberOfRowsTarget.value
      url = `/getmempool?page=${this.nextPage}&records-per-page=${this.selectedNumberOfRowsberOfRows}&view-option=${this.selectedViewOption}`
    } else {
      url = `/mempoolcharts?chart-data-type=${this.dataType}&view-option=${this.selectedViewOption}`
    }

    const _this = this
    axios.get(url).then(function (response) {
      let result = response.data
      console.log(result)
      if (display === 'table' && result.message) {
        hideLoading(_this.loadingDataTarget, [_this.tableWrapperTarget])
        let messageHTML = ''
        messageHTML += `<div class="alert alert-primary">
                       <strong>${result.message}</strong>
                  </div>`

        _this.messageViewTarget.innerHTML = messageHTML
        show(_this.messageViewTarget)
        hide(_this.tableBodyTarget)
        window.history.pushState(window.history.state, _this.addr, `/mempool?page=${_this.nextPage}&records-per-page=${_this.selectedNumberOfRowsberOfRows}&view-option=${_this.selectedViewOption}`)
      } else if (display === 'table' && result.mempoolData) {
        hideLoading(_this.loadingDataTarget, [_this.tableWrapperTarget])
        hide(_this.messageViewTarget)
        show(_this.tableBodyTarget)
        _this.totalPageCountTarget.textContent = result.totalPages
        _this.currentPageTarget.textContent = result.currentPage
        let url = `/mempool?page=${result.currentPage}&records-per-page=${result.selectedNumberOfRows}&view-option=${_this.selectedViewOption}`
        window.history.pushState(window.history.state, _this.addr, url)

        _this.currentPage = result.currentPage
        if (_this.currentPage <= 1) {
          _this.currentPage = result.currentPage
          hide(_this.previousPageButtonTarget)
        } else {
          show(_this.previousPageButtonTarget)
        }

        if (_this.currentPage >= result.totalPages) {
          hide(_this.nextPageButtonTarget)
        } else {
          show(_this.nextPageButtonTarget)
        }

        _this.displayMempool(result.mempoolData)
      } else {
        hideLoading(_this.loadingDataTarget, [_this.chartWrapperTarget])
        let url = `/mempool?chart-data-type=${_this.dataType}&view-option=${_this.selectedViewOption}`
        window.history.pushState(window.history.state, _this.addr, url)
        _this.plotGraph(result)
      }
    }).catch(function (e) {
      console.log(e) // todo: handle error
    })
  }

  displayMempool (data) {
    const _this = this
    this.tableBodyTarget.innerHTML = ''

    data.forEach(item => {
      const exRow = document.importNode(_this.rowTemplateTarget.content, true)
      const fields = exRow.querySelectorAll('td')

      fields[0].innerText = item.time
      fields[1].innerText = item.number_of_transactions
      fields[2].innerText = item.size
      fields[3].innerHTML = item.total_fee.toFixed(8)

      _this.tableBodyTarget.appendChild(exRow)
    })
  }

  // exchange chart
  plotGraph (exs) {
    const _this = this
    console.log(exs)
    if (exs.error) {
      this.drawInitialGraph()
    } else {
      let chartData = exs.mempoolchartData
      let csv = ''
      switch (this.dataType) {
        case 'size':
          this.title = 'Size'
          csv = 'Date,Size\n'
          break
        case 'total_fee':
          this.title = 'Total Fee'
          csv = 'Date,Total Fee\n'
          break
        default:
          this.title = '# of Transactions'
          csv = 'Date,# of Transactions\n'
          break
      }
      let minDate, maxDate

      chartData.forEach(mp => {
        let date = new Date(mp.time)
        if (minDate === undefined || new Date(mp.time) < minDate) {
          minDate = new Date(mp.time)
        }

        if (maxDate === undefined || new Date(mp.time) > maxDate) {
          maxDate = new Date(mp.time)
        }

        let record
        if (_this.dataType === 'size') {
          record = mp.size
        } else if (_this.dataType === 'total_fee') {
          record = mp.total_fee
        } else {
          record = mp.number_of_transactions
        }
        csv += `${date},${record}\n`
      })

      _this.chartsView = new Dygraph(
        _this.chartsViewTarget,
        csv,
        {
          legend: 'always',
          includeZero: true,
          dateWindow: [minDate, maxDate],
          legendFormatter: legendFormatter,
          plotter: barChartPlotter,
          digitsAfterDecimal: 8,
          labelsDiv: _this.labelsTarget,
          ylabel: _this.title,
          xlabel: 'Date',
          labelsUTC: true,
          labelsKMB: true,
          maxNumberWidth: 10,
          showRangeSelector: true,
          axes: {
            x: {
              drawGrid: false
            },
            y: {
              axisLabelWidth: 90
            }
          }
        }
      )
    }
  }

  drawInitialGraph () {
    var extra = {
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: this.title,
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
