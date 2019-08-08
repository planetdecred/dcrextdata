package web

import (
	"encoding/json"
	"log"
	"net/http"
)

func (s *Server) render(tplName string, data map[string]interface{}, res http.ResponseWriter) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if tpl, ok := s.templates[tplName]; ok {
		err := tpl.Execute(res, data)
		if err != nil {
			log.Fatalf("error executing template: %s", err.Error())
		}
		return
	}

	log.Fatalf("template %s is not registered", tplName)
}

func (routes *Server) renderError(errorMessage string, res http.ResponseWriter) {
	data := map[string]interface{}{
		"error": errorMessage,
	}
	routes.render("error.html", data, res)
}

func (routes *Server) renderErrorJSON(errorMessage string, res http.ResponseWriter) {
	data := map[string]interface{}{
		"error": errorMessage,
	}
	routes.renderJSON(data, res)
}

func (routes *Server) renderJSON(data interface{}, res http.ResponseWriter) {
	d, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("error marshalling data: %s", err.Error())
	}

	res.Header().Set("Content-Type", "application/json")
	res.Write(d)
}
