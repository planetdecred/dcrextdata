import { Controller } from 'stimulus'
import axios from 'axios'
import {
  hide,
  show,
  legendFormatter,
  setActiveOptionBtn,
  showLoading,
  hideLoading,
  formatDate,
  trimUrl,
  insertOrUpdateQueryParam,
  removeUrlParam,
  selectedOption,
  updateZoomSelector,
  zipXYZData
} from '../utils'
import Zoom from '../helpers/zoom_helper'
import { animationFrame } from '../helpers/animation_helper'
import TurboQuery from '../helpers/turbolinks_helper'

const Dygraph = require('../../../dist/js/dygraphs.min.js')
const redditPlatform = 'Reddit'
const twitterPlatform = 'Twitter'
const githubPlatform = 'GitHub'
const youtubePlatform = 'YouTube'

export default class extends Controller {
  viewOption
  platform
  subreddit
  twitterHandle
  repository
  dataType

  static get targets () {
    return [
      'paginationWrapper', 'previousPageButton', 'totalPageCount', 'nextPageButton',
      'currentPage', 'pageSizeWrapper', 'pageSize', 'messageView',
      'viewOptionControl', 'viewOption',
      'chartWrapper', 'chartsView', 'labels', 'tableWrapper', 'loadingData', 'messageView',
      'tableWrapper', 'table', 'rowTemplate', 'tableCol1', 'tableCol2', 'tableCol3',
      'platform', 'subreddit', 'subAccountWrapper', 'dataTypeWrapper', 'dataTypeContainer', 'dataType',
      'twitterHandle', 'repository', 'channel', 'zoomSelector', 'zoomOption'
    ]
  }

  initialize () {
    this.query = new TurboQuery()
    this.settings = TurboQuery.nullTemplate(['zoom', 'dataType'])
    this.query.update(this.settings)
    this.zoomCallback = this._zoomCallback.bind(this)
    this.drawCallback = this._drawCallback.bind(this)
    this.currentPage = parseInt(this.currentPageTarget.dataset.currentPage)
    if (this.currentPage < 1) {
      this.currentPage = 1
    }

    this.pageSize = this.pageSizeTarget.value

    this.platform = this.platformTarget.dataset.initialValue
    if (this.platform === '' && this.platformTarget.options.length > 0) {
      this.platform = this.platformTarget.value = this.platformTarget.options[0].innerText
    }

    this.showCurrentSubAccountWrapper()

    this.subreddit = this.subredditTarget.dataset.initialValue
    if (this.subreddit === '' && this.subredditTarget.options.length > 0) {
      this.subreddit = this.subredditTarget.value = this.subredditTarget.options[0].innerText
    }

    this.twitterHandle = this.twitterHandleTarget.dataset.initialValue
    if (this.twitterHandle === '' && this.twitterHandleTarget.options.length > 0) {
      this.twitterHandle = this.twitterHandleTarget.value = this.twitterHandleTarget.options[0].innerText
    }

    this.repository = this.repositoryTarget.dataset.initialValue
    if (this.repository === '' && this.repositoryTarget.options.length > 0) {
      this.repository = this.repositoryTarget.value = this.repositoryTarget.options[0].innerText
    }

    this.channel = this.channelTarget.dataset.initialValue
    if (this.channel === '' && this.channelTarget.options.length > 0) {
      this.channel = this.channelTarget.value = this.channelTarget.options[0].innerText
    }

    if (this.settings.zoom) {
      setActiveOptionBtn(this.settings.zoom, this.zoomOptionTargets)
    }

    this.viewOption = this.viewOptionControlTarget.dataset.initialValue
    if (this.viewOption === 'chart') {
      this.setChart()
    } else {
      this.setTable()
    }
  }

  setTable () {
    this.viewOption = 'table'
    insertOrUpdateQueryParam('view-option', this.viewOption, 'chart')
    setActiveOptionBtn(this.viewOption, this.viewOptionTargets)
    hide(this.chartWrapperTarget)
    hide(this.messageViewTarget)
    show(this.tableWrapperTarget)
    show(this.pageSizeWrapperTarget)
    show(this.paginationWrapperTarget)
    hide(this.zoomSelectorTarget)
    this.pageSizeTarget.value = this.pageSize
    this.updateDataTypeControl()
    this.fetchData()
    insertOrUpdateQueryParam('view-option', this.viewOption, 'chart')
  }

  setChart () {
    this.viewOption = 'chart'
    insertOrUpdateQueryParam('view-option', this.viewOption, 'chart')
    setActiveOptionBtn(this.viewOption, this.viewOptionTargets)
    hide(this.tableWrapperTarget)
    hide(this.messageViewTarget)
    show(this.chartWrapperTarget)
    hide(this.paginationWrapperTarget)
    hide(this.pageSizeWrapperTarget)
    this.updateDataTypeControl()
    this.fetchDataAndPlotGraph()
    // reset this table properties as the url params will be reset
    this.currentPage = 1
    this.pageSize = 20
  }

  trimUrlParam () {
    var baseSet = ['platform', 'view-option']
    var keepSet = []
    if (this.viewOption === 'table') {
      const tableParams = ['page', 'records-per-page', ...baseSet]
      switch (this.platform) {
        case redditPlatform:
          keepSet = ['subreddit', ...tableParams]
          break
        case youtubePlatform:
          keepSet = ['channel', ...tableParams]
          break
        case githubPlatform:
          keepSet = ['repository', ...tableParams]
          break
        case twitterPlatform:
          keepSet = ['twitter-handle', ...tableParams]
          break
      }
    } else {
      var chartParams = ['zoom', ...baseSet]
      switch (this.platform) {
        case redditPlatform:
          keepSet = ['subreddit', 'data-type', ...chartParams]
          break
        case youtubePlatform:
          keepSet = ['data-type', ...chartParams]
          break
        case githubPlatform:
          keepSet = ['repository', 'data-type', ...chartParams]
          break
        case twitterPlatform:
          keepSet = ['twitter-handle', ...chartParams]
          break
      }
    }

    trimUrl(keepSet)
  }

  platformChanged (event) {
    this.platform = event.currentTarget.value
    insertOrUpdateQueryParam('platform', this.platform, this.platformTarget.options[0].value)
    this.showCurrentSubAccountWrapper()
    this.updateDataTypeControl()
    this.resetSubAccountsAndDataType()
    removeUrlParam('data-type')
    this.currentPage = 1
    if (this.viewOption === 'table') {
      this.fetchData()
    } else {
      this.fetchDataAndPlotGraph()
    }
    insertOrUpdateQueryParam('platform', this.platform, this.platformTarget.options[0].innerText)
  }

  resetSubAccountsAndDataType () {
    if (this.subredditTarget.options.length > 0) {
      this.subredditTarget.value = this.subredditTarget.options[0].value
    }
    if (this.twitterHandleTarget.options.length > 0) {
      this.twitterHandleTarget.value = this.twitterHandleTarget.options[0].value
    }
    if (this.repositoryTarget.options.length > 0) {
      this.repositoryTarget.value = this.repositoryTarget.options[0].value
    }
    if (this.channelTarget.options.length > 0) {
      this.channelTarget.value = this.channelTarget.options[0].value
    }
  }

  subredditChanged (event) {
    this.subreddit = event.currentTarget.value
    let defaultSubreddit
    if (event.currentTarget.options.length > 0) {
      defaultSubreddit = event.currentTarget.options[0].value
    }
    insertOrUpdateQueryParam('subreddit', this.subreddit, defaultSubreddit)
    this.currentPage = 1
    if (this.viewOption === 'table') {
      this.fetchData()
    } else {
      this.fetchDataAndPlotGraph()
    }
    insertOrUpdateQueryParam('subreddit', this.subreddit, event.currentTarget.options[0].innerText)
  }

  twitterHandleChanged (event) {
    this.twitterHandle = event.currentTarget.value
    let defaultTwitterHandle
    if (event.currentTarget.options.length > 0) {
      defaultTwitterHandle = event.currentTarget.options[0].value
    }
    insertOrUpdateQueryParam('twitter-handle', this.twitterHandle, defaultTwitterHandle)
    this.currentPage = 1
    if (this.viewOption === 'table') {
      this.fetchData()
    } else {
      this.fetchDataAndPlotGraph()
    }
    insertOrUpdateQueryParam('twitter-handle', this.twitterHandle, event.currentTarget.options[0].innerText)
  }

  repositoryChanged (event) {
    this.repository = event.currentTarget.value
    let defaultRepository
    if (event.currentTarget.options.length > 0) {
      defaultRepository = event.currentTarget.options[0].value
    }
    insertOrUpdateQueryParam('repository', this.repository, defaultRepository)
    this.currentPage = 1
    if (this.viewOption === 'table') {
      this.fetchData()
    } else {
      this.fetchDataAndPlotGraph()
    }
    insertOrUpdateQueryParam('repository', this.repository, event.currentTarget.options[0].innerText)
  }

  channelChanged (event) {
    this.channel = event.currentTarget.value
    let defaultChannel
    if (event.currentTarget.options.length > 0) {
      defaultChannel = event.currentTarget.options[0].value
    }
    insertOrUpdateQueryParam('channel', this.channel, defaultChannel)
    this.currentPage = 1
    if (this.viewOption === 'table') {
      this.fetchData()
    } else {
      this.fetchDataAndPlotGraph()
    }
    insertOrUpdateQueryParam('channel', this.channel, event.currentTarget.options[0].innerText)
  }

  dataTypeChanged (event) {
    // if this will lead to empty selection, check it back and discontinue
    let selectionCount = 0
    this.dataTypeTargets.forEach(el => {
      if (el.checked) {
        selectionCount++
      }
    })
    if (selectionCount === 0) {
      event.currentTarget.checked = true
      return
    }
    this.fetchDataAndPlotGraph()
  }

  showCurrentSubAccountWrapper () {
    const that = this
    this.subAccountWrapperTargets.forEach(el => {
      if (el.dataset.platform === that.platform) {
        show(el)
      } else {
        hide(el)
      }
    })
  }

  updateDataTypeControl () {
    this.dataTypeContainerTarget.innerHTML = ''
    this.labels = []
    hide(this.dataTypeWrapperTarget)
    if (this.viewOption !== 'chart') {
      return
    }

    const _this = this
    const addDataTypeOption = function (value, label, isChecked) {
      let checked = isChecked ? 'checked' : ''
      _this.dataTypeContainerTarget.innerHTML += `
      <div class="form-check form-check-inline ml-1">
          <input data-target="commstat.dataType" data-action="click->commstat#dataTypeChanged"
          class="form-check-input" type="checkbox" value="${value}" data-label="${label}" ${checked}>
          <label class="form-check-label">${label}</label>
      </div>`
    }
    switch (this.platform) {
      case redditPlatform:
        if (this.dataType !== 'subscribers' && this.dataType !== 'active_accounts') {
          this.dataType = 'subscribers'
        }
        addDataTypeOption('subscribers', 'Subscribers', true)
        addDataTypeOption('active_accounts', 'Active Accounts')
        show(_this.dataTypeWrapperTarget)
        break
      case githubPlatform:
        if (this.dataType !== 'folks' && this.dataType !== 'stars') {
          this.dataType = 'folks'
        }
        addDataTypeOption('folks', 'Forks', true)
        addDataTypeOption('stars', 'Stars')
        show(_this.dataTypeWrapperTarget)
        break
      case youtubePlatform:
        if (this.dataType !== 'subscribers' && this.dataType !== 'view_count') {
          this.dataType = 'subscribers'
        }
        addDataTypeOption('subscribers', 'Subscribers', true)
        addDataTypeOption('view_count', 'View Count')
        show(_this.dataTypeWrapperTarget)
        break
      case twitterPlatform:
        this.dataType = 'followers'
        this.label = 'Followers'
        break
    }
  }

  loadPreviousPage () {
    this.currentPage -= 1
    if (this.currentPage < 1) {
      this.currentPage = 1
    }
    insertOrUpdateQueryParam('page', this.currentPage, 1)
    this.fetchData()
    insertOrUpdateQueryParam('page', this.nextPage, 1)
  }

  loadNextPage () {
    this.currentPage += 1
    insertOrUpdateQueryParam('page', this.currentPage, 1)
    this.fetchData()
    insertOrUpdateQueryParam('page', this.nextPage, 1)
  }

  pageSizeChanged (event) {
    this.currentPage = 1
    this.pageSize = event.currentTarget.value
    let defaultPageSize
    if (event.currentTarget.options.length > 0) {
      defaultPageSize = event.currentTarget.options[0].value
    }
    insertOrUpdateQueryParam('page', this.currentPage, 1)
    insertOrUpdateQueryParam('records-per-page', this.pageSize, defaultPageSize)
    this.fetchData()
    insertOrUpdateQueryParam('records-per-page', this.selectedNumTarget.value, 20)
  }

  fetchData () {
    let elementsToToggle = [this.tableWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)
    const _this = this
    const queryString = `page=${_this.currentPage}&records-per-page=${this.pageSize}&view-option=` +
      `${_this.viewOption}&platform=${this.platform}&subreddit=${this.subreddit}&twitter-handle=${this.twitterHandle}` +
      `&repository=${this.repository}&channel=${this.channel}`
    axios.get(`/getCommunityStat?${queryString}`)
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
          hide(_this.tableTarget)
          hide(_this.paginationWrapperTarget)
          _this.totalPageCountTarget.textContent = 0
          _this.currentPageTarget.textContent = 0
          _this.trimUrlParam()
        } else {
          show(_this.tableTarget)
          show(_this.paginationWrapperTarget)
          hide(_this.messageViewTarget)
          _this.trimUrlParam()

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

          _this.displayRecord(result.stats, result.columns)
        }
      }).catch(function (e) {
        console.log(e)
      })
  }

  displayRecord (stats, columns) {
    hide(this.messageViewTarget)
    show(this.tableWrapperTarget)
    const _this = this
    this.tableTarget.innerHTML = ''

    this.tableCol1Target.innerText = columns[0]
    this.tableCol2Target.innerText = columns[1]
    if (columns.length > 2) {
      this.tableCol3Target.innerText = columns[2]
      show(this.tableCol3Target)
    } else {
      hide(this.tableCol3Target)
    }

    if (!stats) {
      return
    }

    stats.forEach(stat => {
      const exRow = document.importNode(_this.rowTemplateTarget.content, true)
      const fields = exRow.querySelectorAll('td')

      fields[0].innerHTML = formatDate(new Date(stat.date))
      switch (_this.platform) {
        case 'Reddit':
          _this.displayRedditData(stat, fields)
          break
        case 'Twitter':
          _this.displayTwitterStat(stat, fields)
          break
        case 'GitHub':
          _this.displayGithubData(stat, fields)
          break
        case 'YouTube':
          _this.displayYoutubeData(stat, fields)
          break
      }

      _this.tableTarget.appendChild(exRow)
    })
  }

  displayRedditData (stat, fields) {
    fields[1].innerHTML = stat.subscribers
    fields[2].innerText = stat.active_user_count
  }

  displayTwitterStat (stat, fields) {
    fields[1].innerHTML = stat.followers
    hide(fields[2])
  }

  displayGithubData (stat, fields) {
    fields[1].innerHTML = stat.stars
    fields[2].innerText = stat.folks
  }

  displayYoutubeData (stat, fields) {
    fields[1].innerHTML = stat.subscribers
    fields[2].innerText = stat.view_count
  }

  fetchDataAndPlotGraph () {
    let elementsToToggle = [this.chartWrapperTarget]
    showLoading(this.loadingDataTarget, elementsToToggle)

    const _this = this
    const selections = []
    this.labels = []
    this.dataTypeTargets.forEach(el => {
      if (!el.checked) return
      _this.labels.push(el.getAttribute('data-label'))
      selections.push(el.value)
    })
    if (selections.length === 0) {
      this.labels.push(this.label)
      selections.push(this.dataType)
    }
    const queryString = `data-type=${selections.join('--')}&platform=${this.platform}&subreddit=${_this.subreddit}` +
      `&twitter-handle=${this.twitterHandle}&view-option=${this.viewOption}&repository=${this.repository}&channel=${this.channel}`
    _this.trimUrlParam()

    axios.get(`/communitychat?${queryString}`).then(function (response) {
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

  plotGraph (dataSet) {
    const _this = this

    let minDate, maxDate
    dataSet.x.forEach(unixTime => {
      let date = new Date(unixTime * 1000)
      if (minDate === undefined || date < minDate) {
        minDate = date
      }

      if (maxDate === undefined || date > maxDate) {
        maxDate = date
      }
    })

    let options = {
      legend: 'always',
      includeZero: true,
      legendFormatter: legendFormatter,
      labelsDiv: _this.labelsTarget,
      ylabel: dataSet.ylabel,
      xlabel: 'Date',
      labels: ['Date', ...this.labels],
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

    const chartData = zipXYZData(dataSet)
    _this.chartsView = new Dygraph(_this.chartsViewTarget, chartData, options)
    _this.validateZoom()
    if (updateZoomSelector(_this.zoomOptionTargets, minDate, maxDate, 1)) {
      show(this.zoomSelectorTarget)
    } else {
      hide(this.zoomSelectorTarget)
    }
  }

  selectedZoom () { return selectedOption(this.zoomOptionTargets) }

  setZoom (e) {
    var target = e.srcElement || e.target
    var option
    if (!target) {
      let ex = this.chartsView.xAxisExtremes()
      option = Zoom.mapKey(e, ex, 1)
    } else {
      option = target.dataset.option
    }
    setActiveOptionBtn(option, this.zoomOptionTargets)
    if (!target) return // Exit if running for the first time
    this.validateZoom()
    insertOrUpdateQueryParam('zoom', option, 'all')
  }

  async validateZoom () {
    await animationFrame()
    await animationFrame()
    let oldLimits = this.limits || this.chartsView.xAxisExtremes()
    this.limits = this.chartsView.xAxisExtremes()
    var selected = this.selectedZoom()
    if (selected) {
      this.lastZoom = Zoom.validate(selected, this.limits, 1, 1)
    } else {
      this.lastZoom = Zoom.project(this.settings.zoom, oldLimits, this.limits)
    }
    if (this.lastZoom) {
      this.chartsView.updateOptions({
        dateWindow: [this.lastZoom.start, this.lastZoom.end]
      })
    }
    if (selected !== this.settings.zoom) {
      this._zoomCallback(this.lastZoom.start, this.lastZoom.end)
    }
    await animationFrame()
    this.chartsView.updateOptions({
      zoomCallback: this.zoomCallback,
      drawCallback: this.drawCallback
    })
  }

  _zoomCallback (start, end) {
    this.lastZoom = Zoom.object(start, end)
    this.settings.zoom = Zoom.encode(this.lastZoom)
    // this.query.replace(this.settings)
    let ex = this.chartsView.xAxisExtremes()
    let option = Zoom.mapKey(this.settings.zoom, ex, 1)
    setActiveOptionBtn(option, this.zoomOptionTargets)
  }

  _drawCallback (graph, first) {
    if (first) return
    var start, end
    [start, end] = this.chartsView.xAxisRange()
    if (start === end) return
    if (this.lastZoom.start === start) return // only handle slide event.
    this._zoomCallback(start, end)
  }
}
