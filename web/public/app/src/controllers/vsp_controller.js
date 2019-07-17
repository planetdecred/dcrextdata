import { Controller } from 'stimulus'
import axios from 'axios'
import { hide, show, date, legendFormatter, options, getRandomColor } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')
var opt = 'table'

export default class extends Controller {
  static get targets () {
    return [
      'selectedFilterWrapper', 'selectedFilter', 'vspTicksTable', 'numPageWrapper',
      'previousPageButton', 'totalPageCount', 'nextPageButton',
      'vspRowTemplate', 'currentPage', 'selectedNum', 'vspTableWrapper',
      'graphTypeWrapper', 'graphType', 'chartSourceWrapper', 'chartSource',
      'chartWrapper', 'labels', 'chartsView', 'viewOption'
    ]
  }

  setTable () {
    opt = 'table'
    this.setActiveOptionBtn(opt, this.viewOptionTargets)
    hide(this.chartWrapperTarget)
    hide(this.graphTypeWrapperTarget)
    hide(this.chartSourceWrapperTarget)
    show(this.selectedFilterWrapperTarget)
    show(this.vspTableWrapperTarget)
    show(this.numPageWrapperTarget)
    this.vspTicksTableTarget.innerHTML = ''
    this.nextPage = 1
    this.fetchExchange('table')
  }

  setChart () {
    opt = 'chart'
    hide(this.numPageWrapperTarget)
    hide(this.vspTableWrapperTarget)
    hide(this.selectedFilterWrapperTarget)
    show(this.graphTypeWrapperTarget)
    show(this.chartWrapperTarget)
    show(this.chartSourceWrapperTarget)
    this.setActiveOptionBtn(opt, this.viewOptionTargets)
    this.nextPage = 1
    this.fetchExchange('chart')
  }

  loadPreviousPage () {
    this.nextPage = this.previousPageButtonTarget.getAttribute('data-next-page')
    if (this.nextPage <= 1) {
      hide(this.previousPageButtonTarget)
    }
    this.vspTicksTableTarget.innerHTML = ''
    this.fetchExchange(opt)
  }

  loadNextPage () {
    this.nextPage = this.nextPageButtonTarget.getAttribute('data-next-page')
    this.totalPages = (this.nextPageButtonTarget.getAttribute('data-total-page'))
    if (this.nextPage > 1) {
      show(this.previousPageButtonTarget)
    }
    if (this.totalPages === this.nextPage) {
      hide(this.nextPageButtonTarget)
    }
    this.vspTicksTableTarget.innerHTML = ''
    this.fetchExchange(opt)
  }

  selectedFilterChanged () {
    this.vspTicksTableTarget.innerHTML = ''
    this.nextPage = 1
    this.fetchExchange(opt)
  }

  numberOfRowsChanged () {
    this.vspTicksTableTarget.innerHTML = ''
    this.nextPage = 1
    this.fetchExchange(opt)
  }

  fetchExchange (display) {
    const selectedFilter = this.selectedFilterTarget.value
    var numberOfRows
    if (display === 'chart') {
      this.fetchDataAndGraph()
      return
    } else {
      numberOfRows = this.selectedNumTarget.value
    }

    const _this = this
    axios.get(`/filteredvspticks?page=${this.nextPage}&filter=${selectedFilter}&recordsPerPage=${numberOfRows}`)
      .then(function (response) {
      // since results are appended to the table, discard this response
      // if the user has changed the filter before the result is gotten
        if (_this.selectedFilterTarget.value !== selectedFilter) {
          return
        }

        let result = response.data
        _this.totalPageCountTarget.textContent = result.totalPages
        _this.currentPageTarget.textContent = result.currentPage
        _this.previousPageButtonTarget.setAttribute('data-next-page', `${result.previousPage}`)
        _this.nextPageButtonTarget.setAttribute('data-next-page', `${result.nextPage}`)
        _this.nextPageButtonTarget.setAttribute('data-total-page', `${result.totalPages}`)

        if (display === 'table') {
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
      fields[10].innerHTML = date(vsp.time)

      _this.vspTicksTableTarget.appendChild(vspRow)
    })
  }

  fetchDataAndGraph () {
    let vspSources = []
    this.chartSourceTargets.forEach(el => {
      if (el.checked) {
        vspSources.push(el.value)
      }
    })

    if (vspSources.length === 0) {
      return
    }

    let _this = this
    const selectedAttribute = this.graphTypeTarget.value
    let url = `/vspchartdata?selectedAttribute=${selectedAttribute}&sources=${vspSources.join('|')}`
    axios.get(url).then(function (response) {
      _this.plotGraph(response.data)
    })
  }

  graphTypeChanged () {
    this.fetchDataAndGraph()
  }

  chartSourceCheckChanged () {
    this.fetchDataAndGraph()
  }
  // vsp chart
  plotGraph (dataSet) {
    dataSet = Object.values(dataSet)
    for (let i = 0; i < dataSet.length; i++) {
      if (!Array.isArray(dataSet[i])) continue
      for (let j = 0; j < dataSet[i].length; j++) {
        if (j === 0) {
          dataSet[i][j] = new Date(dataSet[i][j])
        } else if (!isNaN(dataSet[i][j])) {
          dataSet[i][j] = parseFloat(dataSet[i][j])
        }
      }
    }

    let labels = ['Date']
    let colors = []
    this.chartSourceTargets.forEach(el => {
      if (!el.checked) {
        return
      }
      labels.push(el.value)
      colors.push(getRandomColor())
    })

    let yLabel = this.graphTypeTarget.value.split('_').join(' ')
    var extra = {
      legendFormatter: legendFormatter,
      labelsDiv: this.labelsTarget,
      ylabel: yLabel,
      sigFigs: 1,
      labels: labels,
      colors: colors
    }

    const _this = this

    /* var data = []
    var dataSet = []
    vsps.forEach(vsp => {
      data.push(new Date(vsp.time))
      // data.push(vsp.pool_fees)
      // data.push(vsp.immature)
      // data.push(vsp.voted)
      // data.push(vsp.missed)
      data.push(vsp.proportion_live)
      data.push(vsp.proportion_missed)

      dataSet.push(data)
      data = []
    }) */

    _this.chartsView = new Dygraph(
      _this.chartsViewTarget,
      dataSet, { ...options, ...extra }
    )
  }

  setActiveOptionBtn (opt, optTargets) {
    optTargets.forEach(li => {
      if (li.dataset.option === opt) {
        li.classList.add('active')
      } else {
        li.classList.remove('active')
      }
    })
  }
}
