package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var TEST_URLS []string = []string{
	"https://www.zillow.com/homedetails/361-Blaine-St-Seattle-WA-98109/48689963_zpid/",
	"https://www.zillow.com/homedetails/2854-S-Nevada-St-Seattle-WA-98108/70579954_zpid/",
	"https://www.zillow.com/homedetails/1109-122nd-Ave-E-Puyallup-WA-98372/2061905748_zpid/",
}

type IndexData struct {
	ListId   string
	Listings []*Listing
}

func init() {
	rand.Seed(time.Now().Unix())
}

// from /usr/share/dict/words
// confirm whether it exists in docker?
var words []string = []string{
	"abacay",
	"abacinate",
	"abacination",
	"abaciscus",
	"abacist",
	"aback",
	"abactinal",
	"abactinally",
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		id := []string{}
		for i := 0; i < 3; i++ {
			id = append(id, words[rand.Int()%len(words)])
		}
		http.Redirect(w, r, fmt.Sprintf("/%s", strings.Join(id, "-")), http.StatusSeeOther)
		return
	}

	listings := []*Listing{}
	for _, url := range TEST_URLS {
		l := NewListing(url)
		if err := l.Populate(); err != nil {
			fmt.Println(err)
			continue
		}

		listings = append(listings, l)
	}

	render(w, r, "index.html", IndexData{
		ListId:   strings.TrimLeft(r.URL.Path, "/"),
		Listings: listings,
	})
}
