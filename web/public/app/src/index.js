import { Application } from 'stimulus'
import { definitionsFromContext } from 'stimulus/webpack-helpers'
import './css/style.scss'
import '../node_modules/bootstrap4-toggle/css/bootstrap4-toggle.css'
import '../node_modules/bootstrap4-toggle/js/bootstrap4-toggle.js'

const application = Application.start()
const context = require.context('./controllers', true, /\.js$/)
application.load(definitionsFromContext(context))
