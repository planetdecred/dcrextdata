package web

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/planetdecred/dcrextdata/app/helpers"
)

var templateDirs = []string{"web/views"}
var templates *template.Template

func (s *Server) loadTemplates() {
	layout := "web/views/layout.html"
	for _, dir := range templateDirs {
		files2, _ := ioutil.ReadDir(dir)
		for _, file := range files2 {
			filename := file.Name()
			if !strings.HasSuffix(filename, ".html") {
				continue
			}
			var files = []string{"web/views/" + filename}
			if !strings.HasPrefix(filename, "_") {
				files = append(files, layout)
			}
			tpl, err := template.New(filename).Funcs(templateFuncMap()).ParseFiles(files...)
			if err != nil {
				log.Errorf("Error loading templates: %s", err.Error())
			}

			s.lock.Lock()
			s.templates[filename] = tpl
			s.lock.Unlock()
		}
	}
}

var pairMap = map[string]string{
	"BTC/DCR": "DCR/BTC",
	"USD/BTC": "BTC/USD",
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
			return helpers.NowUTC().Unix()
		},
		"timeSince": func(timestamp int64) string {
			return time.Since(time.Unix(timestamp, 0).UTC()).String()
		},
		"formatUnixTime": func(timestamp int64) string {
			return time.Unix(timestamp, 0).Format(time.UnixDate)
		},
		"unixTimeAgo": func(timestamp int64) string {
			return time.Since(time.Unix(timestamp, 0)).String()
		},
		"strListContains": func(stringList []string, needle string) bool {
			for _, value := range stringList {
				if value == needle {
					return true
				}
			}
			return false
		},
		"stringsReplace": func(input string, old string, new string) string {
			return strings.Replace(input, old, new, -1)
		},
		"humanizeInt": func(number int64) string {
			s := strconv.Itoa(int(number))
			r1 := ""
			idx := 0

			// Reverse and interleave the separator.
			for i := len(s) - 1; i >= 0; i-- {
				idx++
				if idx == 4 {
					idx = 1
					r1 = r1 + ","
				}
				r1 = r1 + string(s[i])
			}

			// Reverse back and return.
			r2 := ""
			for i := len(r1) - 1; i >= 0; i-- {
				r2 = r2 + string(r1[i])
			}
			return r2
		},
		"commonPair": func(pair string) string {
			if v, f := pairMap[pair]; f {
				return v
			}
			return pair
		},
		"percentage": func(actual int64, total int64) string {
			return fmt.Sprintf("%.2f", 100*float64(actual)/float64(total))
		},
	}
}
