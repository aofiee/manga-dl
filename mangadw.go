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

	"github.com/fatih/color"
	"github.com/gocolly/colly"
)

var target string

const (
	chBuf = 10
)

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
func arg() bool {
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	if len(os.Args) != 2 {
		log.Println(red("Missing argument [ IG account ]"))
		os.Exit(1)
	}
	return true
}
func grepLessonLink() {
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan, color.Italic).SprintFunc()

	c := colly.NewCollector()

	c.OnResponse(func(r *colly.Response) {
		log.Println("response received", r.StatusCode)
	})
	c.OnHTML("div.table-responsive", func(e *colly.HTMLElement) {
		e.ForEach("a", func(_ int, elem *colly.HTMLElement) {
			link := elem.Attr("href")
			chanSendLink := make(chan string, chBuf)
			chanDownloaded := make(chan bool)
			go grepLessonInBook(chanSendLink, link)
			go extracAllPagesFromLesson(chanSendLink, chanDownloaded)
			log.Println("Downloaded "+cyan(link)+" : ", <-chanDownloaded)
		})
	})
	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting : ", green(r.URL))
	})
	c.Visit(target)
}
func grepLessonInBook(ch chan<- string, l string) {
	c := colly.NewCollector()
	color := color.New(color.FgBlue, color.Italic).SprintFunc()
	c.OnHTML("#page_select1", func(e *colly.HTMLElement) {
		e.ForEach("option", func(_ int, elem *colly.HTMLElement) {
			ch <- elem.Attr("value")
			log.Println(color("Send : ", elem.Attr("value")))
		})
		defer close(ch)
	})

	c.Visit(l)
}
func extracAllPagesFromLesson(ch <-chan string, done chan bool) {
	chanExtractLink := make(chan map[string]string, chBuf)
	chanDoneExtractLink := make(chan bool)
	go exctractImgSrc(ch, chanExtractLink)
	go downloadAndSave(chanExtractLink, chanDoneExtractLink)
	log.Println("chanDoneExtractLink ", <-chanDoneExtractLink)
	defer close(done)
	done <- true
}
func exctractImgSrc(rootCh <-chan string, chanExtractLink chan<- map[string]string) {
	for v := range rootCh {
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
		c := colly.NewCollector()
		c.OnResponse(func(r *colly.Response) {
			// log.Println("response received", r.StatusCode)
		})
		c.OnHTML("body > div.container-fluid > center > div.display_content > img", func(e *colly.HTMLElement) {
			log.Println("Prepare to Download : ", color(strings.TrimSpace(e.Attr("src"))))
			var chanPipe map[string]string
			chanPipe = make(map[string]string)
			chanPipe["url"], chanPipe["destination"] = strings.TrimSpace(e.Attr("src")), destination
			chanExtractLink <- chanPipe
		})
		c.OnRequest(func(r *colly.Request) {
			// log.Println("Page : ", green(r.URL))
		})
		c.Visit(v)
	}
	defer close(chanExtractLink)
}
func downloadAndSave(chanExtractLink <-chan map[string]string, done chan bool) {
	for v := range chanExtractLink {

		filename := path.Base(v["url"])
		chanStart := make(chan []byte, chBuf)
		chanDone := make(chan bool)
		go downloadFile(v["url"], chanStart, chanDone)
		go saveImg(filename, v["destination"], chanStart, chanDone)
	}
	defer close(done)
	done <- true
}
func saveImg(filename string, des string, ch <-chan []byte, done chan bool) {
	color := color.New(color.FgMagenta, color.Italic).SprintFunc()
	path := fmt.Sprintf(`%v/%v`, des, filename)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		log.Println(color("-------save------- : ", path))
		f.Close()
		done <- true
	}()
	_, errCopy := io.Copy(f, bytes.NewReader(<-ch))
	if errCopy != nil {
		log.Fatal(errCopy)
	}
}

func downloadFile(s string, ch chan<- []byte, done chan bool) {
	color := color.New(color.FgRed, color.Italic).SprintFunc()
	res, err := http.Get(s)
	if err != nil {
		ch <- nil
	}
	defer func() {
		log.Println(color("Downloader : ", s))
		done <- true
		res.Body.Close()
		close(ch)
	}()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		ch <- nil
	}
	ch <- body
}
