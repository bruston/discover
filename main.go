package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	target := flag.String("u", "", "target url")
	workers := flag.Uint("c", uint(runtime.NumCPU()), "number of concurrent quests")
	timeout := flag.Uint("t", 5, "timeout in seconds")
	wordList := flag.String("w", "", "wordlist")
	hostHeader := flag.String("h", "", "host header")
	customKey := flag.String("ck", "", "add a custom header to all requests (key)")
	customVal := flag.String("cv", "", "add a custom header to all requests (value)")
	extension := flag.String("e", "", "file extension to add (without dot prefix)")
	success := flag.String("s", "200, 204, 301, 302, 307", "status codes indicating success, seperated by commas")
	userAgent := flag.String("a", "discover: https://github.com/bruston/discover", "user-agent to use")
	cookieFile := flag.String("cookies", "", "file containing cookies")
	prefix := flag.String("p", "", "prefix to add to word/directory")
	flag.Parse()
	if *target == "" {
		fmt.Println("You must specify a target URL with the -u parameter.")
		return
	}
	if *wordList == "" {
		fmt.Println("You must specify a word list with the -w parameter.")
	}

	cookies := ""
	if *cookieFile != "" {
		b, err := ioutil.ReadFile(*cookieFile)
		if err != nil {
			log.Fatal(err)
		}
		cookies = string(b)
		cookies = strings.TrimSpace(cookies)
	}
	f, err := os.Open(*wordList)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	work := make(chan string)
	s := bufio.NewScanner(f)
	go func() {
		for s.Scan() {
			work <- s.Text()
		}
		close(work)
	}()

	successCodes, err := cleanCodes(*success)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: time.Second * time.Duration(*timeout),
	}

	if !strings.HasSuffix(*target, "/") {
		*target = *target + "/"
	}

	fmt.Println("Discovering assets on", *target)
	wg := &sync.WaitGroup{}
	for i := 0; i < int(*workers); i++ {
		wg.Add(1)
		go func() {
			for v := range work {
				if *extension != "" {
					v = v + "." + *extension
				}
				code, word, err := doReq(client, *target, *hostHeader, *prefix+v, *customKey, *customVal, *userAgent, cookies)
				if err != nil {
					continue
				}
				for _, v := range successCodes {
					if v == code {
						fmt.Printf("/%s %d\n", word, code)
					}
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func cleanCodes(successCodes string) ([]int, error) {
	split := strings.Split(successCodes, ",")
	codes := []int{}
	for i := range split {
		split[i] = strings.TrimSpace(split[i])
		d, err := strconv.Atoi(split[i])
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid status code", split[i])
		}
		codes = append(codes, d)
	}
	return codes, nil
}

func doReq(client *http.Client, url, host, word, customKey, customVal, userAgent, cookies string) (int, string, error) {
	req, err := http.NewRequest("GET", url+word, nil)
	if err != nil {
		return 0, "", err
	}
	if host != "" {
		req.Host = host
	}
	if customKey != "" {
		req.Header.Set(customKey, customVal)
	}
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return 0, "", err
	}
	defer resp.Body.Close()
	io.Copy(ioutil.Discard, resp.Body)
	return resp.StatusCode, word, nil
}