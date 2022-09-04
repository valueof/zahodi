package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/relvacode/iso8601"
	"golang.org/x/net/html"
)

type Price struct {
	Currency string
	Amount   int
}

type Address struct {
	Value      string
	Street     string
	PostalCode string
	Locality   string
	Region     string
}

type Event struct {
	StartDate time.Time
	EndDate   time.Time
	Name      string
}

type Listing struct {
	URL          string
	CanonicalURL string
	Description  string
	Address      Address
	PhotoUrl     string
	Price        Price
	OpenHouses   []Event
}

type ZillowObject struct {
	Type      string `json:"@type"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Name      string `json:"name"`
}

func getAttr(node *html.Node, key string) string {
	if node.Type != html.ElementNode {
		return ""
	}

	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val
		}
	}

	return ""
}

func maybeParseEvent(text []byte) *Event {
	if !json.Valid([]byte(text)) {
		return nil
	}

	var data ZillowObject

	err := json.Unmarshal([]byte(text), &data)
	if err != nil {
		fmt.Printf("error when parsing JSON object: %v\n", err)
		return nil
	}

	if data.Type != "Event" {
		return nil
	}

	event := Event{Name: data.Name}

	startDate, err := iso8601.ParseString(data.StartDate)
	if err == nil {
		event.StartDate = startDate
	}

	endDate, err := iso8601.ParseString(data.EndDate)
	if err == nil {
		event.EndDate = endDate
	}

	return &event
}

func NewListing(url string) (l *Listing) {
	l = &Listing{
		URL:        url,
		OpenHouses: []Event{},
	}

	return l
}

func (l *Listing) Populate() (err error) {
	url := strings.TrimSpace(l.URL)
	if url == "" {
		return errors.New("URL is empty")
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	// Pretend we're a browser so Zillow doesn't throw CAPTCHA
	req.Header.Add("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
	req.Header.Add("accept-language", "en-US,en;q=0.8")
	req.Header.Add("upgrade-insecure-requests", "1")
	req.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		return
	}

	doc, err := html.Parse(res.Body)
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("expected %d response, got %d", http.StatusOK, res.StatusCode)
	}

	if err != nil {
		return
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}

		if n.Data == "meta" {
			switch {
			case getAttr(n, "name") == "description":
				l.Description = getAttr(n, "content")
			case getAttr(n, "property") == "og:image":
				l.PhotoUrl = getAttr(n, "content")
			case getAttr(n, "property") == "og:zillow_fb:address":
				l.Address = Address{Value: getAttr(n, "content")}
			case getAttr(n, "name") == "canonical":
				l.CanonicalURL = getAttr(n, "content")
			}
			return
		}

		if n.Data == "script" && getAttr(n, "type") == "application/ld+json" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != html.TextNode {
					continue
				}

				event := maybeParseEvent([]byte(c.Data))
				if event != nil {
					l.OpenHouses = append(l.OpenHouses, *event)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		f(c)
	}

	return nil
}
