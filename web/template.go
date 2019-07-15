package web

import (
	"fmt"
	"log"
	"text/template"
	"time"
)

func (s *Server) loadTemplates() {
	layout := "web/views/layout.html"
	tpls := map[string]string{
		"error.html":       "web/views/error.html",
		"exchange.html":    "web/views/exchange.html",
		"vsp.html":         "web/views/vsp.html",
		"pow.html":         "web/views/pow.html",
		"mempool.html":     "web/views/mempool.html",
		"propagation.html": "web/views/propagation.html",
	}

	for i, v := range tpls {
		tpl, err := template.New(i).Funcs(templateFuncMap()).ParseFiles(v, layout)
		if err != nil {
			log.Fatalf("error loading templates: %s", err.Error())
		}

		s.lock.Lock()
		s.templates[i] = tpl
		s.lock.Unlock()
	}
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"incByOne": func(number int) int {
			return number + 1
		},
		"formatDate": func(date time.Time) string {
			return date.Format("2006-01-02 15:04")
		},
		"formatDateMilli": func(date time.Time) string {
			return date.Format("2006-01-02 15:04:05.99")
		},
		"normalizeBalance": func(balance float64) string {
			return fmt.Sprintf("%010.8f DCR", balance)
		},
		"timestamp": func() int64 {
			return time.Now().Unix()
		},
	}
}
