import { Controller } from 'stimulus'
import axios from 'axios'
import { legendFormatter, barChartPlotter, hide, show, setActiveOptionBtn } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'nextPageButton', 'previousPageButton', 'tableBody', 'rowTemplate',
      'totalPageCount', 'currentPage', 'btnWrapper', 'tableWrapper', 'chartsView',
      'chartWrapper', 'viewOption', 'labels',
      'chartDataTypeSelector', 'chartDataType', 'chartOptions', 'labels', 'selectedMempoolOpt',
      'selectedNum', 'numPageWrapper'
    ]
  }

  connect () {
    var num = this.selectedNumTarget.options
    this.selectedNumTarget.value = num[0].text
  }

  initialize () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }
    this.dataType = 'size'
    this.viewOption = 'table'
  }

  setTable () {
    this.viewOption = 'table'
    setActiveOptionBtn(this.viewOption, this.viewOptionTargets)
    hide(this.chartWrapperTarget)
    hide(this.chartDataTypeSelectorTarget)
    show(this.tableWrapperTarget)
    show(this.numPageWrapperTarget)
    show(this.btnWrapperTarget)
    this.currentPage = this.currentPage
    this.fetchData(this.viewOption)
  }

  setChart () {
    this.viewOption = 'chart'
    hide(this.btnWrapperTarget)
    hide(this.tableWrapperTarget)
    var y = this.selectedMempoolOptTarget.options
    this.chartFilter = this.selectedMempoolOptTarget.value = y[0].value
    setActiveOptionBtn(this.viewOption, this.viewOptionTargets)
    show(this.chartDataTypeSelectorTarget)
    hide(this.numPageWrapperTarget)
    show(this.chartWrapperTarget)
    this.nextPage = 1
    this.fetchData(this.viewOption)
  }

  MempoolOptionChanged () {
    this.chartFilter = this.selectedMempoolOptTarget.value
    this.fetchData(this.viewOption)
  }
  setSizeDataType (event) {
    this.dataType = 'size'
    this.chartDataTypeTargets.forEach(el => {
      el.classList.remove('active')
    })
    event.currentTarget.classList.add('active')
    this.fetchData('chart')
  }

  setFeesDataType (event) {
    this.dataType = 'total_fee'
    this.chartDataTypeTargets.forEach(el => {
      el.classList.remove('active')
    })
    event.currentTarget.classList.add('active')
    this.fetchData('chart')
  }

  setTransactionsDataType (event) {
    this.dataType = 'number_of_transactions'
    this.chartDataTypeTargets.forEach(el => {
      el.classList.remove('active')
    })
    event.currentTarget.classList.add('active')
    this.fetchData('chart')
  }

  gotoPreviousPage () {
    this.currentPage = this.currentPage - 1
    this.fetchData(this.viewOption)
  }

  gotoNextPage () {
    this.currentPage = this.currentPage + 1
    this.fetchData(this.viewOption)
  }

  NumberOfRowsChanged () {
    this.selectedNum = this.selectedNumTarget.value
    this.fetchData(this.viewOption)
  }

  fetchData (display) {
    var url
    if (display === 'table') {
      var numberOfRows = this.selectedNumTarget.value
      url = `/getmempool?page=${this.currentPage}&recordsPerPage=${numberOfRows}`
    } else {
      url = `/getmempoolCharts?chartFilter=${this.dataType}`
    }

    const _this = this
    axios.get(url).then(function (response) {
      let result = response.data
      if (display === 'table') {
        _this.totalPageCountTarget.textContent = result.totalPages
        _this.currentPageTarget.textContent = result.currentPage

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

        _this.displayMempool(result.mempoolData)
      } else {
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
    let title

    let chartData = exs.mempoolchartData
    let csv = ''
    switch (this.dataType) {
      case 'size':
        title = 'Size'
        csv = 'Date,Size\n'
        break
      case 'total_fee':
        title = 'Total Fee'
        csv = 'Date,Total Fee\n'
        break
      default:
        title = '# of Transactions'
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
        // title: title,
        includeZero: true,
        dateWindow: [minDate, maxDate],
        animatedZooms: true,
        legendFormatter: legendFormatter,
        plotter: barChartPlotter,
        labelsDiv: _this.labelsTarget,
        ylabel: title,
        xlabel: 'Date',
        labelsUTC: true,
        labelsKMB: true,
        maxNumberWidth: 10,
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
