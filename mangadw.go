package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/gocolly/colly"
)

var target string

const (
	chBuf = 100
)

type scraping struct {
	collector *colly.Collector
}

type pFunc func(_ int, elem *colly.HTMLElement)

func main() {
	log.SetFlags(log.Ltime)
	if arg() {
		setTargetURL(&target)
		grepLessonLink()
	}
}
func setTargetURL(t *string) {
	*t = os.Args[1]
}
func (s *scraping) scraping(url string, findSelector string, useSelector string, p pFunc) {
	s.collector.OnHTML(findSelector, func(e *colly.HTMLElement) {
		e.ForEach(useSelector, p)
	})
	s.collector.Visit(url)

}
func arg() bool {
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	if len(os.Args) != 2 {
		log.Println(red("Missing argument [ IG account ]"))
		os.Exit(1)
	}
	return true
}
func grepLessonLink() {
	cyan := color.New(color.FgCyan, color.Italic).SprintFunc()
	c := scraping{colly.NewCollector()}
	c.scraping(target, "div.table-responsive", "a", func(_ int, elem *colly.HTMLElement) {
		link := elem.Attr("href")
		chanSendLink := make(chan string, chBuf)
		chanDownloaded := make(chan bool)
		go grepLessonInBook(chanSendLink, link)
		go extracAllPagesFromLesson(chanSendLink, chanDownloaded)
		log.Println("Downloaded "+cyan(link)+" : ", <-chanDownloaded)
	})
}
func grepLessonInBook(ch chan<- string, l string) {
	c := scraping{colly.NewCollector()}
	color := color.New(color.FgBlue, color.Italic).SprintFunc()
	c.scraping(l, "#page_select1", "option", func(_ int, elem *colly.HTMLElement) {
		ch <- elem.Attr("value")
		log.Println(color("Send : ", elem.Attr("value")))
	})
	defer close(ch)
}
func extracAllPagesFromLesson(ch <-chan string, done chan bool) {
	counter := sync.WaitGroup{}
	for v := range ch {
		v := v

		u, err := url.Parse(v)
		if err != nil {
			log.Fatal(err)
		}
		splitPath := strings.Split(u.Path, "/")
		destination := fmt.Sprintf(`./%v/%v`, splitPath[1], splitPath[2])
		_, errCheckDirIsNotExist := os.Stat(destination)
		if os.IsNotExist(errCheckDirIsNotExist) {
			errDir := os.MkdirAll(destination, 0755)
			if errDir != nil {
				log.Fatal(err)
			}
		}
		color := color.New(color.FgGreen, color.Italic).SprintFunc()
		c := scraping{colly.NewCollector()}
		c.scraping(v, "body > div.container-fluid > center > div.display_content", "img", func(_ int, elem *colly.HTMLElement) {
			counter.Add(1)
			go func() {
				defer counter.Done()
				log.Println(color("Waiting to Download : ", elem.Attr("src")))
				url, destination := strings.TrimSpace(elem.Attr("src")), destination
				data, err := downloadFile(url)
				if err != nil {
					log.Fatal(err)
				}
				filename := path.Base(url)
				saveImg(filename, destination, data)
			}()
		})
	}
	go func() {
		counter.Wait()
		done <- true
		close(done)
	}()

}
func saveImg(filename string, des string, data []byte) {
	color := color.New(color.FgMagenta, color.Italic).SprintFunc()
	path := fmt.Sprintf(`%v/%v`, des, filename)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		log.Println(color("-------save------- : ", path))
		f.Close()
	}()
	_, errCopy := io.Copy(f, bytes.NewReader(data))
	if errCopy != nil {
		log.Fatal(errCopy)
	}
}

func downloadFile(s string) ([]byte, error) {
	color := color.New(color.FgRed, color.Italic).SprintFunc()
	res, err := http.Get(s)
	if err != nil {
		return nil, err
	}
	defer func() {
		log.Println(color("-------Download------- : ", s))
		res.Body.Close()
	}()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
