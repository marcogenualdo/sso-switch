package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"
)

const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SSO Proxy - User Headers</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .container {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
            max-width: 600px;
            width: 90%;
        }
        h1 {
            color: #333;
            margin: 0 0 1.5rem 0;
            font-size: 1.8rem;
            text-align: center;
        }
        .headers {
            list-style: none;
            padding: 0;
            margin: 0;
        }
        .header-item {
            padding: 0.75rem;
            margin-bottom: 0.5rem;
            background: #f8f9fa;
            border-radius: 4px;
            border-left: 4px solid #667eea;
        }
        .header-name {
            font-weight: 600;
            color: #667eea;
            display: block;
            margin-bottom: 0.25rem;
        }
        .header-value {
            color: #555;
            word-break: break-all;
        }
        .no-headers {
            text-align: center;
            color: #999;
            padding: 2rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Authenticated User Information</h1>
        {{if .Headers}}
        <ul class="headers">
            {{range .Headers}}
            <li class="header-item">
                <span class="header-name">{{.Name}}</span>
                <span class="header-value">{{.Value}}</span>
            </li>
            {{end}}
        </ul>
        {{else}}
        <div class="no-headers">No user headers found</div>
        {{end}}
    </div>
</body>
</html>
`

type HeaderData struct {
	Name  string
	Value string
}

type PageData struct {
	Headers []HeaderData
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Incoming request: %s %s", r.Method, r.URL.Path)

	var headers []HeaderData
	for name, values := range r.Header {
		if strings.HasPrefix(name, "X-User-") || strings.HasPrefix(name, "X-User") {
			for _, value := range values {
				headers = append(headers, HeaderData{
					Name:  name,
					Value: value,
				})
			}
		}
	}

	tmpl, err := template.New("page").Parse(htmlTemplate)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	data := PageData{Headers: headers}
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

func main() {
	http.HandleFunc("/", handler)

	addr := ":8080"
	log.Printf("Listening on %s", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}
