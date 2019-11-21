package web

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

func (s *Server) render(tplName string, data map[string]interface{}, res http.ResponseWriter) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if tpl, ok := s.templates[tplName]; ok {
		err := tpl.Execute(res, data)
		if err == nil {
			return
		}
		// Filter out broken pipe (user pressed "stop") errors
		if _, ok := err.(*net.OpError); ok {
			if strings.Contains(err.Error(), "broken pipe") {
				return
			}
		}
		log.Errorf("Error executing template: %s", err.Error())
		return
	}

	log.Errorf("Template %s is not registered", tplName)
}

func (s *Server) renderError(errorMessage string, res http.ResponseWriter) {
	data := map[string]interface{}{
		"error": errorMessage,
	}
	s.render("error.html", data, res)
}

func (s *Server) renderErrorJSON(errorMessage string, res http.ResponseWriter) {
	data := map[string]interface{}{
		"error": errorMessage,
	}

	s.renderJSON(data, res)
}

func (s *Server) renderJSON(data interface{}, res http.ResponseWriter) {
	d, err := json.Marshal(data)
	if err != nil {
		log.Errorf("Error marshalling data: %s", err.Error())
	}

	res.Header().Set("Content-Type", "application/json")
	res.Write(d)
}
