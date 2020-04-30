package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"text/template"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

/*
处理北朝临高区的数据
数据来源 https://lgqm.gq/thread-1722-1-1.html
*/

type User struct {
	Uid  int
	Name string
}

type Post struct {
	Pid     int
	Uid     int
	Content string

	PostTimeString string `json:"post_time"`

	Username string
	PostTime time.Time
}

type Thread struct {
	Tid       int
	Title     string
	Author    int
	PostCount int

	PostTimeString     string `json:"post_time"`
	LastPostTimeString string `json:"last_post_time"`

	Username     string
	PostTime     time.Time
	LastPostTime time.Time
	Posts        []*Post
}

const PostTimeLayout = "2006-1-2 15:04:05"
const LastPostTimeLayout = "2006-1-2 15:04"
const TimeLayout = "2006-01-02 15:04"

const Northdy = `---
aid: 9025
zid: {{ .Tid }}
title: '{{ .Title }}'
author: {{ .Username }}
date: {{ .PostTime.Format "2006-01-02 15:04:05+07:00"  }}
lastmod: {{ .LastPostTime.Format "2006-01-02 15:04:05+07:00"  }}
---
{{ range .Posts }}
{{ .Username }} 于 {{ .PostTimeString }} 发表了：
{{ .Content }}

---------
{{ end }}
`

var tc *template.Template
var converter *md.Converter
var client *http.Client
var wg sync.WaitGroup

func init() {
	t, err := template.New("northdy").Parse(Northdy)
	if err != nil {
		panic(err)
	}

	tc = t

	converter = md.NewConverter("", true, nil)
	converter.AddRules(
		md.Rule{
			Filter: []string{"img"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				alt := selec.AttrOr("alt", "")
				src := selec.AttrOr("file", "")

				// 可能是表情或者什么奇怪的图片，无视掉
				if src == "" || strings.HasPrefix(src, "static/image/") || strings.HasPrefix(src, "http://bbs.northdy.com//mobcent//app/data/phiz/default/") {
					empty := ""
					return &empty
					//src = selec.AttrOr("src", "")
				}

				// 北朝的图片就下载，否则先无视
				var text string
				if strings.HasPrefix(src, "https://bbs.northdy.com/data/attachment/forum/") {
					//download(src, path.Base(src))
					text = fmt.Sprintf("![%s](/img/%s)", alt, path.Base(src))
				} else {
					text = fmt.Sprintf("![%s](/img/%s)", alt, src)
				}
				return &text
			},
		},
		md.Rule{
			Filter: []string{"a"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				// 无视一些没用的东西
				if content == "下载附件" || content == "保存到相册" {
					empty := ""
					return &empty
				}
				if strings.Contains(content, " 发表于 ") {
					return &content
				}

				href := selec.AttrOr("href", "")
				text := fmt.Sprintf("[%s](%s)", content, href)
				return &text
			},
		},
	)

	client = &http.Client{}
}

func download(url, filename string) {
	fmt.Println(url)
	wg.Add(1)
	go func() {
		defer wg.Done()

		resp, err := client.Get(url)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		out, err := os.Create(fmt.Sprintf("./output/img/%s", filename))
		if err != nil {
			panic(err)
		}

		_, err = io.Copy(out, resp.Body)
		// 下载错误，重试
		if err != nil {
			fmt.Println(err.Error())
			download(url, filename)
		}
	}()
}

func exec(thread *Thread) {
	f, err := os.Create(fmt.Sprintf("./output/9025/%06d.md", thread.Tid))
	if err != nil {
		panic(err)
	}

	fmt.Println(thread.Title)
	err = tc.Execute(f, thread)

	if err != nil {
		panic(err)
	}
}

func loadUsers() (users map[int]string) {
	f, err := os.Open("user.jsonl")
	if err != nil {
		panic(err)
	}

	r := bufio.NewReader(f)
	users = make(map[int]string)

	for {
		//ReadLine 有问题
		l, err := r.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		var user User
		err = json.Unmarshal([]byte(l), &user)
		if err != nil {
			panic(err)
		}

		users[user.Uid] = user.Name
	}

	return
}

func jsonl() {
	users := loadUsers()

	beijing := time.FixedZone("北京时间", int((8 * time.Hour).Seconds()))

	f, err := os.Open("thread.jsonl")
	if err != nil {
		panic(err)
	}

	r := bufio.NewReader(f)
	threads := make([]*Thread, 0)

	c := 0
	for {
		c++
		if c > 50 {
			break
		}

		//ReadLine 有问题
		l, err := r.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		//l = strings.ReplaceAll(l, "&nbsp", " ")
		//fmt.Println(l)

		t := new(Thread)
		err = json.Unmarshal([]byte(l), t)
		if err != nil {
			panic(err)
		}

		if len(t.PostTimeString) <= 10 {
			fmt.Println(t.Title)
			continue
		}

		pt, err := time.ParseInLocation(PostTimeLayout, t.PostTimeString, beijing)
		if err != nil {
			panic(err)
		}
		t.PostTime = pt

		lpt, err := time.ParseInLocation(LastPostTimeLayout, t.LastPostTimeString, beijing)
		if err != nil {
			panic(err)
		}
		t.LastPostTime = lpt

		t.Username = users[t.Author]

		for _, pts := range t.Posts {
			pt, err := time.ParseInLocation(PostTimeLayout, pts.PostTimeString, beijing)
			if err != nil {
				panic(err)
			}

			pts.PostTime = pt
			pts.Username = users[pts.Uid]
			// pts.Content = html2md(pts.Content)
			content, err := converter.ConvertString(pts.Content)
			if err != nil {
				panic(err)
			}

			pts.Content = content
		}

		exec(t)
		threads = append(threads, t)
	}

	fmt.Println(len(threads))

	wg.Wait()
	return
}

func main() {
	//fmt.Println(loadUsers())
	jsonl()
}
