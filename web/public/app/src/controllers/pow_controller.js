import { Controller } from 'stimulus'
import axios from 'axios'
import { hide, show, legendFormatter, setActiveOptionBtn, showLoading, hideLoading } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'powFilterWrapper', 'selectedFilter', 'powTable', 'numPageWrapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton', 'viewOptionControl',
      'powRowTemplate', 'currentPage', 'selectedNum', 'powTableWrapper', 'messageView',
      'chartSourceWrapper', 'pool', 'chartWrapper', 'chartDataTypeSelector', 'dataType', 'labels',
      'chartsView', 'viewOption', 'pageSizeWrapper', 'poolDiv', 'loadingData'
    ]
  }

  initialize () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

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
    show(this.powFilterWrapperTarget)
    hide(this.messageViewTarget)
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
    hide(this.messageViewTarget)
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
    const numberOfRows = this.selectedNumTarget.value

    let elementsToToggle = [this.powTableWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this
    axios.get(`/filteredpow?page=${_this.nextPage}&filter=${selectedFilter}&records-per-page=${numberOfRows}&view-option=${_this.selectedViewOption}`)
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
          window.history.pushState(window.history.state, _this.addr, `/pow?page=${_this.nextPage}&filter=${selectedFilter}&records-per-page=${numberOfRows}&view-option=${_this.selectedViewOption}`)
        } else {
          show(_this.powTableTarget)
          show(_this.pageSizeWrapperTarget)
          hide(_this.messageViewTarget)
          const pageUrl = `/pow?page=${result.currentPage}&filter=${selectedFilter}&records-per-page=${result.selectedNum}&view-option=${_this.selectedViewOption}`
          window.history.pushState(window.history.state, _this.addr, pageUrl)

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
  }

  fetchDataAndPlotGraph () {
    let selectedPools = []
    this.poolTargets.forEach(el => {
      if (el.checked) {
        selectedPools.push(el.value)
      }
    })

    let elementsToToggle = [this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this
    const queryString = `data-type=${this.dataType}&pools=${selectedPools.join('|')}&view-option=${_this.selectedViewOption}`
    window.history.pushState(window.history.state, _this.addr, `/pow?${queryString}`)

    axios.get(`/powchart?${queryString}`).then(function (response) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      let result = response.data
      if (result.error) {
        console.log(result.error) // todo show error page from front page
        return
      }

      _this.plotGraph(result)
    }).catch(function (e) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
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
}
