import { Controller } from 'stimulus'
import axios from 'axios'
import {
  hide,
  show,
  setActiveOptionBtn,
  showLoading,
  hideLoading,
  displayPillBtnOption,
  setActiveRecordSetBtn,
  legendFormatter, insertOrUpdateQueryParam, updateQueryParam, trimUrl, zipXYZData, selectedOption
} from '../utils'
import dompurify from 'dompurify'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  chartType
  syncSources

  static get targets () {
    return [
      'nextPageButton', 'previousPageButton', 'recordSetSelector', 'bothRecordSetOption',
      'tableRecordSetOptions', 'selectedRecordSet', 'bothRecordWrapper',
      'selectedNum', 'numPageWrapper', 'paginationButtonsWrapper',
      'chartTypesWrapper', 'chartType',
      'tablesWrapper', 'table', 'blocksTbody', 'votesTbody', 'chartWrapper', 'chartsView', 'labels', 'messageView',
      'blocksTable', 'blocksTableBody', 'blocksRowTemplate', 'votesTable', 'votesTableBody', 'votesRowTemplate',
      'totalPageCount', 'currentPage', 'viewOptionControl', 'viewOption', 'loadingData',
      'graphIntervalWrapper', 'interval'
    ]
  }

  initialize () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.selectedViewOption = this.viewOptionControlTarget.dataset.initialValue
    this.selectedRecordSet = this.tableRecordSetOptionsTarget.dataset.initialValue
    this.chartType = this.chartTypesWrapperTarget.dataset.initialValue

    const syncSources = this.chartTypesWrapperTarget.dataset.syncSources
    if (syncSources) {
      this.syncSources = syncSources.split('|')
    } else {
      this.syncSources = []
    }

    setActiveOptionBtn(this.selectedViewOption, this.viewOptionControlTargets)

    if (this.selectedViewOption === 'chart') {
      setActiveOptionBtn(this.chartType, this.chartTypeTargets)
      this.setChart()
    } else {
      setActiveOptionBtn(this.selectedRecordSet, this.selectedRecordSetTargets)
      this.setTable()
    }
  }

  setTable () {
    this.selectedViewOption = 'table'
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    show(this.tableRecordSetOptionsTarget)
    hide(this.chartTypesWrapperTarget)
    hide(this.chartWrapperTarget)
    hide(this.messageViewTarget)
    show(this.paginationButtonsWrapperTarget)
    show(this.numPageWrapperTarget)
    hide(this.chartWrapperTarget)
    hide(this.graphIntervalWrapperTarget)
    show(this.tablesWrapperTarget)
    setActiveRecordSetBtn(this.selectedRecordSet, this.selectedRecordSetTargets)
    displayPillBtnOption(this.selectedViewOption, this.selectedRecordSetTargets)
    this.fetchTableData(this.currentPage)
    insertOrUpdateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['view-option', 'page', 'records-per-page', 'record-set'])
  }

  setChart () {
    this.selectedViewOption = 'chart'
    hide(this.tableRecordSetOptionsTarget)
    show(this.chartTypesWrapperTarget)
    hide(this.numPageWrapperTarget)
    hide(this.messageViewTarget)
    hide(this.paginationButtonsWrapperTarget)
    hide(this.tablesWrapperTarget)
    show(this.chartWrapperTarget)
    show(this.graphIntervalWrapperTarget)
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    setActiveRecordSetBtn(this.selectedRecordSet, this.selectedRecordSetTargets)
    displayPillBtnOption(this.selectedViewOption, this.selectedRecordSetTargets)
    this.plotSelectedChart()
    updateQueryParam('view-option', this.selectedViewOption, 'chart')
    trimUrl(['view-option', 'chart-type'])
    // reset this table properties as they are removed from the url
    this.currentPage = 1
    this.selectedNumberOfRowsberOfRows = 20
    if (this.hasSelectedNumberOfRowsTarget) {
      this.selectedNumberOfRowsTarget.value = this.selectedNumberOfRowsberOfRows
    }
  }

  setBothRecordSet () {
    this.selectedRecordSet = 'both'
    setActiveOptionBtn(this.selectedRecordSet, this.selectedRecordSetTargets)
    this.currentPage = 1
    this.selectedNumTarget.value = this.selectedNumTarget.options[0].text
    if (this.selectedViewOption === 'table') {
      this.fetchTableData(1)
    } else if (this.selectedViewOption === 'chart') {
      this.fetchChartDataAndPlot()
    } else {
      this.fetchChartExtDataAndPlot()
    }

    insertOrUpdateQueryParam('record-set', this.selectedRecordSet, 'both')
  }

  setBlocksRecordSet () {
    this.selectedRecordSet = 'blocks'
    setActiveOptionBtn(this.selectedRecordSet, this.selectedRecordSetTargets)
    this.currentPage = 1
    this.selectedNumTarget.value = this.selectedNumTarget.options[0].text
    if (this.selectedViewOption === 'table') {
      this.fetchTableData(1)
    } else if (this.selectedViewOption === 'chart') {
      this.fetchChartDataAndPlot()
    } else {
      this.fetchChartExtDataAndPlot()
    }

    insertOrUpdateQueryParam('record-set', this.selectedRecordSet, 'both')
  }

  setVotesRecordSet () {
    this.selectedRecordSet = 'votes'
    setActiveOptionBtn(this.selectedRecordSet, this.selectedRecordSetTargets)
    this.currentPage = 1
    this.selectedNumTarget.value = this.selectedNumTarget.options[0].text
    if (this.selectedViewOption === 'table') {
      this.fetchTableData(1)
    } else if (this.selectedViewOption === 'chart') {
      this.fetchChartDataAndPlot()
    } else {
      this.fetchChartExtDataAndPlot()
    }

    insertOrUpdateQueryParam('record-set', this.selectedRecordSet, 'both')
  }

  changeChartType (event) {
    this.chartType = event.currentTarget.dataset.option
    setActiveOptionBtn(this.chartType, this.chartTypeTargets)
    this.plotSelectedChart()
    insertOrUpdateQueryParam('chart-type', this.chartType, 'propagation')
  }

  loadPreviousPage () {
    this.currentPage -= 1
    this.fetchTableData(this.currentPage)
    insertOrUpdateQueryParam('page', this.currentPage, 1)
    insertOrUpdateQueryParam('chart-type', this.chartType, 'block-propagation')
  }

  loadNextPage () {
    this.fetchTableData(this.currentPage + 1)
    insertOrUpdateQueryParam('page', this.currentPage + 1, 1)
  }

  numberOfRowsChanged () {
    this.selectedNum = parseInt(this.selectedNumTarget.value)
    this.currentPage = 1
    this.fetchTableData(this.currentPage)
    insertOrUpdateQueryParam('page', this.currentPage, 1)
    insertOrUpdateQueryParam('records-per-page', this.selectedNum, 20)
  }

  fetchTableData (page) {
    const _this = this

    let elementsToToggle = [this.tablesWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    var numberOfRows = this.selectedNumTarget.value
    let url = '/getpropagationdata'
    switch (this.selectedRecordSet) {
      case 'blocks':
        url = 'getblocks'
        break
      case 'votes':
        url = 'getvotes'
        break
      default:
        url = 'getpropagationdata'
        break
    }
    axios.get(`/${url}?page=${page}&records-per-page=${numberOfRows}&view-option=${_this.selectedViewOption}`).then(function (response) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      let result = response.data
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

      _this.displayData(result)
    }).catch(function (e) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      console.log(e) // todo: handle error
    })
  }

  displayData (data) {
    switch (this.selectedRecordSet) {
      case 'blocks':
        this.displayBlocks(data)
        break
      case 'votes':
        this.displayVotes(data)
        break
      default:
        this.displayPropagationData(data)
        break
    }
  }

  displayBlocks (data) {
    const _this = this
    this.blocksTableBodyTarget.innerHTML = ''

    if (data.records) {
      hide(this.messageViewTarget)
      show(this.blocksTableBodyTarget)
      show(this.paginationButtonsWrapperTarget)

      data.records.forEach(block => {
        const exRow = document.importNode(_this.blocksRowTemplateTarget.content, true)
        const fields = exRow.querySelectorAll('td')

        fields[0].innerHTML = `<a target="_blank" href="https://explorer.dcrdata.org/block/${block.block_height}">${block.block_height}</a>`
        fields[1].innerText = block.block_internal_time
        fields[2].innerText = block.block_receive_time
        fields[3].innerText = block.delay
        fields[4].innerHTML = `<a target="_blank" href="https://explorer.dcrdata.org/block/${block.block_height}">${block.block_hash}</a>`

        _this.blocksTableBodyTarget.appendChild(exRow)
      })
    } else {
      let messageHTML = ''
      messageHTML += `<div class="alert alert-primary">
                        <strong>${data.message}</strong>
                      </div>`
      this.messageViewTarget.innerHTML = messageHTML
      show(this.messageViewTarget)
      hide(this.blocksTableBodyTarget)
      hide(this.paginationButtonsWrapperTarget)
    }

    hide(this.tableTarget)
    hide(this.votesTableTarget)
    show(this.blocksTableTarget)
  }

  displayVotes (data) {
    const _this = this
    this.votesTableBodyTarget.innerHTML = ''

    if (data.voteRecords) {
      hide(this.messageViewTarget)
      show(this.votesTableBodyTarget)
      show(this.paginationButtonsWrapperTarget)

      data.voteRecords.forEach(item => {
        const exRow = document.importNode(_this.votesRowTemplateTarget.content, true)
        const fields = exRow.querySelectorAll('td')

        fields[0].innerHTML = `<a target="_blank" href="https://explorer.dcrdata.org/block/${item.voting_on}">${item.voting_on}</a>`
        fields[1].innerHTML = `<a target="_blank" href="https://explorer.dcrdata.org/block/${item.block_hash}">...${item.short_block_hash}</a>`
        fields[2].innerText = item.validator_id
        fields[3].innerText = item.validity
        fields[4].innerText = item.receive_time
        fields[5].innerText = item.block_time_diff
        fields[6].innerText = item.block_receive_time_diff
        fields[7].innerHTML = `<a target="_blank" href="https://explorer.dcrdata.org/tx/${item.hash}">${item.hash}</a>`

        _this.votesTableBodyTarget.appendChild(exRow)
      })
    } else {
      let messageHTML = ''
      messageHTML += `<div class="alert alert-primary">
                        <strong>${data.message}</strong>
                      </div>`
      this.messageViewTarget.innerHTML = messageHTML
      show(this.messageViewTarget)
      hide(this.votesTableBodyTarget)
      hide(this.paginationButtonsWrapperTarget)
    }

    hide(this.tableTarget)
    hide(this.blocksTableTarget)
    show(this.votesTableTarget)
  }

  displayPropagationData (data) {
    let blocksHtml = ''
    if (data.records) {
      hide(this.messageViewTarget)
      show(this.tableTarget)
      show(this.paginationButtonsWrapperTarget)

      data.records.forEach(block => {
        let votesHtml = ''
        let i = 0
        if (block.votes) {
          block.votes.forEach(vote => {
            votesHtml += `<tr>
                              <td><a target="_blank" href="https://explorer.dcrdata.org/block/${vote.voting_on}">${vote.voting_on}</a></td>
                              <td><a target="_blank" href="https://explorer.dcrdata.org/block/${vote.block_hash}">...${vote.short_block_hash}</a></td>
                              <td>${vote.validator_id}</td>
                              <td>${vote.validity}</td>
                              <td>${vote.receive_time}</td>
                              <td>${vote.block_receive_time_diff}s</td>
                              <td><a target="_blank" href="https://explorer.dcrdata.org/tx/${vote.hash}">${vote.hash}</a></td>
                          </tr>`
          })
        }

        let padding = i > 0 ? 'style="padding-top:50px"' : ''
        i++
        blocksHtml += `<tbody data-target="propagation.blockTbody"
                              data-block-hash="${block.block_hash}">
                          <tr>
                              <td colspan="100" ${padding}>
                                <span class="d-inline-block"><b>Height</b>: ${block.block_height} </span>  &#8195;
                                <span class="d-inline-block"><b>Timestamp</b>: ${block.block_internal_time}</span>  &#8195;
                                <span class="d-inline-block"><b>Received</b>: ${block.block_receive_time}</span>  &#8195;
                                <span class="d-inline-block"><b>Hash</b>: <a target="_blank" href="https://explorer.dcrdata.org/block/${block.block_height}">${block.block_hash}</a></span>
                              </td>
                          </tr>
                          </tbody>
                          <tbody data-target="propagation.votesTbody" data-block-hash="${block.block_hash}">
                          <tr style="white-space: nowrap;">
                              <td style="width: 120px;">Voting On</td>
                              <td style="width: 120px;">Block Hash</td>
                              <td style="width: 120px;">Validator ID</td>
                              <td style="width: 120px;">Validity</td>
                              <td style="width: 120px;">Received</td>
                              <td style="width: 120px;">Block Receive Time Diff</td>
                              <td style="width: 120px;">Hash</td>
                          </tr>
                          ${votesHtml}
                          </tbody>
                            <tr>
                                <td colspan="7" height="15" style="border: none !important;"></td>
                            </tr>`
      })

      this.tableTarget.innerHTML = blocksHtml
    } else {
      let messageHTML = ''
      messageHTML += `<div class="alert alert-primary">
                        <strong>${data.message}</strong>
                      </div>`
      this.messageViewTarget.innerHTML = messageHTML

      show(this.messageViewTarget)
      hide(this.tableTarget)
      hide(this.paginationButtonsWrapperTarget)
    }

    show(this.tableTarget)
    hide(this.blocksTableTarget)
    hide(this.votesTableTarget)
  }

  plotSelectedChart () {
    switch (this.chartType) {
      default:
        this.fetchChartExtDataAndPlot()
        break
      case 'block-timestamp':
      case 'votes-receive-time':
        this.fetchChartDataAndPlot()
        break
    }
  }

  fetchChartDataAndPlot () {
    let elementsToToggle = [this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this
    axios.get(`/api/charts/propagation/${this.chartType}?bin=${this.selectedInterval()}`).then(function (response) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      _this.plotGraph(response.data)
    }).catch(function (e) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      console.log(e) // todo: handle error
    })
  }

  fetchChartExtDataAndPlot () {
    if (!this.syncSources || this.syncSources.length === 0) {
      const message = 'Add one or more sync sources to the configuration file to view propagation chart'
      this.messageViewTarget.innerHTML = `<p class="text-danger" style="text-align: center;">${message}</p>`
      show(this.messageViewTarget)
      hide(this.chartWrapperTarget)
      hideLoading(this.loadingDataTarget, [])
      return
    }

    let elementsToToggle = [this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this
    const url = `/api/charts/propagation/${this.chartType}?extras=${this.syncSources.join('|')}&bin=${this.selectedInterval()}`
    axios.get(url).then(function (response) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      if (!response.data.x || response.data.x.length === 0) {
        _this.messageViewTarget.innerHTML = `<p class="text-danger" style="text-align: center;">
            No propagation data found, please add one sync source to the configuration and try again</p>`
        show(_this.messageViewTarget)
        hide(_this.chartWrapperTarget)
      } else {
        hide(_this.messageViewTarget)
        show(_this.chartWrapperTarget)
        _this.plotExtDataGraph(response.data)
      }
    }).catch(function (e) {
      hideLoading(_this.loadingDataTarget, elementsToToggle)
      console.log(e) // todo: handle error
    })
  }

  plotGraph (data) {
    const _this = this

    let yLabel = this.chartType === 'votes-receive-time' ? 'Time Difference (Milliseconds)' : 'Delay (s)'
    let options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: _this.propagationLegendFormatter,
      labelsDiv: _this.labelsTarget,
      ylabel: yLabel,
      xlabel: 'Height',
      labels: ['Height', yLabel],
      labelsKMB: true,
      drawPoints: true,
      strokeWidth: 0.0,
      showRangeSelector: true
    }

    const chartData = zipXYZData(data, true)

    _this.chartsView = new Dygraph(_this.chartsViewTarget, chartData, options)
  }

  plotExtDataGraph (data) {
    const _this = this

    const labels = ['Height']
    this.syncSources.forEach(source => {
      labels.push(source)
    })
    let options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: legendFormatter,
      labelsDiv: _this.labelsTarget,
      ylabel: 'Block Time Variance (seconds)',
      xlabel: 'Height',
      labels: labels,
      labelsKMB: true,
      drawPoints: true,
      strokeWidth: 0.0,
      showRangeSelector: true,
      axes: {
        x: {
          drawGrid: false
        }
      }
    }

    const chartData = zipXYZData(data, true)

    _this.chartsView = new Dygraph(_this.chartsViewTarget, chartData, options)
  }

  propagationLegendFormatter (data) {
    let html = ''
    const votesDescription = '&nbsp;&nbsp;&nbsp;&nbsp;Measured as the difference between the blocks timestamp and the time the block was received by this node.'
    const blocksDescription = '&nbsp;&nbsp;&nbsp;&nbsp;Showing the difference in time between the block and the votes.'
    let descriptionText = this.selectedRecordSet === 'votes' ? votesDescription : blocksDescription
    if (data.x == null) {
      let dashLabels = data.series.reduce((nodes, series) => {
        return `${nodes} <div class="pr-2">${series.dashHTML} ${series.labelHTML} ${descriptionText}</div>`
      }, '')
      html = `<div class="d-flex flex-wrap justify-content-center align-items-center" style="text-align: center !important;">
              <div class="pr-3">${this.getLabels()[0]}: N/A</div>
              <div class="d-flex flex-wrap">${dashLabels}</div>
            </div>`
    } else {
      data.series.sort((a, b) => a.y > b.y ? -1 : 1)

      let yVals = data.series.reduce((nodes, series) => {
        if (!series.isVisible) return nodes
        let yVal = series.yHTML
        yVal = series.y

        if (yVal === undefined) {
          yVal = 'N/A'
        }
        return `${nodes} <div class="pr-2">${series.dashHTML} ${series.labelHTML}: ${yVal} ${descriptionText}</div>`
      }, '')

      html = `<div class="d-flex flex-wrap justify-content-center align-items-center">
                <div class="pr-3">${this.getLabels()[0]}: ${data.xHTML}</div>
                <div class="d-flex flex-wrap"> ${yVals}</div>
            </div>`
    }

    dompurify.sanitize(html)
    return html
  }

  selectedInterval () { return selectedOption(this.intervalTargets) }

  setInterval (e) {
    const option = e.currentTarget.dataset.option
    setActiveOptionBtn(option, this.intervalTargets)
    this.plotSelectedChart()
  }
}
