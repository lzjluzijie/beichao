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
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
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
var dc = make(chan string)

func init() {
	for i := 0; i < 20; i++ {
		go func() {
			for {
				url := <-dc
				download(url)
			}
		}()
	}

	t, err := template.New("northdy").Parse(Northdy)
	if err != nil {
		panic(err)
	}

	tc = t

	converter = md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())
	converter.AddRules(
		md.Rule{
			Filter: []string{"img"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				alt := selec.AttrOr("alt", "")
				src := selec.AttrOr("file", "")

				if src == "" {
					src = selec.AttrOr("src", "")
				}
				// 可能是表情或者什么奇怪的图片，无视掉
				if src == "" || strings.HasPrefix(src, "static/image/") || strings.HasPrefix(src, "http://bbs.northdy.com//mobcent//app/data/phiz/default/") || strings.HasPrefix(src, "http://bbs.cctvdream.com.cn/static/image/") {
					empty := ""
					return &empty
				}

				// 北朝的图片就下载，否则先无视
				var text string
				if strings.Contains(src, "//bbs.northdy.com/data/attachment/") {
					dc <- src
					text = fmt.Sprintf("![%s](https://cdn.jsdelivr.net/gh/lzjluzijie/beichao@main/static/img/%s)", alt, path.Base(src))
				} else if strings.Contains(src, "//cdn1.northdy.com/data/attachment/") {
					dc <- strings.Replace(src, "//cdn1.northdy.com/data/attachment/", "//bbs.northdy.com/data/attachment/", 1)
					text = fmt.Sprintf("![%s](https://cdn.jsdelivr.net/gh/lzjluzijie/beichao@main/static/img/%s)", alt, path.Base(src))
				} else if strings.Contains(src, "//bbs.cctvdream.com.cn/data/attachment/") {
					dc <- strings.Replace(src, "//bbs.cctvdream.com.cn/data/attachment/", "//bbs.northdy.com/data/attachment/", 1)
					text = fmt.Sprintf("![%s](https://cdn.jsdelivr.net/gh/lzjluzijie/beichao@main/static/img/%s)", alt, path.Base(src))
				} else if strings.Contains(src, "//cdn1.xbahv.cn/data/attachment/") {
					dc <- strings.Replace(src, "//cdn1.xbahv.cn/data/attachment/", "//bbs.northdy.com/data/attachment/", 1)
					text = fmt.Sprintf("![%s](https://cdn.jsdelivr.net/gh/lzjluzijie/beichao@main/static/img/%s)", alt, path.Base(src))
				} else if strings.Contains(src, "//cdn1.oksvip.cn/data/attachment/") {
					dc <- strings.Replace(src, "//cdn1.oksvip.cn/data/attachment/", "//bbs.northdy.com/data/attachment/", 1)
					text = fmt.Sprintf("![%s](https://cdn.jsdelivr.net/gh/lzjluzijie/beichao@main/static/img/%s)", alt, path.Base(src))
				} else if strings.Contains(src, "//7xlupq.com1.z0.glb.clouddn.com/data/attachment/") {
					dc <- strings.Replace(src, "//7xlupq.com1.z0.glb.clouddn.com/data/attachment/", "//bbs.northdy.com/data/attachment/", 1)
					text = fmt.Sprintf("![%s](https://cdn.jsdelivr.net/gh/lzjluzijie/beichao@main/static/img/%s)", alt, path.Base(src))
				} else if strings.HasPrefix(src, "data/attachment/") {
					dc <- "https://bbs.northdy.com/" + src
					text = fmt.Sprintf("![%s](https://cdn.jsdelivr.net/gh/lzjluzijie/beichao@main/static/img/%s)", alt, path.Base(src))
				} else {
					//fmt.Println(src)
					if strings.Contains(src, "data/attachment") {
						//fmt.Println(src)
					}
					text = fmt.Sprintf("![%s](%s)", alt, src)
				}
				return &text
			},
		},
		md.Rule{
			Filter: []string{"a"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				// 无视一些没用的东西
				if content == "" || content == "下载附件" || content == "保存到相册" {
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

func download(url string) {
	if strings.HasPrefix(url, "http://") {
		url = strings.Replace(url, "http://", "https://", 1)
	}
	filename := path.Base(url)
	fmt.Println(url)

	resp, err := client.Get(url)
	if err != nil {
		fmt.Println(err.Error())
		download(url)
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
		download(url)
	}
}

func exec(thread *Thread) {
	f, err := os.Create(fmt.Sprintf("./output/9025/%06d.md", thread.Tid))
	if err != nil {
		panic(err)
	}

	//fmt.Println(thread.Title)
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
		//if c > 50 {
		//	break
		//}

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

			content = strings.ReplaceAll(content, "\n", "\n\n")
			content = strings.ReplaceAll(content, "\n\n> ", "\n> \n> ")
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
