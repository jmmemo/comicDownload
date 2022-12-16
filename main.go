package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// https://hanime1.me/comics

var prefix_format = "./%s/%d.png" //eg: ./xxpkg/1.png
var url_format = "https://i.nhentai.net/galleries/%d/%d.jpg"

var id int         //漫画cdn ID
var page int       //漫画页数
var restart_at = 1 //默认从1开始
var tag string     //漫画文件夹名字
var logFile *os.File
var job_doing int32 = 0

func init() {
	f, err := os.OpenFile("record.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}

	logFile = f
}

func main() {
	jobs := make(chan string)

	go func() {
		for {
			fmt.Println("要添加多个任务,请以','分割分别输入: id,page,tag 如果要指定重新开始的页数: id,page,tag,restart_at")
			r := bufio.NewReader(os.Stdin)
			b, _, err := r.ReadLine()
			if err != nil {
				panic(err)
			}

			jobs <- string(b)
		}
	}()

	for {
		select {
		case <-time.After(time.Second * 60):
			// fmt.Println("时间到了,看你走不走吧")
			if atomic.LoadInt32(&job_doing) == 0 {
				fmt.Println("没事干,我走了")
				close(jobs)
				return
			}

		case job := <-jobs:
			arr := strings.Split(job, ",")
			len_args := len(arr)
			if len_args != 3 && len_args != 4 { //参数至少需要3个,至多4个
				panic("以','分割分别输入: id,page,tag  如果要指定重新开始的页数: id,page,tag,restart_at")
			}

			id, _ = strconv.Atoi(arr[0])
			page, _ = strconv.Atoi(arr[1])
			tag = arr[2]

			if len_args == 4 {
				restart_at, _ = strconv.Atoi(arr[3])
			}

			go download_work(id, page, restart_at, tag)
		}
	}
}

func download_work(in_id, in_page, restart_at int, in_tag string) {
	atomic.AddInt32(&job_doing, 1)

	once_path := sync.Once{}
	info_log(in_id, in_page, in_tag, restart_at)

	rand.Seed(time.Now().UnixNano())
	s := time.Now()
	// defer fmt.Println(time.Since(s)) //bad

	for i := restart_at; i <= in_page; i++ {
		url := fmt.Sprintf(url_format, in_id, i)
		req, _ := http.NewRequest(http.MethodGet, url, nil)

		//
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
		req.Header.Add("Accept-Encoding", "gzip, deflate, br")
		req.Header.Add("Accept-Language", "zh-CN,zh;q=0.8,en-US;q=0.6,en;q=0.4")
		req.Header.Add("Connection", "keep-alive")
		req.Header.Add("Content-Length", "25")
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		cli := http.Client{}
		r, err := cli.Do(req)
		if err_record_download_info("下载中断", err, in_id, in_page, i, in_tag) {
			return
		}
		defer r.Body.Close()

		b, err := ioutil.ReadAll(r.Body)
		if err_record_download_info("下载中断", err, in_id, in_page, i, in_tag) {
			return
		}

		once_path.Do(func() {
			err = os.Mkdir(in_tag, os.ModeDir)
			if err != nil && !os.IsExist(err) {
				err_record_download_info("下载中断", err, in_id, in_page, i, in_tag)
			}

			err = os.Chmod(in_tag, 0777)
			if err_record_download_info("下载中断", err, in_id, in_page, i, in_tag) {
				return
			}
		})

		pa := fmt.Sprintf(prefix_format, in_tag, i)
		// fmt.Println(pa)

		f, err := os.OpenFile(pa, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err_record_download_info("下载中断", err, in_id, in_page, i, in_tag) {
			return
		}

		f.Write(b)

		time.Sleep(time.Millisecond * time.Duration(rand.Intn(66)))
	}

	fmt.Printf("[%s]下载完成,耗时:[%v]\n", in_tag, time.Since(s))

	atomic.AddInt32(&job_doing, -1)
	info_log(in_id, in_page, fmt.Sprintf("[%s]下载完成,耗时[%v]", in_tag, time.Since(s)), restart_at)
}

func info_log(in_id, in_page int, in_tag string, restart_at int) {
	if restart_at > 1 {
		logFile.WriteString(fmt.Sprintf("[RESTART] restart_at[%d] %s id[%d] page[%d] tag[%s]\n", restart_at, time.Now().Format("2006-01-02 15:04:05"), in_id, in_page, in_tag))
	}
	logFile.WriteString(fmt.Sprintf("%s id[%d] page[%d] tag[%s]\n", time.Now().Format("2006-01-02 15:04:05"), in_id, in_page, in_tag))
}

func err_log(msg string, err error, in_id, in_page, failed_at int, in_tag string) {
	logFile.WriteString(fmt.Sprintf("[ERROR] msg[%s] err[%s] fail_at_page[%d] %s id[%d] page[%d] tag[%s]\n", msg, err, failed_at, time.Now().Format("2006-01-02 15:04:05"), in_id, in_page, in_tag))
}

func err_record_download_info(msg string, err error, in_id, in_page, failed_at int, in_tag string) bool {
	if err != nil {
		atomic.AddInt32(&job_doing, -1)
		err_log(msg, err, in_id, in_page, failed_at, in_tag)
		return true
	}

	return false
}
