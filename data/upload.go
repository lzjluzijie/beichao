package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

var client = http.DefaultClient

func upload(file os.FileInfo) {
	f, err := os.Open(fmt.Sprintf("img/%s", file.Name()))
	if err != nil {
		panic(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("_resp_mode", "json")
	writer.WriteField("release_id", "72869")

	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(part, f)
	if err != nil {
		fmt.Println(file.Name())
		panic(err)
	}

	err = writer.Close()
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", "https://osdn.net/ajax/downloads/file_upload.php", body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:75.0) Gecko/20100101 Firefox/75.0`)

	// 手动设置
	req.AddCookie(&http.Cookie{Name: "username", Value: ""})
	req.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: ""})
	req.AddCookie(&http.Cookie{Name: "session_ser", Value: ""})

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.StatusCode)
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		upload(file)
	}
	fmt.Println(string(data))
}

func main() {
	//client.Jar, _ = cookiejar.New(nil)
	//u, _ := url.Parse("")
	//client.Jar.SetCookies(u, []*http.Cookie{{Name: "username", Value: "lzjluzijie"}, {Name: "PHPSESSID", Value: "mctt1fcbgeatn9pevd1e4rfru0"}, {Name: "session_ser", Value: "YuDJM4HWB%2BvMUHmcKUBog4wrwRuA5sEx8odUIvQM8ScDA%2BocEP%2Bpb9jz%2B4QBtmoW%2FuthgCSaV1mxqp0xSUiTw5Vof5zWjywGPFfqMeDE7YfSe3%2BvwkPUo3%2BaZLym90ox8ea69vtWq0MVclSZKXy5uQ%3D%3D-a40845922cc3dbfe4b529d4cb2854f4c"}})

	files, err := ioutil.ReadDir("img")
	if err != nil {
		panic(err)
	}

	c := make(chan os.FileInfo)
	for i := 0; i < 10; i++ {
		go func() {
			for {
				file := <-c
				upload(file)
			}
		}()
	}

	for _, file := range files {
		c <- file
	}

	time.Sleep(time.Hour * 24)
}
