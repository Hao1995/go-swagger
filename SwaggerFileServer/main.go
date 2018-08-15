package main

import (
	"net/http"
)

func main() {
	http.HandleFunc("/swagger.json", swagger)
	changeHeaderThenServe := func(h http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			h.ServeHTTP(w, r)
		}
	}

	http.Handle("/", changeHeaderThenServe(http.FileServer(http.Dir("/swagger"))))
	// http.Handle("/", changeHeaderThenServe(http.FileServer(http.Dir("C:\\docker\\repo\\hook_1"))))

	// //Gitlab Webhooks
	// http.HandleFunc("/gitlab", gitlabMergeEvent)

	http.ListenAndServe(":8080", nil)
}

func swagger(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	http.ServeFile(w, req, "swagger.json")
}

// type Data struct {
// 	Name  string
// 	Value string
// }

// func gitlabMergeEvent(w http.ResponseWriter, req *http.Request) {
// 	if err := req.ParseForm(); err != nil {
// 		fmt.Fprintf(w, "ParseForm() err: %v", err)
// 		return
// 	}

// 	total := []*Data{}

// 	for k, v := range req.Form {
// 		item := &Data{}
// 		item.Name = k
// 		item.Value = strings.Join(v, "")

// 		total = append(total, item)
// 	}

// 	if err := json.NewEncoder(w).Encode(total); err != nil {
// 		panic(err)
// 	}
// }
