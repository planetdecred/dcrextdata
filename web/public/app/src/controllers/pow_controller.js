import { Controller } from 'stimulus'
import axios from 'axios'
import { hide, show, legendFormatter, setActiveOptionBtn } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'powFilterWrapper', 'selectedFilter', 'powTable', 'numPageWrapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton', 'viewOptionControl',
      'powRowTemplate', 'currentPage', 'selectedNum', 'powTableWrapper', 'loadingData',
      'chartSourceWrapper', 'pool', 'chartWrapper', 'chartDataTypeSelector', 'dataType', 'labels',
      'chartsView', 'viewOption', 'pageSizeWrapper'
    ]
  }

  initialize () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }
    this.dataType = 'pool_hashrate'

    this.selectedViewOption = this.viewOptionControlTarget.getAttribute('data-initial-value')
    console.log(this.selectedViewOption)
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
    show(this.powFilterWrapperTarget)
    show(this.powTableWrapperTarget)
    show(this.numPageWrapperTarget)
    show(this.pageSizeWrapperTarget)
    hide(this.chartDataTypeSelectorTarget)
    this.nextPage = this.currentPage
    this.fetchData()
  }

  setChart () {
    this.selectedViewOption = 'chart'
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    hide(this.numPageWrapperTarget)
    hide(this.powFilterWrapperTarget)
    hide(this.powTableWrapperTarget)
    show(this.chartSourceWrapperTarget)
    show(this.chartWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    show(this.chartDataTypeSelectorTarget)
    this.fetchDataAndPlotGraph()
  }

  poolCheckChanged (event) {
    this.fetchDataAndPlotGraph()
  }

  selectedFilterChanged () {
    this.nextPage = 1
    this.fetchData()
  }

  loadPreviousPage () {
    this.nextPage = this.currentPage - 1
    this.fetchData()
  }

  loadNextPage () {
    this.nextPage = this.currentPage + 1
    this.fetchData()
  }

  numberOfRowsChanged () {
    this.nextPage = 1
    this.fetchData()
  }

  fetchData () {
    const selectedFilter = this.selectedFilterTarget.value
    var numberOfRows = this.selectedNumTarget.value

    this.showLoading()

    const _this = this
    axios.get(`/filteredpow?page=${this.nextPage}&filter=${selectedFilter}&recordsPerPage=${numberOfRows}&viewOption=${_this.selectedViewOption}`)
      .then(function (response) {
        let result = response.data
        window.history.pushState(window.history.state, _this.addr, `pow?page=${result.currentPage}&filter=${selectedFilter}&recordsPerPage=${result.selectedNum}&viewOption=${_this.selectedViewOption}`)

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

        show(_this.powTableWrapperTarget)
        _this.displayPoW(result.powData)
        _this.hideLoading()
      }).catch(function (e) {
        _this.hideLoading()
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
    this.fetchDataAndPlotGraph()
  }

  fetchDataAndPlotGraph () {
    let selectedPools = []
    this.poolTargets.forEach(el => {
      if (el.checked) {
        selectedPools.push(el.value)
      }
    })
    this.showLoading()
    const _this = this
    const url = `/powchart?pools=${selectedPools.join('|')}&datatype=${this.dataType}&viewOption=${_this.selectedViewOption}`
    window.history.pushState(window.history.state, _this.addr, url + `&refresh=${1}`)
    axios.get(url).then(function (response) {
      let result = response.data
      if (result.error) {
        console.log(result.error) // todo show error page fron front page
        return
      }

      show(_this.chartWrapperTarget)

      _this.plotGraph(result)
      _this.hideLoading()
    }).catch(function (e) {
      _this.hideLoading()
      console.log(e)
    })
  }

  // vsp chart
  plotGraph (dataSet) {
    const _this = this
    let dataTypeLabel = 'Pool Hashrate (Th/s)'
    if (_this.dataType === 'workers') {
      dataTypeLabel = 'Workers'
    }

    let options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: legendFormatter,
      labelsDiv: _this.labelsTarget,
      ylabel: dataTypeLabel,
      xlabel: 'Date',
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

    _this.chartsView = new Dygraph(_this.chartsViewTarget, dataSet.csv, options)
  }

  showLoading () {
    hide(this.powTableWrapperTarget)
    hide(this.chartWrapperTarget)
    hide(this.messageViewTarget)
    show(this.loadingDataTarget)
  }

  hideLoading () {
    hide(this.loadingDataTarget)
  }
}
