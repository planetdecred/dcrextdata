import { Controller } from 'stimulus'
import axios from 'axios'
import { hide, show, legendFormatter } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'selectedFilter', 'vspTicksTable', 'numPageWrapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton',
      'vspRowTemplate', 'currentPage', 'selectedNum', 'vspTableWrapper',
      'graphTypeWrapper', 'graphType', 'pageSizeWrapper',
      'vspSelectorWrapper', 'chartSourceWrapper', 'chartSource',
      'chartWrapper', 'labels', 'chartsView', 'viewOption'
    ]
  }

  connect () {
    var filter = this.selectedFilterTarget.options
    var num = this.selectedNumTarget.options
    this.selectedFilterTarget.value = filter[0].text
    this.selectedNumTarget.value = num[0].text
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
    this.setChart()
  }

  setTable () {
    this.viewOption = 'table'
    this.setActiveOptionBtn(this.viewOption, this.viewOptionTargets)
    hide(this.chartWrapperTarget)
    hide(this.graphTypeWrapperTarget)
    hide(this.chartSourceWrapperTarget)
    show(this.vspTableWrapperTarget)
    show(this.numPageWrapperTarget)
    show(this.pageSizeWrapperTarget)
    show(this.vspSelectorWrapperTarget)
    this.vspTicksTableTarget.innerHTML = ''
    this.nextPage = 1
    this.fetchExchange('table')
  }

  setChart () {
    this.viewOption = 'chart'
    hide(this.numPageWrapperTarget)
    hide(this.vspTableWrapperTarget)
    hide(this.vspSelectorWrapperTarget)
    show(this.graphTypeWrapperTarget)
    show(this.chartWrapperTarget)
    show(this.chartSourceWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    this.setActiveOptionBtn(this.viewOption, this.viewOptionTargets)
    this.nextPage = 1
    if (this.selectedFilterTarget.selectedIndex === 0) {
      this.selectedFilterTarget.selectedIndex = 1
    }
    this.fetchExchange('chart')
  }

  loadPreviousPage () {
    this.nextPage = this.previousPageButtonTarget.getAttribute('data-next-page')
    this.fetchExchange(this.viewOption)
  }

  loadNextPage () {
    this.nextPage = this.nextPageButtonTarget.getAttribute('data-next-page')
    this.totalPages = (this.nextPageButtonTarget.getAttribute('data-total-page'))
    this.fetchExchange(this.viewOption)
  }

  selectedFilterChanged () {
    if (this.viewOption === 'table') {
      this.nextPage = 1
      this.fetchExchange(this.viewOption)
    } else {
      if (this.selectedFilterTarget.selectedIndex === 0) {
        this.selectedFilterTarget.selectedIndex = 1
      }
      this.fetchDataAndPlotGraph()
    }
  }

  numberOfRowsChanged () {
    this.nextPage = 1
    this.fetchExchange(this.viewOption)
  }

  fetchExchange (display) {
    const selectedFilter = this.selectedFilterTarget.value
    var numberOfRows
    if (display === 'chart') {
      this.fetchDataAndPlotGraph()
      return
    } else {
      numberOfRows = this.selectedNumTarget.value
    }

    const _this = this
    axios.get(`/filteredvspticks?page=${this.nextPage}&filter=${selectedFilter}&recordsPerPage=${numberOfRows}`)
      .then(function (response) {
        let result = response.data

        if (display === 'table') {
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
          _this.previousPageButtonTarget.setAttribute('data-next-page', `${result.previousPage}`)
          _this.nextPageButtonTarget.setAttribute('data-next-page', `${result.nextPage}`)

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
    const element = event.currentTarget
    if (this.vsps.length === 0 && !element.checked) {
      element.checked = true
      this.vsps.push(element.value)
    }
    this.fetchDataAndPlotGraph()
  }

  graphTypeChanged () {
    this.fetchDataAndPlotGraph()
  }

  fetchDataAndPlotGraph () {
    if (this.vsps.length === 0) {
      return
    }
    let _this = this
    let url = `/vspchartdata?selectedAttribute=${this.graphTypeTarget.value}&vsps=${this.vsps.join('|')}`
    axios.get(url).then(function (response) {
      _this.plotGraph(response.data)
    })
  }

  // vsp chart
  plotGraph (dataSet) {
    const _this = this
    let yLabel = this.graphTypeTarget.value.split('_').join(' ')
    if ((yLabel.toLowerCase() === 'proportion live' || yLabel.toLowerCase() === 'proportion missed')) {
      yLabel += ' (%)'
    }

    let options = {
      legend: 'always',
      includeZero: true,
      // dateWindow: [dataSet.min_date, dataSet.max_date],
      animatedZooms: true,
      legendFormatter: legendFormatter,
      // plotter: barChartPlotter,
      labelsDiv: _this.labelsTarget,
      ylabel: yLabel,
      xlabel: 'Date',
      labelsUTC: true,
      labelsKMB: true,
      connectSeparatedPoints: true,
      axes: {
        x: {
          drawGrid: false
        }
      }
    }
    switch (this.graphTypeTarget.value) {
      case 'Immature':

        break
    }
    _this.chartsView = new Dygraph(
      _this.chartsViewTarget,
      dataSet.csv,
      options
    )
  }

  setActiveOptionBtn (opt, optTargets) {
    optTargets.forEach(li => {
      if (li.dataset.option === this.viewOption) {
        li.classList.add('active')
      } else {
        li.classList.remove('active')
      }
    })
  }
}
