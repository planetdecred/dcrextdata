import { Controller } from 'stimulus'
import axios from 'axios'
import { hide, show, legendFormatter, setActiveOptionBtn } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'selectedFilter', 'vspTicksTable', 'numPageWrapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton',
      'vspRowTemplate', 'currentPage', 'selectedNum', 'vspTableWrapper',
      'graphTypeWrapper', 'dataType', 'pageSizeWrapper', 'viewOptionControl',
      'vspSelectorWrapper', 'chartSourceWrapper', 'chartSource',
      'chartWrapper', 'labels', 'chartsView', 'viewOption'
    ]
  }

  initialize () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.vsps = []
    this.chartSourceTargets.forEach(chartSource => {
      if (chartSource.checked) {
        this.vsps.push(chartSource.value)
      }
    })

    this.dataType = this.dataTypeTarget.value = this.dataTypeTarget.getAttribute('data-initial-value')

    this.vsps = []
    // if no vsp is selected, select the first one
    let noVspSelected = true
    this.chartSourceTargets.forEach(el => {
      if (el.checked) {
        noVspSelected = false
        this.vsps.push(el.value)
      }
    })
    if (noVspSelected) {
      this.chartSourceTarget.checked = true
      this.vsps.push(this.chartSourceTarget.value)
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
    hide(this.graphTypeWrapperTarget)
    hide(this.chartSourceWrapperTarget)
    show(this.vspTableWrapperTarget)
    show(this.numPageWrapperTarget)
    show(this.pageSizeWrapperTarget)
    show(this.vspSelectorWrapperTarget)
    this.vspTicksTableTarget.innerHTML = ''
    this.nextPage = this.currentPage
    this.fetchData('table')
  }

  setChart () {
    this.selectedViewOption = 'chart'
    hide(this.numPageWrapperTarget)
    hide(this.vspTableWrapperTarget)
    hide(this.vspSelectorWrapperTarget)
    show(this.graphTypeWrapperTarget)
    show(this.chartWrapperTarget)
    show(this.chartSourceWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    this.fetchData('chart')
  }

  selectedFilterChanged () {
    if (this.selectedViewOption === 'table') {
      this.nextPage = 1
      this.fetchData(this.selectedViewOption)
    } else {
      if (this.selectedFilterTarget.selectedIndex === 0) {
        this.selectedFilterTarget.selectedIndex = 1
      }
      this.fetchDataAndPlotGraph()
    }
  }

  loadPreviousPage () {
    this.nextPage = this.currentPage - 1
    this.fetchData(this.selectedViewOption)
  }

  loadNextPage () {
    this.nextPage = this.currentPage + 1
    this.fetchData(this.selectedViewOption)
  }

  numberOfRowsChanged () {
    this.nextPage = 1
    this.fetchData(this.selectedViewOption)
  }

  fetchData (display) {
    const selectedFilter = this.selectedFilterTarget.value
    var numberOfRows
    if (display === 'chart') {
      this.fetchDataAndPlotGraph()
      return
    } else {
      numberOfRows = this.selectedNumTarget.value
    }

    const _this = this
    axios.get(`/vsps?page=${this.nextPage}&filter=${selectedFilter}&records-per-page=${numberOfRows}&view-option=${_this.selectedViewOption}`)
      .then(function (response) {
        let result = response.data

        if (display === 'table') {
          window.history.pushState(window.history.state, _this.addr, `/vsp?page=${result.currentPage}&filter=${selectedFilter}&records-per-page=${result.selectedNum}&view-option=${_this.selectedViewOption}`)
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
        } else {
          _this.plotGraph(result.vspData)
        }
      }).catch(function (e) {
        console.log(e)
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

  chartSourceCheckChanged (event) {
    this.vsps = []
    this.chartSourceTargets.forEach(chartSource => {
      if (chartSource.checked) {
        this.vsps.push(chartSource.value)
      }
    })
    this.fetchDataAndPlotGraph()
  }

  dataTypeChanged () {
    this.dataType = this.dataTypeTarget.value
    this.fetchDataAndPlotGraph()
  }

  fetchDataAndPlotGraph () {
    const _this = this
    if (this.vsps.length === 0) {
      return
    }
    const queryString = `data-type=${this.dataType}&vsps=${this.vsps.join('|')}&view-option=${_this.selectedViewOption}`
    window.history.pushState(window.history.state, _this.addr, `/vsp?${queryString}`)
    axios.get(`/vspchartdata?${queryString}`).then(function (response) {
      _this.plotGraph(response.data)
    }).catch(function (e) {
      _this.drawInitialGraph()
    })
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

    let options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: legendFormatter,
      labelsDiv: _this.labelsTarget,
      ylabel: _this.yLabel,
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
    _this.chartsView = new Dygraph(
      _this.chartsViewTarget,
      dataSet.csv,
      options
    )
  }

  drawInitialGraph () {
    var options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: this.yLabel,
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

    this.chartsView = new Dygraph(
      this.chartsViewTarget,
      [[0, 0]],
      options
    )
  }
}
