// run: go run main.go -apikey=cbcb28b2886f4b24be3fa8b6bc4ce020

package main

import (
	"encoding/json"
	"flag"
	"fmt"           // I/O printf/scanf
	"html/template" // generate html + prevent code injection
	"log"
	"math"
	"net/http" // provide HTTP client and server implementations
	"net/url"  // parse URLs
	"os"       // general operating system functionalities
	"strconv"
	"time"
)

var tpl = template.Must(template.ParseFiles("index.html"))

var apiKey *string

type Source struct {
	ID   interface{} `json:"id"`
	Name string      `json:"name"`
}

type Article struct {
	Source      Source    `json:"source"`
	Author      string    `json:"author"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	URLToImage  string    `json:"urlToImage"`
	PublishedAt time.Time `json:"publishedAt"`
	Content     string    `json:"content"`
}

func (a *Article) FormatPublishedDate() string {
	year, month, day := a.PublishedAt.Date()
	return fmt.Sprintf("%v %d, %d", month, day, year)
}

type Results struct {
	Status       string    `json:"status"`
	TotalResults int       `json:"totalResults"`
	Articles     []Article `json:"articles"`
}

type Search struct {
	SearchKey  string
	NextPage   int
	TotalPages int
	Results    Results
}

func (s *Search) IsLastPage() bool {
	return s.NextPage >= s.TotalPages
}

func (s *Search) CurrentPage() int {
	if s.NextPage == 1 {
		return s.NextPage
	}

	return s.NextPage - 1
}

func (s *Search) PreviousPage() int {
	return s.CurrentPage() - 1
}

type NewsAPIError struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// w: send response to an HTTP request
// r: HTTP request received from the client
func indexHandler(w http.ResponseWriter, r *http.Request) {
	tpl.Execute(w, nil)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.String())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
		return
	}

	params := u.Query()
	searchKey := params.Get("q")
	page := params.Get("page")
	if page == "" {
		page = "1"
	}

	search := &Search{}
	search.SearchKey = searchKey

	next, err := strconv.Atoi(page)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	search.NextPage = next
	pageSize := 10

	endpoint := fmt.Sprintf("https://newsapi.org/v2/everything?q=%s&pageSize=%d&page=%d&apiKey=%s&sortBy=publishedAt&language=en", url.QueryEscape(search.SearchKey), pageSize, search.NextPage, *apiKey)
	resp, err := http.Get(endpoint)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		newError := &NewsAPIError{}
		err := json.NewDecoder(resp.Body).Decode(newError)
		if err != nil {
			http.Error(w, "Unexpected server error", http.StatusInternalServerError)
			return
		}

		http.Error(w, newError.Message, http.StatusInternalServerError)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&search.Results)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	search.TotalPages = int(math.Ceil(float64(search.Results.TotalResults / pageSize)))
	if ok := !search.IsLastPage(); ok {
		search.NextPage++
	}
	err = tpl.Execute(w, search)
	if err != nil {
		log.Println(err)
	}
}

func main() {
	apiKey = flag.String("apikey", "", "Newsapi.org access key")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("apiKey must be set")
	}

	port := os.Getenv("PORT")
	fs := http.FileServer(http.Dir("assets"))

	if port == "" {
		port = "3000"
	}

	// create a new HTTP request multiplexer
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", fs))
	// handle the HTTP request
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/search", searchHandler)

	// start the server on port 3000
	http.ListenAndServe(":"+port, mux)
}
