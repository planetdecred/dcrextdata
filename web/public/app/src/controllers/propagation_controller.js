import { Controller } from 'stimulus'
import axios from 'axios'
import { hide, show, setActiveOptionBtn, legendFormatter } from '../utils'

const Dygraph = require('../../../dist/js/dygraphs.min.js')

export default class extends Controller {
  static get targets () {
    return [
      'nextPageButton', 'previousPageButton',
      'bothRecordSetOption', 'selectedRecordSet', 'selectedNum', 'numPageWrapper', 'paginationButtonsWrapper',
      'tablesWrapper', 'table', 'blocksTbody', 'votesTbody', 'chartWrapper', 'chartsView', 'labels',
      'blocksTable', 'blocksTableBody', 'blocksRowTemplate', 'votesTable', 'votesTableBody', 'votesRowTemplate',
      'totalPageCount', 'currentPage', 'viewOptionControl', 'chartSelector', 'viewOption'
    ]
  }

  initialize () {
    this.currentPage = parseInt(this.currentPageTarget.getAttribute('data-current-page'))
    if (this.currentPage < 1) {
      this.currentPage = 1
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
    this.selectedRecordSet = this.selectedRecordSetTarget.value = this.selectedRecordSetTarget.options[0].value
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    show(this.selectedRecordSetTarget.options[0])
    this.selectedRecordSet = this.selectedRecordSetTarget.value
    hide(this.chartWrapperTarget)
    show(this.bothRecordSetOptionTarget)
    show(this.paginationButtonsWrapperTarget)
    show(this.numPageWrapperTarget)
    hide(this.chartWrapperTarget)
    show(this.tablesWrapperTarget)
    this.fetchData(this.currentPage)
  }

  setChart () {
    this.selectedViewOption = 'chart'
    setActiveOptionBtn(this.selectedViewOption, this.viewOptionTargets)
    hide(this.selectedRecordSetTarget.options[0])
    hide(this.numPageWrapperTarget)
    hide(this.paginationButtonsWrapperTarget)
    hide(this.tablesWrapperTarget)
    show(this.chartWrapperTarget)
    this.selectedRecordSet = this.selectedRecordSetTarget.value = this.selectedRecordSetTarget.options[1].value

    this.fetchChartDataAndPlot()
  }

  selectedRecordSetChanged () {
    this.currentPage = 1
    this.selectedNumTarget.value = this.selectedNumTarget.options[0].text
    this.selectedRecordSet = this.selectedRecordSetTarget.value
    if (this.selectedViewOption === 'table') {
      this.fetchData(1)
    } else {
      this.fetchChartDataAndPlot()
    }
  }

  loadPreviousPage () {
    this.fetchData(this.currentPage - 1)
  }

  loadNextPage () {
    this.fetchData(this.currentPage + 1)
  }

  numberOfRowsChanged () {
    this.selectedRecordSet = this.selectedRecordSetTarget.value
    this.selectedNum = this.selectedNumTarget.value
    this.fetchData(1)
  }

  fetchData (page) {
    const _this = this

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
    axios.get(`/${url}?page=${page}&recordsPerPage=${numberOfRows}&viewOption=${_this.selectedViewOption}`).then(function (response) {
      let result = response.data
      console.log(result)
      _this.totalPageCountTarget.textContent = result.totalPages
      _this.currentPageTarget.textContent = result.currentPage
      window.history.pushState(window.history.state, _this.addr, `${result.url}?page=${result.currentPage}&recordsPerPage=${result.selectedNum}&viewOption=${_this.selectedViewOption}`)

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
      // console.log(e) // todo: handle error
    })
  }

  displayData (data) {
    switch (this.selectedRecordSet) {
      case 'blocks':
        this.displayBlocks(data.records)
        break
      case 'votes':
        this.displayVotes(data.voteRecords)
        break
      default:
        this.displayPropagationData(data.records)
        break
    }
  }

  displayBlocks (data) {
    const _this = this
    this.blocksTableBodyTarget.innerHTML = ''
    data.forEach(block => {
      const exRow = document.importNode(_this.blocksRowTemplateTarget.content, true)
      const fields = exRow.querySelectorAll('td')

      fields[0].innerHTML = `<a target="_blank" href="https://explorer.dcrdata.org/block/${block.block_height}">${block.block_height}</a>`
      fields[1].innerText = block.block_internal_time
      fields[2].innerText = block.block_receive_time
      fields[3].innerText = block.delay
      fields[4].innerHTML = `<a target="_blank" href="https://explorer.dcrdata.org/block/${block.block_height}">${block.block_hash}</a>`

      _this.blocksTableBodyTarget.appendChild(exRow)
    })

    hide(this.tableTarget)
    hide(this.votesTableTarget)
    show(this.blocksTableTarget)
  }

  displayVotes (data) {
    const _this = this
    this.votesTableBodyTarget.innerHTML = ''

    if (data) {
      data.forEach(item => {
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
      this.votesTableBodyTarget.innerHTML = 'No votes to show'
    }

    hide(this.tableTarget)
    hide(this.blocksTableTarget)
    show(this.votesTableTarget)
  }

  displayPropagationData (data) {
    let blocksHtml = ''
    data.forEach(block => {
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
                              <td colspan="7" height="50" style="border: none !important;"></td>
                          </tr>`
    })

    this.tableTarget.innerHTML = blocksHtml

    show(this.tableTarget)
    hide(this.blocksTableTarget)
    hide(this.votesTableTarget)
  }

  fetchChartDataAndPlot () {
    const _this = this
    const url = '/propagationchartdata?recordset=' + this.selectedRecordSet + `&viewOption=${_this.selectedViewOption}`
    window.history.pushState(window.history.state, _this.addr, url + `&refresh=${1}`)

    axios.get(url).then(function (response) {
      _this.plotGraph(response.data)
    }).catch(function (e) {
      console.log(e) // todo: handle error
    })
  }

  plotGraph (csv) {
    const _this = this

    let yLabel = this.selectedRecordSet === 'votes' ? 'Time Difference (s)' : 'Delay (s)'
    let options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: legendFormatter,
      labelsDiv: _this.labelsTarget,
      ylabel: yLabel,
      xlabel: 'Height',
      labelsKMB: true,
      drawPoints: true,
      strokeWidth: 0.0,
      showRangeSelector: true
    }

    _this.chartsView = new Dygraph(_this.chartsViewTarget, csv, options)
  }
}
